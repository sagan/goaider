package api

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

// Parse ComfyUI generated .png images EXIF

type ComfyUIPngMeta struct {
	Prompt   map[string]any `json:"prompt,omitempty"`
	Workflow map[string]any `json:"workflow,omitempty"`
}

// ExtractComfyMetadata scans a PNG file for tEXt chunks without decoding the image
func ExtractComfyMetadata(f io.Reader) (cuMeta *ComfyUIPngMeta, err error) {
	// 1. Verify PNG Header
	header := make([]byte, 8)
	if _, err := io.ReadFull(f, header); err != nil {
		return nil, err
	}
	if !bytes.Equal(header, []byte{137, 80, 78, 71, 13, 10, 26, 10}) {
		return nil, fmt.Errorf("not a valid PNG file")
	}

	// See comfyui_png_meta_sample.json for example .
	// soure: https://raw.githubusercontent.com/Comfy-Org/example_workflows/main/flux/text-to-image/flux_dev_fp8.png .
	// from: https://docs.comfy.org/tutorials/flux/flux-1-text-to-image
	metadata := make(map[string]string)

	// 2. Iterate over Chunks
	for {
		// Read Length (4 bytes)
		var length uint32
		if err := binary.Read(f, binary.BigEndian, &length); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		// Read Type (4 bytes)
		chunkType := make([]byte, 4)
		if _, err := io.ReadFull(f, chunkType); err != nil {
			return nil, err
		}

		// Calculate End of Chunk position to skip parsing non-text chunks
		// We need to read the data anyway to verify CRC or just skip it
		// For efficiency, we read data only if it's tEXt

		if string(chunkType) == "tEXt" {
			data := make([]byte, length)
			if _, err := io.ReadFull(f, data); err != nil {
				return nil, err
			}

			// tEXt format: Keyword + Null Separator + TextString
			parts := bytes.SplitN(data, []byte{0}, 2)
			if len(parts) == 2 {
				key := string(parts[0])
				val := string(parts[1])
				metadata[key] = val
			}

			// Read CRC (4 bytes) - strictly we should skip 4 bytes
			if _, err := io.CopyN(io.Discard, f, 4); err != nil {
				return nil, err
			}

		} else {
			// Skip data (length) + CRC (4 bytes)
			if _, err := io.CopyN(io.Discard, f, int64(length)+4); err != nil {
				return nil, err
			}
		}
	}

	cuMeta = &ComfyUIPngMeta{}
	if promptJSON, ok := metadata["prompt"]; ok {
		err = json.Unmarshal([]byte(promptJSON), &cuMeta.Prompt)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal prompt JSON: %w", err)
		}
	}
	if workflowJSON, ok := metadata["workflow"]; ok {
		err = json.Unmarshal([]byte(workflowJSON), &cuMeta.Workflow)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal workflow JSON: %w", err)
		}
	}

	return cuMeta, nil
}
