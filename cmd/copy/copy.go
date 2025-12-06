package copy

import (
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sagan/goaider/cmd"
	"github.com/sagan/goaider/features/clipboard"
	"github.com/sagan/goaider/util"
)

// copyCmd represents the copy command
var copyCmd = &cobra.Command{
	Use:   "copy",
	Short: "Copy stdin to clipboard. Windows only",
	Long:  `Copy stdin to clipboard. Windows only.`,
	RunE:  doCopy,
}

var (
	flagImage bool   // write to clipboard as image
	flagInput string // copy file to clipboard instead
)

func init() {
	cmd.RootCmd.AddCommand(copyCmd)
	copyCmd.Flags().BoolVarP(&flagImage, "image", "I", false, `Optional: write to clipboard as image. `+
		`Non-png image will be converted to png first`)
	copyCmd.Flags().StringVarP(&flagInput, "input", "i", "", `Optional: copy file to clipboard instead of stdin. `+
		`If set, stdin is ignored`)
}

func doCopy(cmd *cobra.Command, args []string) error {
	err := clipboard.Init()
	if err != nil {
		return err
	}
	var input io.Reader
	if flagInput == "" || flagInput == "-" {
		input = os.Stdin
	} else {
		f, err := os.Open(flagInput)
		if err != nil {
			return err
		}
		defer f.Close()
		input = f
		flagImage = flagImage || strings.HasPrefix(util.GetMimeType(flagInput), "image/")
	}

	err = clipboard.Copy(input, flagImage)
	if err != nil {
		return err
	}
	return nil
}
