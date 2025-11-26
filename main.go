package main

import (
	"os"
	"runtime"

	"github.com/sagan/goaider/cmd"
	_ "github.com/sagan/goaider/cmd/all"
)

func main() {
	if runtime.GOOS == "windows" {
		// https://github.com/golang/go/issues/43947
		os.Setenv("NoDefaultCurrentDirectoryInExePath", "1")
	}
	cmd.Execute()
}
