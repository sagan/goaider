package paste

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/natefinch/atomic"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/sagan/goaider/cmd"
	"github.com/sagan/goaider/features/clipboard"
	"github.com/sagan/goaider/util"
	"github.com/sagan/goaider/util/helper"
)

// pasteCmd represents the copy command
var pasteCmd = &cobra.Command{
	Use:   "paste [filename]",
	Short: "Paste clipboard to file. Windows only",
	Long: `Paste clipboard to file. Windows only.

- If [filename] is "-", it outputs clipboard contents to stdout.
- If [filename] is not "-", it outputs clipboard contents to the file
  and outputs the full path of written file to stdout on success.
- If [filename] is not set, a "clipboard-<timestamp>" style name .txt or .png file
  in dir (default to ".") is used, where <timestamp> is yyyyMMddHHmmss format.`,
	Args: cobra.MaximumNArgs(1),
	RunE: doPaste,
}

var (
	flagDir   string // Manually specify output dir, if set, it's joined with filename
	flagForce bool   // override existing file
)

func init() {
	pasteCmd.Flags().StringVarP(&flagDir, "dir", "d", "", "Optional: output dir. Defaults to current dir. "+
		"If both --dir flag and [filename] arg are set, the joined path of them is used")
	pasteCmd.Flags().BoolVarP(&flagForce, "force", "", false, "Optional: override existing file")
	cmd.RootCmd.AddCommand(pasteCmd)
}

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
	if argFilename != "-" {
		fullpath = filepath.Join(flagDir, fullpath)
		fullpath, err = filepath.Abs(fullpath)
		if err != nil {
			return err
		}
	}

	var data []byte
	data, isImage, err := clipboard.Get()
	if err != nil {
		return err
	}
	if isImage {
		if argFilename == "" { // only append ext if filename is not provided by user
			fullpath += ".png"
		} else if argFilename == "-" {
			if term.IsTerminal(int(os.Stdout.Fd())) && !flagForce {
				return fmt.Errorf("clipboard is image but stdout is tty, refuse to write")
			}
		} else if ext := filepath.Ext(fullpath); ext != ".png" && ext != ".PNG" {
			return fmt.Errorf("clipboard is png image but filename ext is not")
		}
	} else if len(data) > 0 {
		if argFilename == "" {
			fullpath += ".txt"
		} else if argFilename != "-" {
			if strings.HasPrefix(util.GetMimeType(fullpath), "image/") {
				return fmt.Errorf("clipboard is text but provided filename is image ext")
			}
		}
	} else {
		return fmt.Errorf("clipboard has no valid data")
	}

	if argFilename == "" {
		fullpath, err = helper.GetNewFilePath(fullpath, "")
		if err != nil {
			return err
		}
	} else if argFilename == "-" {
		_, err = os.Stdout.Write(data)
		return err
	} else if exists, err := util.FileExists(fullpath); err != nil || (exists && !flagForce) {
		return fmt.Errorf("target file %q already exists or access error. err: %w", fullpath, err)
	}

	err = atomic.WriteFile(fullpath, bytes.NewReader(data))
	if err != nil {
		return err
	}

	if argFilename != "-" {
		fmt.Print(fullpath)
	}
	return nil
}
