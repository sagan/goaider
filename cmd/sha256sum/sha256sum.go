package sha256sum

import (
	"github.com/spf13/cobra"

	"github.com/sagan/goaider/cmd"
	"github.com/sagan/goaider/constants"
	"github.com/sagan/goaider/util/helper"
)

var sha256sumCmd = &cobra.Command{
	Use:   "sha256sum [file]...",
	Short: "Calculate sha256 hash of files",
	Long: `Calculate sha256 hash of files.
	
If [file] is -, read from stdin.

It outputs in same format as Linux's "sha256sum" util.`,
	RunE: doSha256sum,
}

var (
	flagForce  bool
	flagText   string
	flagOutput string
)

func doSha256sum(cmd *cobra.Command, args []string) (err error) {
	return helper.DoHashSum(constants.HASH_SHA256, flagText, args, flagOutput, true, flagForce,
		cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
}

func init() {
	sha256sumCmd.Flags().BoolVarP(&flagForce, "force", "", false, "Force overwriting without confirmation")
	sha256sumCmd.Flags().StringVarP(&flagText, "text", "t", "", `Use text as input instead`)
	sha256sumCmd.Flags().StringVarP(&flagOutput, "output", "o", "-", `Output file path. Use "-" for stdout`)
	cmd.RootCmd.AddCommand(sha256sumCmd)
}
