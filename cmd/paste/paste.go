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

Output file path can be set by [filename] or --output <filename>.

- If {filename} is "-", it outputs clipboard contents to stdout.
- If {filename} is not "-", it outputs clipboard contents to the file
  and outputs the full path of written file to stdout on success.
- If {filename} is not set, a "clipboard-<timestamp>" style name .txt or .png file
  in dir (default to ".") is used, where <timestamp> is yyyyMMddHHmmss format.`,
	Args: cobra.MaximumNArgs(1),
	RunE: doPaste,
}

var (
	flagForce     bool   // override existing file
	flagOutputDir string // Manually specify output dir, if set, it's joined with filename
	flagOutput    string
)

func init() {
	pasteCmd.Flags().BoolVarP(&flagForce, "force", "", false, "Optional: override existing file")
	pasteCmd.Flags().StringVarP(&flagOutputDir, "output-dir", "O", ".", "Optional: output dir. "+
		"If both --dir flag and {filename} are set, the joined path is used")
	pasteCmd.Flags().StringVarP(&flagOutput, "output", "o", "", `Output file path. Use "-" for stdout`)
	cmd.RootCmd.AddCommand(pasteCmd)
}

func doPaste(cmd *cobra.Command, args []string) (err error) {
	err = clipboard.Init()
	if err != nil {
		return err
	}
	err = os.MkdirAll(flagOutputDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create output directory %q: %w", flagOutputDir, err)
	}

	fullpath := ""
	if len(args) > 0 && flagOutput != "" {
		return fmt.Errorf("--output flag and [filename] argument can NOT be both set")
	} else if len(args) > 0 {
		flagOutput = args[0]
		fullpath = args[0]
	} else if flagOutput != "" {
		fullpath = flagOutput
	} else {
		fullpath = "clipboard-" + time.Now().UTC().Format("20060102150304")
	}
	if flagOutput != "-" {
		fullpath = filepath.Join(flagOutputDir, fullpath)
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
		if flagOutput == "" { // only append ext if filename is not provided by user
			fullpath += ".png"
		} else if flagOutput == "-" {
			if cmd.OutOrStdout() == os.Stdout && term.IsTerminal(int(os.Stdout.Fd())) && !flagForce {
				return fmt.Errorf("clipboard is image but stdout is tty, refuse to write")
			}
		} else if ext := filepath.Ext(fullpath); ext != ".png" && ext != ".PNG" {
			return fmt.Errorf("clipboard is png image but filename ext is not")
		}
	} else if len(data) > 0 {
		if flagOutput == "" {
			fullpath += ".txt"
		} else if flagOutput != "-" {
			if strings.HasPrefix(util.GetMimeType(fullpath), "image/") {
				return fmt.Errorf("clipboard is text but provided filename is image ext")
			}
		}
	} else {
		return fmt.Errorf("clipboard has no valid data")
	}

	if flagOutput == "" {
		fullpath, err = helper.GetNewFilePath(fullpath, "")
		if err != nil {
			return err
		}
	} else if flagOutput == "-" {
		_, err = cmd.OutOrStdout().Write(data)
		return err
	} else if exists, err := util.FileExists(fullpath); err != nil || (exists && !flagForce) {
		return fmt.Errorf("target file %q already exists or access error. err: %w", fullpath, err)
	}

	err = atomic.WriteFile(fullpath, bytes.NewReader(data))
	if err != nil {
		return err
	}

	if flagOutput != "-" {
		fmt.Print(fullpath)
	}
	return nil
}
