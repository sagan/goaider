package mediainfo

import (
	"bufio"
	"bytes"
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
	"time"

	exif "github.com/dsoprea/go-exif/v3"
	exifcommon "github.com/dsoprea/go-exif/v3/common"

	log "github.com/sirupsen/logrus"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

type MediaFileInfo struct {
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	Duration  string    `json:"duration"`  // video / audio duration (seconds)
	Signature string    `json:"signature"` // image signature (sha256 of pixel data)`
	Ctime     time.Time `json:"ctime"`     // photo / video creation_time
}

type FfprobeOutput struct {
	Streams []Stream `json:"streams"`
	Format  Format   `json:"format"`
}

// Stream represents a single video or audio stream
type Stream struct {
	Index          int    `json:"index"`
	ID             string `json:"id"`
	CodecName      string `json:"codec_name"`
	CodecLongName  string `json:"codec_long_name"`
	Profile        string `json:"profile,omitempty"`
	CodecType      string `json:"codec_type"`
	CodecTagString string `json:"codec_tag_string"`
	CodecTag       string `json:"codec_tag"`

	// Video specific fields
	Width          int    `json:"width,omitempty"`
	Height         int    `json:"height,omitempty"`
	CodedWidth     int    `json:"coded_width,omitempty"`
	CodedHeight    int    `json:"coded_height,omitempty"`
	ClosedCaptions int    `json:"closed_captions,omitempty"`
	FilmGrain      int    `json:"film_grain,omitempty"`
	HasBFrames     int    `json:"has_b_frames,omitempty"`
	PixFmt         string `json:"pix_fmt,omitempty"`
	Level          int    `json:"level,omitempty"`
	ColorRange     string `json:"color_range,omitempty"`
	ColorSpace     string `json:"color_space,omitempty"`
	ColorTransfer  string `json:"color_transfer,omitempty"`
	ColorPrimaries string `json:"color_primaries,omitempty"`
	ChromaLocation string `json:"chroma_location,omitempty"`
	FieldOrder     string `json:"field_order,omitempty"`
	Refs           int    `json:"refs,omitempty"`
	IsAvc          string `json:"is_avc,omitempty"`
	NalLengthSize  string `json:"nal_length_size,omitempty"`

	// Audio specific fields
	SampleFmt      string `json:"sample_fmt,omitempty"`
	SampleRate     string `json:"sample_rate,omitempty"`
	Channels       int    `json:"channels,omitempty"`
	ChannelLayout  string `json:"channel_layout,omitempty"`
	BitsPerSample  int    `json:"bits_per_sample,omitempty"`
	InitialPadding int    `json:"initial_padding,omitempty"`

	// Common metrics (Note: ffprobe returns these as strings)
	RFrameRate       string `json:"r_frame_rate"`
	AvgFrameRate     string `json:"avg_frame_rate"`
	TimeBase         string `json:"time_base"`
	StartPts         int64  `json:"start_pts"`
	StartTime        string `json:"start_time"`
	DurationTs       int64  `json:"duration_ts"`
	Duration         string `json:"duration"`
	BitRate          string `json:"bit_rate"`
	BitsPerRawSample string `json:"bits_per_raw_sample,omitempty"`
	NbFrames         string `json:"nb_frames"`
	ExtradataSize    int    `json:"extradata_size"`

	Disposition  Disposition       `json:"disposition"`
	Tags         map[string]string `json:"tags,omitempty"`
	SideDataList []SideData        `json:"side_data_list,omitempty"`
}

// Format represents the container-level metadata
type Format struct {
	Filename       string            `json:"filename"`
	NbStreams      int               `json:"nb_streams"`
	NbPrograms     int               `json:"nb_programs"`
	FormatName     string            `json:"format_name"`
	FormatLongName string            `json:"format_long_name"`
	StartTime      string            `json:"start_time"`
	Duration       string            `json:"duration"`
	Size           string            `json:"size"`
	BitRate        string            `json:"bit_rate"`
	ProbeScore     int               `json:"probe_score"`
	Tags           map[string]string `json:"tags,omitempty"`
}

// Disposition contains boolean flags (0 or 1) for stream attributes
type Disposition struct {
	Default         int `json:"default"`
	Dub             int `json:"dub"`
	Original        int `json:"original"`
	Comment         int `json:"comment"`
	Lyrics          int `json:"lyrics"`
	Karaoke         int `json:"karaoke"`
	Forced          int `json:"forced"`
	HearingImpaired int `json:"hearing_impaired"`
	VisualImpaired  int `json:"visual_impaired"`
	CleanEffects    int `json:"clean_effects"`
	AttachedPic     int `json:"attached_pic"`
	TimedThumbnails int `json:"timed_thumbnails"`
	NonDiegetic     int `json:"non_diegetic"`
	Captions        int `json:"captions"`
	Descriptions    int `json:"descriptions"`
	Metadata        int `json:"metadata"`
	Dependent       int `json:"dependent"`
	StillImage      int `json:"still_image"`
}

// SideData contains extra stream information (like rotation)
type SideData struct {
	SideDataType  string `json:"side_data_type"`
	DisplayMatrix string `json:"displaymatrix,omitempty"`
	Rotation      int    `json:"rotation,omitempty"`
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
			log.Warnf(`"ffprobe" not found in PATH. Video/audio media info parsing will not work: %v`, err)
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
	cmd := exec.Command("ffprobe", "-v", "error", "-show_format", "-show_streams", "-print_format", "json", "-")
	cmd.Stdin = input
	var output []byte
	output, err = cmd.Output()
	if err != nil {
		return nil, err
	}
	info = &MediaFileInfo{}
	var videoProbeResult *FfprobeOutput
	err = json.Unmarshal(output, &videoProbeResult)
	if err != nil {
		return nil, err
	}
	if len(videoProbeResult.Streams) > 0 {
		info.Width = videoProbeResult.Streams[0].Width
		info.Height = videoProbeResult.Streams[0].Height
	}
	info.Duration = videoProbeResult.Format.Duration
	if videoProbeResult.Format.Tags != nil {
		if timeStr := videoProbeResult.Format.Tags["creation_time"]; timeStr != "" {
			if t, err := ParseCreationTime(timeStr); err == nil {
				info.Ctime = t
			} else {
				log.Warnf("failed to parse creation_time %q: %v", timeStr, err)
			}
		}
	}
	return info, nil
}

func ParseImageFileMediaInfo(input io.Reader) (mediainfo *MediaFileInfo, err error) {
	data, err := io.ReadAll(input)
	if err != nil {
		return nil, err
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}
	bounds := img.Bounds()
	signature := PixelDataHashAlphaAware(img)
	mediainfo = &MediaFileInfo{
		Width:     bounds.Dx(),
		Height:    bounds.Dy(),
		Signature: signature,
	}

	(func() {
		rawExif, err := exif.SearchAndExtractExif(data)
		if err != nil {
			if err != exif.ErrNoExif {
				log.Warnf("Failed to parse EXIF data: %v", err)
			}
			return
		}
		// Parse the raw EXIF data
		im, err := exifcommon.NewIfdMappingWithStandard()
		if err != nil {
			log.Warnf(`Could not create IfdMapping: %v`, err)
			return
		}
		ti := exif.NewTagIndex()

		_, index, err := exif.Collect(im, ti, rawExif)
		if err != nil {
			log.Warnf(`Could not collect EXIF data: %v`, err)
			return
		}

		// Locate the "Exif IFD" where DateTimeOriginal lives
		// (Root IFD usually has "DateTime", but "DateTimeOriginal" is inside the Exif sub-IFD)
		ifd, err := index.RootIfd.ChildWithIfdPath(exifcommon.IfdExifStandardIfdIdentity)
		if err != nil {
			log.Warnf(`Could not find Exif IFD: %v`, err)
			return
		}

		// Find the specific tag
		results, err := ifd.FindTagWithName("DateTimeOriginal")
		if err != nil {
			log.Printf("DateTimeOriginal not found, trying DateTime...")
			// Fallback to standard DateTime in Root if Original is missing
			results, err = index.RootIfd.FindTagWithName("DateTime")
			if err != nil {
				return
			}
		}
		dateString, err := results[0].Format()
		if err != nil {
			return
		}
		// Parse into a Go time object
		// Note: EXIF uses colons for the date part: "2006:01:02 15:04:05"
		t, err := ParseCreationTime(dateString)
		if err != nil {
			log.Warnf("Invalid EXIF creation time str %q", dateString)
			return
		}
		mediainfo.Ctime = t
	})()

	return mediainfo, nil
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

func ParseCreationTime(rawTime string) (time.Time, error) {
	// 1. Define the layouts you expect to encounter
	layouts := []string{
		"2006:01:02 15:04:05",
		// The one you saw (Micros + Z)
		"2006-01-02T15:04:05.000000Z",
		// Standard ISO 8601 (No micros)
		"2006-01-02T15:04:05Z",
		// ISO 8601 with timezone offset
		"2006-01-02T15:04:05-07:00",
		// Space separated (Common in MKV/WebM)
		"2006-01-02 15:04:05",
		time.RFC3339,
		time.RFC3339Nano,
	}

	// 2. Try parsing against each layout
	for _, layout := range layouts {
		if t, err := time.Parse(layout, rawTime); err == nil {
			return t, nil
		}
	}

	// 3. Fail if none match
	return time.Time{}, fmt.Errorf("could not parse time: %s", rawTime)
}
