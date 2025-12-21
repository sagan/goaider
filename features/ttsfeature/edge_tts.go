package ttsfeature

import (
	"bytes"
	"fmt"

	"github.com/natefinch/atomic"
	"github.com/wujunwei928/edge-tts-go/edge_tts"
)

type EdgeTts struct {
	SpeakerBasics
}

func (e *EdgeTts) Generate(text string) (filename string, err error) {
	// https://github.com/rany2/edge-tts
	// edge-tts --list-voices
	// https://huggingface.co/spaces/innoai/Edge-TTS-Text-to-Speech
	// 选择标准: 1. female only. 2. 优先 multilingual 版本.
	langs := map[string]string{
		"en":    "en-US-AvaMultilingualNeural",
		"ja":    "ja-JP-NanamiNeural",
		"fr":    "fr-FR-VivienneMultilingualNeural",
		"de":    "de-DE-SeraphinaMultilingualNeural",
		"es":    "es-ES-XimenaNeural",
		"pt":    "pt-PT-RaquelNeural",
		"ko":    "ko-KR-SunHiNeural",
		"ru":    "ru-RU-SvetlanaNeural",
		"ar":    "ar-EG-SalmaNeural",
		"zh-tw": "zh-TW-HsiaoChenNeural",
		"zh":    "zh-CN-XiaoxiaoNeural",
		"zh-cn": "zh-CN-XiaoxiaoNeural",
		"cht":   "zh-TW-HsiaoChenNeural",
		"chs":   "zh-CN-XiaoxiaoNeural",
	}

	filename = e.GenFilename(text)

	// Voice to use
	voice, ok := langs[e.Lang]
	if !ok {
		return "", fmt.Errorf("unsupported lang %s", e.Lang)
	}

	connOptions := []edge_tts.CommunicateOption{
		edge_tts.SetVoice(voice),
		edge_tts.SetRate("+0%"),
		edge_tts.SetVolume("+0%"),
		edge_tts.SetPitch("+0Hz"),
		edge_tts.SetReceiveTimeout(20),
	}

	conn, err := edge_tts.NewCommunicate(
		text,
		connOptions...,
	)
	if err != nil {
		return "", err
	}
	audioData, err := conn.Stream()
	if err != nil {
		return "", err
	}
	err = atomic.WriteFile(filename, bytes.NewReader(audioData))
	return filename, err
}

var _ Speaker = (*EdgeTts)(nil)
