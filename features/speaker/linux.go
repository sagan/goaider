//go:build !windows
// +build !windows

// we don't import "github.com/hegedustibor/htgo-tts" in Linux as it will break cross compile

package speaker

import (
	"fmt"
	"runtime"
)

func GetSpeaker(lang string) (Speaker, error) {
	return nil, fmt.Errorf("%s platform is not supported", runtime.GOOS)
}
