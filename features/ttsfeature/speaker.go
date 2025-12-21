package ttsfeature

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/hajimehoshi/go-mp3"
	"github.com/natefinch/atomic"
	log "github.com/sirupsen/logrus"

	"github.com/sagan/goaider/constants"
	"github.com/sagan/goaider/util"
)

const CACHE_FOLDER_NAME = ".gatts"

type Speaker interface {
	// Generate audio mp3 file
	Generate(text string) (filename string, err error)
}

var (
	initializeOnce sync.Once
	Ffmpeg         string // ffmpeg binary path
)

// Init function. Safe to call multiple times.
func Init() {
	initializeOnce.Do(func() {
		ffmpeg := os.Getenv(constants.ENV_FFMPEG)
		switch ffmpeg {
		case constants.NULL:
			log.Printf("ttsfeature: force disable ffmpeg")
			return
		case "":
			ffmpeg, _ = exec.LookPath(constants.FFMPEG)
		}
		if ffmpeg == "" {
			log.Tracef("ttsfeature: ffmpeg not found. some format audio files are not supported. " +
				"Install ffmpeg in PATH or set " + constants.ENV_FFMPEG + " env to it's binary path")
		}
		Ffmpeg = ffmpeg
	})
}

func CleanCacheDir() error {
	cacheDir, err := GetCacheDir()
	if err != nil {
		return err
	}
	return os.RemoveAll(cacheDir)
}

// Create cache dir and return
func GetCacheDir() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	tmpdir := filepath.Join(cacheDir, CACHE_FOLDER_NAME)

	err = os.MkdirAll(tmpdir, 0755)
	if err != nil {
		return "", err
	}
	return tmpdir, nil
}

func PlayText(lang string, text string) error {
	speaker, err := NewSpeaker(lang)
	if err != nil {
		return err
	}
	filename, err := speaker.Generate(text)
	if err != nil {
		return err
	}
	return PlayAudioFile(filename)
}

type SpeakerBasics struct {
	Lang   string
	Id     string // engine identifier
	Folder string
}

func (s *SpeakerBasics) GenFilename(text string) string {
	// even if it's a case insensitive file system, base64 sha256 filename still has 220+ bits security
	hash, err := util.Hash(strings.NewReader(text), constants.HASH_SHA256, false)
	if err != nil {
		panic(err)
	}
	if s.Id != "" {
		return filepath.Join(s.Folder, fmt.Sprintf("%s_%s_%s.mp3", s.Id, s.Lang, hash))
	}
	return filepath.Join(s.Folder, fmt.Sprintf("%s_%s.mp3", s.Lang, hash))
}

type OnlineSpeaker struct {
	SpeakerBasics
	GetRequest func(lang, text string) (*http.Request, error)
	Client     *http.Client
}

var _ Speaker = (*OnlineSpeaker)(nil)

func (s *OnlineSpeaker) Generate(text string) (filename string, err error) {
	filename = s.GenFilename(text)
	exists, err := util.FileExists(filename)
	if err != nil {
		return "", err
	}
	if exists {
		return filename, nil
	}
	req, err := s.GetRequest(s.Lang, text)
	if err != nil {
		return "", err
	}
	client := s.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("status=%d", resp.StatusCode)
	}
	err = atomic.WriteFile(filename, resp.Body)
	if err != nil {
		return "", err
	}
	return filename, nil
}

func NewSpeaker(lang string) (Speaker, error) {
	tmpdir, err := GetCacheDir()
	if err != nil {
		return nil, err
	}
	tts := os.Getenv(constants.ENV_TTS)
	if tts == "" {
		tts = constants.DEFAULT_TTS
	}
	switch tts {
	case constants.TTS_EDGE:
		return &EdgeTts{
			SpeakerBasics: SpeakerBasics{
				Folder: tmpdir,
				Id:     constants.TTS_EDGE,
				Lang:   lang,
			},
		}, nil
	case constants.TTS_GOOGLE:
		return &OnlineSpeaker{
			SpeakerBasics: SpeakerBasics{
				Folder: tmpdir,
				Id:     constants.TTS_GOOGLE,
				Lang:   lang,
			},
			GetRequest: func(lang, text string) (*http.Request, error) {
				req, err := http.NewRequest(http.MethodGet, fmt.Sprintf(
					"http://translate.google.com/translate_tts?ie=UTF-8&total=1&idx=0&textlen=32&client=tw-ob&q=%s&tl=%s",
					url.QueryEscape(text), lang), nil,
				)
				return req, err
			},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported tts engine %q. Supported: %s, %s",
			tts, constants.TTS_EDGE, constants.TTS_GOOGLE)
	}
}

func PlayAudioFile(fileName string) error {
	file, err := os.Open(fileName)
	if err != nil {
		return err
	}
	defer file.Close()
	return PlayAudio(file, fileName)
}

// Get a oto compatible audio stream from raw input.
// filename is optional, help detect the audio file type.
// It supports wav & mp3 internally;
// For other format, it tries to use ffmpeg to convert input to wav.
func GetOtoAudio(input io.Reader, filename string) (audio io.Reader, sampleRate int, channelCount int, err error) {
	input, contentType, err := util.DetectContentType(input)
	if err != nil {
		return nil, 0, 0, err
	}
	mimeType, _, _ := strings.Cut(contentType, ";")

	// Go http.DetectContentType fails to detect some audio file (like mp3)
	if mimeType == constants.MIME_BINARY {
		mimeType = util.GetMimeType(filename)
	}
	switch mimeType {
	case constants.MIME_WAV:
		sampleRate, channelCount, err := ReadWavHeader(input)
		if err != nil {
			return nil, 0, 0, err
		}
		return input, sampleRate, channelCount, nil
	case constants.MIME_MP3:
		decodedMp3, err := mp3.NewDecoder(input)
		if err != nil {
			return nil, 0, 0, err
		}
		return decodedMp3, decodedMp3.SampleRate(), 2, nil
	}

	if Ffmpeg != "" {
		// Use ffmpeg to decode audio to wav format
		cmd := exec.Command(Ffmpeg, "-i", "-", "-f", "wav", "-")
		var stderr bytes.Buffer
		cmd.Stdin = input
		cmd.Stderr = &stderr
		audio, err := cmd.StdoutPipe()
		if err != nil {
			return nil, 0, 0, fmt.Errorf("failed to create stdout pipe for ffmpeg: %w", err)
		}
		if err := cmd.Start(); err != nil {
			return nil, 0, 0, fmt.Errorf("failed to start ffmpeg: %w, stderr: %s", err, stderr.String())
		}
		sampleRate, channelCount, err := ReadWavHeader(audio)
		if err != nil {
			return nil, 0, 0, err
		}
		return audio, sampleRate, channelCount, nil
	}
	return nil, 0, 0, fmt.Errorf("unknown or unsupported audio file, mime=%s", mimeType)
}

// ReadWavHeader parses the RIFF header and all subchunks until it finds the 'data' chunk.
// It returns the sample rate, channel count, and any error encountered.
func ReadWavHeader(input io.Reader) (sampleRate int, channelCount int, err error) {
	// 1. Read the Master RIFF Header (12 bytes)
	// [0:4] "RIFF", [4:8] FileSize, [8:12] "WAVE"
	riffHeader := make([]byte, 12)
	if _, err := io.ReadFull(input, riffHeader); err != nil {
		return 0, 0, fmt.Errorf("failed to read RIFF header: %w", err)
	}
	if string(riffHeader[:4]) != "RIFF" || string(riffHeader[8:12]) != "WAVE" {
		return 0, 0, errors.New("not a valid WAV file")
	}

	foundFmt := false

	// 2. Iterate through subchunks until we find 'data'
	for {
		chunkHeader := make([]byte, 8)
		if _, err := io.ReadFull(input, chunkHeader); err != nil {
			return 0, 0, fmt.Errorf("failed to read chunk header: %w", err)
		}

		chunkID := string(chunkHeader[:4])
		chunkSize := binary.LittleEndian.Uint32(chunkHeader[4:8])

		switch chunkID {
		case "fmt ":
			// Parse the format chunk to get sample rate and channels
			fmtBody := make([]byte, 16) // Standard PCM fmt is 16 bytes
			if _, err := io.ReadFull(input, fmtBody); err != nil {
				return 0, 0, err
			}

			channelCount = int(binary.LittleEndian.Uint16(fmtBody[2:4]))
			sampleRate = int(binary.LittleEndian.Uint32(fmtBody[4:8]))
			foundFmt = true

			// If the fmt chunk is larger than 16 bytes (e.g. 18 or 40), skip the rest
			if chunkSize > 16 {
				if _, err := io.CopyN(io.Discard, input, int64(chunkSize-16)); err != nil {
					return 0, 0, err
				}
			}

		case "data":
			// We found the audio data!
			// Ensure we found 'fmt' first, as its metadata is required to interpret 'data'
			if !foundFmt {
				return 0, 0, errors.New("data chunk found before fmt chunk")
			}
			// The reader is now positioned at the start of the audio stream
			return sampleRate, channelCount, nil

		default:
			// Skip unknown chunks (LIST, JUNK, bext, etc.)
			if _, err := io.CopyN(io.Discard, input, int64(chunkSize)); err != nil {
				return 0, 0, fmt.Errorf("failed to skip chunk %s: %w", chunkID, err)
			}
		}
	}
}
