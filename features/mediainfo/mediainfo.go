package mediainfo

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

type MediaFileInfo struct {
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Duration  string `json:"duration"`  // video / audio duration (seconds)
	Signature string `json:"signature"` // image signature (sha256 of pixel data)`
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

var (
	initializeOnce sync.Once
	ffprobeExists  bool
)

// Init function. Safe to call multiples.
func Init() {
	initializeOnce.Do(func() {
		_, err := exec.LookPath("ffprobe")
		if err == nil {
			ffprobeExists = true
		} else {
			log.Warnf(`"ffprobe" not found in PATH. Video/audio media info parsing will be disabled: %v`, err)
		}
	})
}

// ParseMediaInfo parses media file info from a given input reader.
// It attempts to detect the MIME type if not provided.
// For image files, it extracts width, height, and calculates a SHA256 signature of pixel data.
// For video/audio files, it uses ffprobe to extract width, height, and duration.
// Requires "ffprobe" to be in the system's PATH for video/audio files.
//
// Parameters:
//
//	input: An io.Reader from which to read the media file's content.
//	mimeType: The MIME type of the input file (e.g., "image/jpeg", "video/mp4").
//	  If empty, the function will attempt to detect it from the input.
//
// Returns:
//
//	*MediaFileInfo: A pointer to a struct containing the extracted media information.
//	error: An error if parsing fails or the media type is unsupported.
func ParseMediaInfo(input io.Reader, mimeType string) (info *MediaFileInfo, err error) {
	if mimeType == "" {
		br := bufio.NewReader(input)
		header, err := br.Peek(512)
		if err != nil && err != io.EOF {
			return nil, err
		}
		mimeType = http.DetectContentType(header)
		input = br
	}
	switch {
	case strings.HasPrefix(mimeType, "image/"):
		// https://github.com/golang/go/issues/43382
		if mimeType == "image/png" {
			input = &PNGAncillaryChunkStripper{Reader: input}
		}
		return ParseImageFileMediaInfo(input)
	case strings.HasPrefix(mimeType, "video/"), strings.HasPrefix(mimeType, "audio/"):
		return ParseVideoAudioMediaInfo(input)
	}
	return nil, fmt.Errorf("unknown media type %s", mimeType)
}

func ParseVideoAudioMediaInfo(input io.Reader) (info *MediaFileInfo, err error) {
	if !ffprobeExists {
		return nil, fmt.Errorf("ffprobe not found in PATH. Please install ffmpeg/ffprobe to parse video/audio media info")
	}
	cmd := exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0",
		"-show_entries", "stream=width,height", "-show_entries", "format=duration", "-of", "json", "-")
	cmd.Stdin = input
	var output []byte
	output, err = cmd.Output()
	if err != nil {
		return nil, err
	}
	info = &MediaFileInfo{}
	var videoProbeResult *ffprobeOutput
	err = json.Unmarshal(output, &videoProbeResult)
	if err != nil {
		return nil, err
	}
	if len(videoProbeResult.Streams) > 0 {
		info.Width = videoProbeResult.Streams[0].Width
		info.Height = videoProbeResult.Streams[0].Height
	}
	info.Duration = videoProbeResult.Format.Duration
	return info, nil
}

func ParseImageFileMediaInfo(input io.Reader) (mediainfo *MediaFileInfo, err error) {
	img, _, err := image.Decode(input)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}
	bounds := img.Bounds()
	signature := PixelDataHashAlphaAware(img)
	return &MediaFileInfo{
		Width:     bounds.Dx(),
		Height:    bounds.Dy(),
		Signature: signature,
	}, nil
}

// PixelDataHashAlphaAware returns a SHA-256 hash of the image's
// pixel data in a canonical, alpha-aware format:
//
//   - Convert to image.NRGBA (non-premultiplied sRGB RGBA, 8-bit)
//   - Walk pixels in scanline order (top-to-bottom, left-to-right)
//   - For pixels with A == 0, RGB is forced to 0 (canonicalize invisible pixels)
//   - For A > 0, RGB and A are used as-is
//
// This means:
//   - Differences only in the RGB values of fully transparent pixels
//     DO NOT change the hash.
//   - Any difference that could affect visible output (A>0 or different RGB
//     where A>0) WILL change the hash.
func PixelDataHashAlphaAware(img image.Image) string {
	b := img.Bounds()

	// Ensure non-premultiplied 8-bit RGBA backing store.
	var nrgba *image.NRGBA
	if v, ok := img.(*image.NRGBA); ok && v.Bounds() == b {
		nrgba = v
	} else {
		nrgba = image.NewNRGBA(b)
		draw.Draw(nrgba, b, img, b.Min, draw.Src)
	}

	h := sha256.New()

	w, hgt := b.Dx(), b.Dy()
	rowBuf := make([]byte, w*4) // temporary row buffer (RGBA per pixel)

	for y := range hgt {
		srcOff := y * nrgba.Stride

		// Copy the row so we can normalize it before hashing
		copy(rowBuf, nrgba.Pix[srcOff:srcOff+w*4])

		// Canonicalize fully transparent pixels.
		for x := range w {
			i := x * 4
			a := rowBuf[i+3]
			if a == 0 {
				// Make invisible pixels fully canonical: RGBA = (0,0,0,0).
				rowBuf[i+0] = 0
				rowBuf[i+1] = 0
				rowBuf[i+2] = 0
				// A is already 0
			}
		}

		h.Write(rowBuf)
	}

	return hex.EncodeToString(h.Sum(nil))
}
