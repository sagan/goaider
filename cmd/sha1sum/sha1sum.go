package sha1sum

import (
	"github.com/spf13/cobra"

	"github.com/sagan/goaider/cmd"
	"github.com/sagan/goaider/constants"
	"github.com/sagan/goaider/util/helper"
)

var sha1sumCmd = &cobra.Command{
	Use:   "sha1sum [file]...",
	Short: "Calculate sha1 hash of files",
	Long: `Calculate sha1 hash of files.
	
If [file] is -, read from stdin.

It outputs in same format as Linux's "sha1sum" util.`,
	RunE: doSha1sum,
}

var (
	flagForce  bool
	flagText   string
	flagOutput string
)

func doSha1sum(cmd *cobra.Command, args []string) (err error) {
	return helper.DoHashSum(constants.HASH_SHA1, flagText, args, flagOutput, true, flagForce,
		cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
}

func init() {
	sha1sumCmd.Flags().BoolVarP(&flagForce, "force", "", false, "Force overwriting without confirmation")
	sha1sumCmd.Flags().StringVarP(&flagText, "text", "t", "", `Use text as input instead`)
	sha1sumCmd.Flags().StringVarP(&flagOutput, "output", "o", "-", `Output file path. Use "-" for stdout`)
	cmd.RootCmd.AddCommand(sha1sumCmd)
}
