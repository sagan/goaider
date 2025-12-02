package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/natefinch/atomic"
	"github.com/richinsley/comfy2go/client"
	"github.com/richinsley/comfy2go/graphapi"
	log "github.com/sirupsen/logrus"

	"github.com/sagan/goaider/util"
)

// ComfyUI "LoadImage" node, widgets_values[0] is the filename that must be uploaded to server "input/" folder first.
const NODE_TYPE_LOAD_IMAGE = "LoadImage"
const NODE_TYPE_LOAD_IMAGE_MASK = "LoadImageMask"
const NODE_TYPE_SAVE_VIDEO = "SaveVideo"

// Try to extract addr (hostname) & port from a rawUrl,
// which could be "127.0.0.1:8188" or "http://127.0.0.1:8188" format.
// Return: "http", "127.0.0.1", 8188.
func parseAddrFromUrl(rawURL string) (schema string, addr string, port int, err error) {
	if !strings.HasPrefix(rawURL, "http:") && !strings.HasPrefix(rawURL, "https:") {
		rawURL = "http://" + rawURL
	}
	urlObj, err := url.Parse(rawURL)
	if err != nil {
		return "", "", 0, err
	}
	if urlObj.Port() == "" {
		if urlObj.Scheme == "https" {
			return urlObj.Scheme, urlObj.Hostname(), 443, nil
		} else {
			return urlObj.Scheme, urlObj.Hostname(), 80, nil
		}
	}
	port, err = strconv.Atoi(urlObj.Port())
	if err != nil {
		return urlObj.Scheme, urlObj.Hostname(), 0, err
	}
	return urlObj.Scheme, urlObj.Hostname(), port, nil
}

// 这个 client.ComfyClient 真 TMD 难用。
type Client struct {
	*client.ComfyClient
	Base string // Base addr. E.g. "http://127.0.0.1:8188"
}

// fileType: input | output.
func (c *Client) CheckFileExists(filename string, fileType client.ImageType) (exists bool, err error) {
	params := url.Values{}
	params.Add("filename", filename)
	params.Add("type", string(fileType))
	resp, err := http.DefaultClient.Get(fmt.Sprintf("%s/view?%s", c.Base, params.Encode()))
	if err != nil {
		return false, err
	}
	exists = resp.StatusCode == 200
	return exists, nil
}

func (c *Client) CheckInputFileExists(filename string) (exists bool, err error) {
	return c.CheckFileExists(filename, client.InputImageType)
}

// clientaddr : "127.0.0.1:8188" or "http://127.0.0.1:8188" .
func CreateAndInitComfyClient(clientaddr string) (comfyClient *Client, err error) {
	schema, addr, port, err := parseAddrFromUrl(clientaddr)
	if err != nil {
		return nil, err
	}
	if schema != "http" && schema != "https" {
		return nil, fmt.Errorf("unsupported schema: %s", schema)
	}
	interClient := client.NewComfyClient(schema, addr, port, nil)
	if !interClient.IsInitialized() {
		err = interClient.Init()
		if err != nil {
			return nil, err
		}
	}
	base := schema + "://" + addr
	if schema == "http" && port != 80 || schema == "https" && port != 443 {
		base = fmt.Sprintf("%s:%d", base, port)
	}

	return &Client{
		ComfyClient: interClient,
		Base:        base,
	}, nil
}

// comfyui output file
type ComfyuiOutput struct {
	Data     []byte // image data
	Filename string // unique filename. format: "cu-<hash>png". hash is sha256 url-safe base64.
	Text     string // exists if it's "text" type data output
	Type     string // "output", "input"
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

// generate a global unique "cu-<hash>.png" style filename for a ComfyUI output file.
func genFilename(data []byte, output *client.DataOutput) string {
	s := sha256.New()
	s.Write(data)
	b64 := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(s.Sum(nil))
	ext := filepath.Ext(output.Filename)
	return "cu-" + b64 + ext
}

// Ensure all input images in graph exists in ComfyUI server, upload missing files
// node: LoadImage .
// filename: widgets_values[0].
// Note: it modify the graph.
func (comfyClient *Client) PrepareGraph(graph *graphapi.Graph) (err error) {
	for _, node := range graph.Nodes {
		if node.Type != NODE_TYPE_LOAD_IMAGE && node.Type != NODE_TYPE_LOAD_IMAGE_MASK || node.WidgetValues == nil {
			continue
		}
		weightValues, ok := node.WidgetValues.([]any)
		if !ok || len(weightValues) == 0 {
			log.Warnf("node %d (LoadImage) has no widget values", node.ID)
			continue
		}
		filename, ok := weightValues[0].(string)
		if !ok {
			log.Warnf("node %d (LoadImage) has no filename in widget values", node.ID)
			continue
		}
		hash, err := util.Sha256sumFile(filename, false)
		if err != nil {
			return fmt.Errorf("failed to calc input image %q hash: %w", filename, err)
		}
		serverFilename := hash + filepath.Ext(filename)
		log.Printf("check image %q => %q", filename, serverFilename)
		exists, err := comfyClient.CheckInputFileExists(serverFilename)
		if err != nil {
			return fmt.Errorf("failed to check if input file filename %q (%q) exists: %w", filename, serverFilename, err)
		}
		if !exists {
			log.Printf("uploading input file %q => %q", filename, serverFilename)
			file, err := os.Open(filename)
			if err != nil {
				return err
			}
			defer file.Close()
			_, err = comfyClient.UploadFileFromReader(file, serverFilename, false, client.InputImageType, "", nil)
			if err != nil {
				return fmt.Errorf("failed to upload input file %q: %w", filename, err)
			}
			err = SetGraphNodeWidetValue(graph, node.ID, "0", serverFilename, 0)
			if err != nil {
				return err
			}
		}
	}
	for _, node := range graph.Nodes {
		switch node.Type {
		case NODE_TYPE_SAVE_VIDEO: // comfy2go doesn't handle this node
			node.Properties["filename_prefix"] = *graphapi.NewPropertyFromInput("filename_prefix",
				false, &node.WidgetValues, 0)
			node.Properties["format"] = *graphapi.NewPropertyFromInput("format",
				false, &node.WidgetValues, 1)
			node.Properties["codec"] = *graphapi.NewPropertyFromInput("codec",
				false, &node.WidgetValues, 2)
		case NODE_TYPE_LOAD_IMAGE, NODE_TYPE_LOAD_IMAGE_MASK:
			// we changed the filename, need to re-calculate properties?
		}
	}
	return nil
}

// RunWorkflow runs a ComfyUI workflow and returns the outputs.
// It initializes the client, queues the prompt, and waits for the workflow to complete,
// collecting any image or GIF outputs.
// Each item in returned outputs have global unique filename.
func (comfyClient *Client) RunWorkflow(graph *graphapi.Graph) (outputs ComfyuiOutputs, err error) {
	// queue the prompt and get the resulting image
	item, err := comfyClient.QueuePrompt(graph)
	if err != nil {
		return nil, fmt.Errorf("failed to queue prompt: %w", err)
	}
	defer func() {
		go func() {
			// read and discard all left message in item.Messages channel.
			for range item.Messages {
			}
		}()
	}()

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
		default: // progress
			// log.Printf("event %s: %v", msg.Type, msg.Message)
		}
	}
	return outputs, fmt.Errorf("comfyui server disconnected")
}

// Load graph from filename, if it's "-", read from stdin.
func NewGraph(comfyClient *Client, filename string) (graph *graphapi.Graph, err error) {
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
		// Remove some nodes that comfy2go doesn't yet support.
		// remove those nodes from obj.nodes array.
		if nodes, ok := obj["nodes"].([]any); ok {
			var filteredNodes []any
			for _, node := range nodes {
				if nodeMap, isMap := node.(map[string]any); isMap {
					if nodeType, typeOk := nodeMap["type"].(string); typeOk &&
						(nodeType == "MarkdownNote" || nodeType == "Note") {
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

func GetGraphNodeWidetValue(graph *graphapi.Graph, nodeId int, accessor string) (value any, err error) {
	node := graph.GetNodeById(nodeId)
	if node == nil {
		return nil, fmt.Errorf("node %d not found", nodeId)
	}
	if node.WidgetValues == nil {
		return nil, fmt.Errorf("node %d has no widget values", nodeId)
	}

	arr, ok := node.WidgetValues.([]any)
	if !ok {
		return nil, fmt.Errorf("node %d widget values is not an array", nodeId)
	}
	index, err := strconv.Atoi(accessor)
	if err != nil {
		return nil, fmt.Errorf("accessor is not int")
	}
	if index < 0 || index >= len(arr) {
		return nil, fmt.Errorf("index %d out of bounds for node %d widget values (len %d)", index, nodeId, len(arr))
	}

	return arr[index], nil
}

// accessor: currently only a single array index is supported;
// in the future it may support deep attribute like "foo.bar.baz".
// value should either string or int.
// Special placeholder in value: "%seed%" : a random integer.
func SetGraphNodeWidetValue(graph *graphapi.Graph, nodeId int, accessor string, value any, seed int64) (err error) {
	node := graph.GetNodeById(nodeId)
	if node == nil {
		return fmt.Errorf("node %d not found", nodeId)
	}
	if node.WidgetValues == nil {
		return fmt.Errorf("node %d has no widget values", nodeId)
	}
	// widgets_values can be an array of values, or a map of values maps of values can represent
	// cascading style properties in which the setting of one property makes certain other properties available.
	// Only array values is supported at this time.
	arr, ok := node.WidgetValues.([]any)
	if !ok {
		return fmt.Errorf("node %d widget values is not an array", nodeId)
	}
	index, err := strconv.Atoi(accessor)
	if err != nil {
		return fmt.Errorf("accessor is not int")
	}
	if index < 0 || index >= len(arr) {
		return fmt.Errorf("index %d out of bounds for node %d widget values (len %d)", index, nodeId, len(arr))
	}

	if str, ok := value.(string); ok {
		value = strings.ReplaceAll(str, "%seed%", fmt.Sprint(seed))
	}

	// if new value and existing value has different types (string / number), coalesce value to match existing type
	switch arr[index].(type) {
	case string:
		if v, isString := value.(string); isString {
			arr[index] = v
		} else {
			arr[index] = fmt.Sprintf("%v", value)
		}
	case float64: // JSON unmarshals numbers to float64 by default
		if v, isFloat := value.(float64); isFloat {
			arr[index] = v
		} else if v, isInt := value.(int); isInt {
			arr[index] = float64(v)
		} else if v, isString := value.(string); isString {
			if fv, err := strconv.ParseFloat(v, 64); err == nil {
				arr[index] = fv
			} else {
				return fmt.Errorf("cannot convert string %q to float64 for node %d widget value at index %d", v, nodeId, index)
			}
		} else {
			return fmt.Errorf("unsupported value type for float64 target at node %d widget value at index %d", nodeId, index)
		}
	case bool:
		if v, isBool := value.(bool); isBool {
			arr[index] = v
		} else if v, isString := value.(string); isString {
			if bv, err := strconv.ParseBool(v); err == nil {
				arr[index] = bv
			} else {
				return fmt.Errorf("cannot convert string %q to bool for node %d widget value at index %d", v, nodeId, index)
			}
		} else {
			return fmt.Errorf("unsupported value type for bool target at node %d widget value at index %d", nodeId, index)
		}
	default:
		// Fallback for other types, attempt direct assignment
		arr[index] = value
	}
	node.WidgetValues = arr
	return nil
}

// Return a random seed for ComfyUI of range [0, 2⁵³ - 1].
// The upper bound is capped to the MAX_SAFE_ITNEGER (IEEE float64 precision bits) of JavaScript for compability.
func RandSeed() int64 {
	return util.RandInt(0, 9007199254740991)
}

// values item format: "node_id:index:value", e.g. "42:0:foo.png"
func SetGraphNodeWeightValues(graph *graphapi.Graph, values []string, seed int64) error {
	for _, item := range values {
		parts := strings.SplitN(item, ":", 3)
		if len(parts) != 3 {
			return fmt.Errorf("invalid value format: %s, expected 'node_id:index:value'", item)
		}

		nodeID, err := strconv.Atoi(parts[0])
		if err != nil {
			return fmt.Errorf("invalid node ID %q: %w", parts[0], err)
		}

		accessor := parts[1] // This is the index as a string

		// Attempt to infer type for the value.
		// For now, we'll pass it as a string and let SetGraphNodeWidetValue handle conversion.
		value := parts[2]

		if err := SetGraphNodeWidetValue(graph, nodeID, accessor, value, seed); err != nil {
			return fmt.Errorf("failed to set widget value for node %d, accessor %s: %w", nodeID, accessor, err)
		}
	}
	return nil
}
