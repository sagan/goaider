package copy

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/sagan/goaider/cmd"
	"github.com/sagan/goaider/features/clipboard"
)

// copyCmd represents the copy command
var copyCmd = &cobra.Command{
	Use:   "copy",
	Short: "Copy stdin to clipboard. Windows only",
	Long:  `Copy stdin to clipboard. Windows only.`,
	RunE:  doCopy,
}

var (
	flagImage bool // write to clipboard as image
)

func init() {
	cmd.RootCmd.AddCommand(copyCmd)
	copyCmd.Flags().BoolVarP(&flagImage, "image", "i", false, `Optional: write to clipboard as image. `+
		`Non-png image will be converted to png first`)
}

func doCopy(cmd *cobra.Command, args []string) error {
	err := clipboard.Init()
	if err != nil {
		return err
	}
	err = clipboard.Copy(os.Stdin, flagImage)
	if err != nil {
		return err
	}
	return nil
}
