package paste

import (
	"github.com/spf13/cobra"

	"github.com/sagan/goaider/cmd"
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
