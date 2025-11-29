//go:build windows

package paste

import (
	"bytes"
	"fmt"
	"mime"
	"path/filepath"
	"strings"
	"time"

	"github.com/natefinch/atomic"
	"github.com/spf13/cobra"
	"golang.design/x/clipboard"

	"github.com/sagan/goaider/util"
	"github.com/sagan/goaider/util/helper"
)

func doPaste(cmd *cobra.Command, args []string) (err error) {
	err = clipboard.Init()
	if err != nil {
		return err
	}

	argFilename := ""
	fullpath := ""
	if len(args) > 0 {
		argFilename = args[0]
		fullpath = args[0]
	} else {
		fullpath = "clipboard-" + time.Now().Format("20060102150304")
	}
	fullpath = filepath.Join(flagDir, fullpath)
	fullpath, err = filepath.Abs(fullpath)
	if err != nil {
		return err
	}

	var data []byte
	// try image first
	if data = clipboard.Read(clipboard.FmtImage); len(data) > 0 {
		if argFilename == "" { // only append ext if filename is not provided by user
			fullpath += ".png"
		} else if ext := filepath.Ext(fullpath); ext != ".png" && ext != ".PNG" {
			return fmt.Errorf("clipboard is png image but filename ext is not")
		}
	} else if data = clipboard.Read(clipboard.FmtText); len(data) > 0 {
		if argFilename == "" {
			fullpath += ".txt"
		} else if ext := filepath.Ext(fullpath); strings.HasPrefix(mime.TypeByExtension(ext), "image/") {
			return fmt.Errorf("clipboard is text but provided filename is image ext")
		}
	} else {
		return fmt.Errorf("clipboard has no valid data")
	}

	if argFilename == "" {
		fullpath, err = helper.GetNewFilePath(fullpath, "")
		if err != nil {
			return err
		}
	} else if exists, err := util.FileExists(fullpath); err != nil || (exists && !flagForce) {
		return fmt.Errorf("target file %q already exists or access error. err: %w", fullpath, err)
	}

	err = atomic.WriteFile(fullpath, bytes.NewReader(data))
	if err != nil {
		return err
	}

	fmt.Print(fullpath)
	return nil
}
