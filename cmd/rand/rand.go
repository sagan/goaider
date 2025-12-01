package rand

import (
	"fmt"

	"github.com/sagan/goaider/cmd"
	"github.com/sagan/goaider/util"
	"github.com/spf13/cobra"
)

var randCmd = &cobra.Command{
	Use:     "rand",
	Aliases: []string{"random"},
	Short:   "Get a cryptographically secure random string of [a-zA-Z0-9]{22}",
	Long: `Get a cryptographically secure random string of [a-zA-Z0-9]{2}.

It outputs to stdout.`,
	RunE: doRand,
}

var (
	flagLength    int  // output length, default is 22 (130 bit security)
	flagDigitOnly bool // output digits ([0-9]) only
)

func doRand(cmd *cobra.Command, args []string) error {
	fmt.Print(util.RandString(flagLength, flagDigitOnly))
	return nil
}

func init() {
	randCmd.Flags().IntVarP(&flagLength, "length", "l", 22, "Length of the random string")
	randCmd.Flags().BoolVarP(&flagDigitOnly, "digits", "d", false, "Output digits only")
	cmd.RootCmd.AddCommand(randCmd)
}
