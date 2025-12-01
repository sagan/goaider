package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/natefinch/atomic"
	"github.com/richinsley/comfy2go/client"
	"github.com/richinsley/comfy2go/graphapi"
	"github.com/sagan/goaider/util"
	log "github.com/sirupsen/logrus"
)

// Try to extract addr (hostname) & port from a rawUrl,
// which could be "127.0.0.1:80" or "http://127.0.0.1:8080" format.
func ParseAddrFromUrl(rawURL string) (addr string, port int, err error) {
	if !strings.HasPrefix(rawURL, "http:") && !strings.HasPrefix(rawURL, "https:") {
		rawURL = "http://" + rawURL
	}
	urlObj, err := url.Parse(rawURL)
	if err != nil {
		return "", 0, err
	}
	if urlObj.Port() == "" {
		if urlObj.Scheme == "https" {
			return urlObj.Hostname(), 443, nil
		} else {
			return urlObj.Hostname(), 80, nil
		}
	}
	port, err = strconv.Atoi(urlObj.Port())
	if err != nil {
		return urlObj.Hostname(), 0, err
	}
	return urlObj.Hostname(), port, nil
}

// clientaddr : "127.0.0.1:1080" or "http://127.0.0.1:1080" .
func CreateAndInitComfyClient(clientaddr string) (comfyClient *client.ComfyClient, err error) {
	addr, port, err := ParseAddrFromUrl(clientaddr)
	if err != nil {
		return nil, err
	}
	comfyClient = client.NewComfyClient(addr, port, nil)
	if !comfyClient.IsInitialized() {
		err = comfyClient.Init()
		if err != nil {
			return nil, err
		}
	}
	return comfyClient, nil
}

// comfyui output file
type ComfyuiOutput struct {
	Data     []byte // image data
	Filename string // unique filename. format: "cu-<hash>png". hash is sha256 url-safe base64.
	Text     string // exists if it's "text" type data output
	Type     string // "output"
}

type ComfyuiOutputs []*ComfyuiOutput

// Save the first output to filename. If filename is "-", output to stdout.
// If filename exists and force is false, returns an error.
func (outputs ComfyuiOutputs) Save(filename string, force bool) (err error) {
	for _, output := range outputs {
		if output.Type == "text" {
			fmt.Printf("text output %s: %s\n", output.Filename, output.Text)
			continue
		}
		if filename == "-" {
			_, err = os.Stdout.Write(output.Data)
			return err
		}
		if exists, err := util.FileExists(filename); err != nil || (exists && !force) {
			return fmt.Errorf("output file %q exists or access failed. err: %w", filename, err)
		}
		return atomic.WriteFile(filename, bytes.NewReader(output.Data))
	}
	return fmt.Errorf("no output")
}

// Save all outputs to dir.
// If force is true, overwrite any existing file, otherwise skip them.
func (outputs ComfyuiOutputs) SaveAll(dir string, force bool) error {
	var lastErr error
	for _, output := range outputs {
		if output.Type == "text" {
			fmt.Printf("text output %s: %s\n", output.Filename, output.Text)
			continue
		}
		outputFile := filepath.Join(dir, output.Filename)
		if exists, err := util.FileExists(outputFile); err != nil || (exists && !force) {
			if err != nil {
				lastErr = fmt.Errorf("output file %q access failed: %w", outputFile, err)
			}
			continue
		}
		err := atomic.WriteFile(outputFile, bytes.NewReader(output.Data))
		if err != nil {
			log.Errorf("error saving %s: %v", output.Filename, err)
			lastErr = err
		} else {
			log.Printf("Output saved to %s\n", outputFile)
		}
	}
	return lastErr
}

func genFilename(data []byte, output *client.DataOutput) string {
	s := sha256.New()
	s.Write(data)
	b64 := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(s.Sum(nil))
	ext := filepath.Ext(output.Filename)
	return "cu-" + b64 + ext
}

// RunWorkflow runs a ComfyUI workflow and returns the outputs.
// It initializes the client, queues the prompt, and waits for the workflow to complete,
// collecting any image or GIF outputs.
// Each item in returned outputs have unique filename.
func RunWorkflow(comfyClient *client.ComfyClient,
	graph *graphapi.Graph) (outputs ComfyuiOutputs, err error) {
	// queue the prompt and get the resulting image
	item, err := comfyClient.QueuePrompt(graph)
	if err != nil {
		return nil, fmt.Errorf("failed to queue prompt: %w", err)
	}

	// continuously read messages from the QueuedItem until we get the "stopped" message type
	for continueLoop := true; continueLoop; {
		msg := <-item.Messages
		switch msg.Type {
		case "stopped":
			// if we were stopped for an exception, display the exception message
			qm := msg.ToPromptMessageStopped()
			if qm.Exception != nil {
				return nil, fmt.Errorf("exception: %v", qm.Exception)
			}
			continueLoop = false
		case "data":
			qm := msg.ToPromptMessageData()
			// data objects have the fields: Filename, Subfolder, Type
			// * Subfolder is the subfolder in the output directory
			// * Type is the type of the image temp/
			for k, v := range qm.Data {
				log.Printf("comfyui item data: %s => %v", k, v)
				if k == "images" || k == "gifs" {
					for _, output := range v {
						imgData, err := comfyClient.GetImage(output)
						if err != nil {
							return outputs, fmt.Errorf("failed to get image: %w", err)
						}
						if imgData == nil || len(*imgData) == 0 {
							log.Warnf("image data is empty for output %v", output)
							continue
						}
						outputs = append(outputs, &ComfyuiOutput{
							Data:     *imgData,
							Filename: genFilename(*imgData, &output),
							Text:     output.Text,
							Type:     output.Type,
						})
					}
					return outputs, nil
				}
			}
		default:
			log.Printf("event %s: %v", msg.Type, msg.Message)
		}
	}
	return outputs, fmt.Errorf("comfyui server disconnected")
}

// Load graph from filename, if it's "-", read from stdin.
func NewGraph(comfyClient *client.ComfyClient, filename string) (graph *graphapi.Graph, err error) {
	var data []byte
	if filename == "-" {
		data, err = io.ReadAll(os.Stdin)
		if err != nil {
			return nil, err
		}
	} else {
		data, err = os.ReadFile(filename)
		if err != nil {
			return nil, err
		}
	}
	// json workflow
	if utf8.Valid(data) {
		var obj map[string]any
		err = json.Unmarshal(data, &obj)
		if err != nil {
			return nil, err
		}
		// Skip some nodes that comfy2go doesn't yet support.
		// remove those nodes from obj.nodes array.
		// unsupported nodes: node.type == "MarkdownNote"
		if nodes, ok := obj["nodes"].([]any); ok {
			var filteredNodes []any
			for _, node := range nodes {
				if nodeMap, isMap := node.(map[string]any); isMap {
					if nodeType, typeOk := nodeMap["type"].(string); typeOk && nodeType == "MarkdownNote" {
						continue
					}
				}
				filteredNodes = append(filteredNodes, node)
			}
			obj["nodes"] = filteredNodes
			data, err = json.Marshal(obj)
			if err != nil {
				return nil, err
			}
		}
		graph, _, err = comfyClient.NewGraphFromJsonString(string(data))
	} else {
		// png workflow
		graph, _, err = comfyClient.NewGraphFromPNGReader(bytes.NewReader(data))
	}
	return graph, err
}
