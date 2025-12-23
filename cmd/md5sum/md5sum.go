package md5sum

import (
	"github.com/sagan/goaider/cmd"
	"github.com/sagan/goaider/constants"
	"github.com/sagan/goaider/util/helper"
	"github.com/spf13/cobra"
)

var md5sumCmd = &cobra.Command{
	Use:   "md5sum [file]...",
	Short: "Calculate md5 hash of files",
	Long: `Calculate md5 hash of files.
	
If [file] is -, read from stdin.

It outputs in same format as Linux's "md5sum" util.`,
	RunE: doMd5sum,
}

var (
	flagForce  bool
	flagText   string
	flagOutput string
)

func doMd5sum(cmd *cobra.Command, args []string) (err error) {
	return helper.DoHashSum(constants.HASH_MD5, flagText, args, flagOutput, true, flagForce,
		cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
}

func init() {
	md5sumCmd.Flags().BoolVarP(&flagForce, "force", "", false, "Force overwriting without confirmation")
	md5sumCmd.Flags().StringVarP(&flagText, "text", "t", "", `Use text as input instead`)
	md5sumCmd.Flags().StringVarP(&flagOutput, "output", "o", "-", `Output file path. Use "-" for stdout`)
	cmd.RootCmd.AddCommand(md5sumCmd)
}
