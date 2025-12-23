package crc32sum

import (
	"github.com/sagan/goaider/cmd"
	"github.com/sagan/goaider/constants"
	"github.com/sagan/goaider/util/helper"
	"github.com/spf13/cobra"
)

var crc32sumCmd = &cobra.Command{
	Use:   "crc32sum [file]...",
	Short: "Calculate crc32 hash of files",
	Long: `Calculate crc32 hash of files.
	
If [file] is -, read from stdin.

It uses IEEE standard crc32 polynomial of 0xedb88320.
`,
	RunE: doCrc32sum,
}

var (
	flagForce  bool
	flagDigit  bool
	flagText   string
	flagOutput string
)

func doCrc32sum(cmd *cobra.Command, args []string) (err error) {
	return helper.DoHashSum(constants.HASH_CRC32, flagText, args, flagOutput, !flagDigit, flagForce,
		cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
}

func init() {
	crc32sumCmd.Flags().BoolVarP(&flagForce, "force", "", false, "Force overwriting without confirmation")
	crc32sumCmd.Flags().BoolVarP(&flagDigit, "digit", "d", false, "Output crc32 in digit number form")
	crc32sumCmd.Flags().StringVarP(&flagText, "text", "t", "", `Use text as input instead`)
	crc32sumCmd.Flags().StringVarP(&flagOutput, "output", "o", "-", `Output file path. Use "-" for stdout`)
	cmd.RootCmd.AddCommand(crc32sumCmd)
}
