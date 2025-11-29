package paste

import (
	"github.com/spf13/cobra"

	"github.com/sagan/goaider/cmd"
)

// pasteCmd represents the copy command
var pasteCmd = &cobra.Command{
	Use:   "paste [filename]",
	Short: "paste clipboard to file. Windows only",
	Long: `paste clipboard to file. Windows only.

If [filename] is not provided, a "clipboard-<timestamp>" style name .txt or .png file
in dir (default to ".") is used, where <timestamp> is yyyyMMddHHmmss format.

On success, it outputs the full path of written file.`,
	Args: cobra.MaximumNArgs(1),
	RunE: doPaste,
}

var (
	flagDir   string // Manually specify output dir, if set, it's joined with filename
	flagForce bool   // override existing file
)

func init() {
	pasteCmd.Flags().StringVarP(&flagDir, "dir", "d", "", "Optional: output dir. default to current dir. "+
		"If both --dir flag and filename arg are set, the joined path is used.")
	pasteCmd.Flags().BoolVarP(&flagForce, "force", "", false, "Optional: override existing file.")
	cmd.RootCmd.AddCommand(pasteCmd)
}
