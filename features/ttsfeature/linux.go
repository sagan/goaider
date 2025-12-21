//go:build !windows
// +build !windows

package ttsfeature

import (
	"fmt"
	"io"
	"runtime"
)

func PlayAudio(input io.Reader, filename string) error {
	return fmt.Errorf("play audio is not supported in %s platform", runtime.GOOS)
}
