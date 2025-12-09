package mediainfo

import (
	"encoding/json"
	"fmt"
	"os/exec"
)

type MediaFileInfo struct {
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
	Duration string `json:"duration,omitempty"` // video / audio duration (seconds)
}

type ffprobeOutput struct {
	Streams []struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	} `json:"streams"`
	Format struct {
		Duration string `json:"duration"` // duration (seconds) string
	} `json:"format"`
}

// Run ffprobe command to get media info
func ParseMediaInfo(filename string) (info *MediaFileInfo, err error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0",
		"-show_entries", "stream=width,height", "-show_entries", "format=duration", "-of", "json", filename)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run ffprobe for video info: %w", err)
	}

	var videoProbeResult ffprobeOutput
	err = json.Unmarshal(output, &videoProbeResult)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe video output: %w", err)
	}
	info = &MediaFileInfo{}
	if len(videoProbeResult.Streams) > 0 {
		info.Width = videoProbeResult.Streams[0].Width
		info.Height = videoProbeResult.Streams[0].Height
	}
	info.Duration = videoProbeResult.Format.Duration
	return info, nil
}
