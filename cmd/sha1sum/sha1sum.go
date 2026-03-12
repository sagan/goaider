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
	flagForce    bool
	flagBase64   bool
	flagHashOnly bool
	flagText     string
	flagOutput   string
)

func doSha1sum(cmd *cobra.Command, args []string) (err error) {
	return helper.DoHashSum(constants.HASH_SHA1, flagText, args, flagOutput, !flagBase64, flagForce,
		cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr(), flagHashOnly)
}

func init() {
	sha1sumCmd.Flags().BoolVarP(&flagForce, "force", "", false, "Force overwriting without confirmation")
	sha1sumCmd.Flags().BoolVarP(&flagBase64, "base64", "b", false, "Output base64 string")
	sha1sumCmd.Flags().BoolVarP(&flagHashOnly, "hash-only", "", false, "Output hash only, no filename")
	sha1sumCmd.Flags().StringVarP(&flagText, "text", "t", "", `Use text as input instead`)
	sha1sumCmd.Flags().StringVarP(&flagOutput, "output", "o", "-", `Output file path. Use "-" for stdout`)
	cmd.RootCmd.AddCommand(sha1sumCmd)
}
