package copy

import (
	"github.com/spf13/cobra"

	"github.com/sagan/goaider/cmd"
)

// copyCmd represents the copy command
var copyCmd = &cobra.Command{
	Use:   "copy",
	Short: "copy stdin to clipboard. Windows only",
	Long:  `copy stdin to clipboard. Windows only.`,
	RunE:  copyFunc,
}

var (
	flagImage bool // write to clipboard as image
)

func init() {
	cmd.RootCmd.AddCommand(copyCmd)
	copyCmd.Flags().BoolVarP(&flagImage, "image", "i", false, `Optional: write to clipboard as image. `+
		`Non-png image will be converted to png first.`)
}
