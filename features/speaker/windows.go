//go:build windows
// +build windows

package speaker

import (
	"fmt"
	"path/filepath"
	"strings"

	htgotts "github.com/hegedustibor/htgo-tts"
	"github.com/hegedustibor/htgo-tts/handlers"
	"github.com/sagan/goaider/constants"
	"github.com/sagan/goaider/util"
)

type HtgoSpeaker struct {
	htgotts.Speech
}

func (s *HtgoSpeaker) GenerateAndSpeak(text string) (filename string, err error) {
	err = s.Speak(text)
	if err != nil {
		return "", err
	}
	md5, err := util.Hash(strings.NewReader(text), constants.HASH_MD5, true)
	if err != nil {
		return "", err
	}
	filename = filepath.Join(s.Folder, fmt.Sprintf("%s_%s.mp3", s.Language, md5))
	return filename, nil
}

func GetSpeaker(lang string) (Speaker, error) {
	tmpdir, err := GetCacheDir()
	if err != nil {
		return nil, err
	}
	return &HtgoSpeaker{
		htgotts.Speech{Folder: tmpdir, Language: lang, Handler: &handlers.Native{}},
	}, nil
}
