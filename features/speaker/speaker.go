package speaker

import (
	"os"
	"path/filepath"
)

const CACHE_FOLDER_NAME = ".gatts"

type Speaker interface {
	// Generate audio and play it.
	GenerateAndSpeak(text string) (filename string, err error)
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

func Play(lang string, text string) error {
	speaker, err := GetSpeaker(lang)
	if err != nil {
		return err
	}
	_, err = speaker.GenerateAndSpeak(text)
	if err != nil {
		return err
	}
	return nil
}
