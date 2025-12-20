package rand

import (
	"fmt"
	"strings"

	"github.com/natefinch/atomic"
	"github.com/spf13/cobra"

	"github.com/sagan/goaider/cmd"
	"github.com/sagan/goaider/util"
)

var randCmd = &cobra.Command{
	Use:     "rand",
	Aliases: []string{"random"},
	Short:   "Get a cryptographically secure random string of [a-zA-Z0-9]{22}",
	Long: `Get a cryptographically secure random string of [a-zA-Z0-9]{22}.

It outputs to stdout.`,
	RunE: doRand,
}

var (
	flagForce     bool
	flagDigitOnly bool // output digits ([0-9]) only
	flagLength    int  // output length, default is 22 (130 bit security)
	flagOutput    string
)

func doRand(cmd *cobra.Command, args []string) (err error) {
	if flagOutput != "-" {
		if exists, err := util.FileExists(flagOutput); err != nil || (exists && !flagForce) {
			return fmt.Errorf("output file %q exists or can't access, err=%w", flagOutput, err)
		}
	}
	if flagLength <= 0 {
		return fmt.Errorf("length must be greater than 0")
	}

	data := util.RandString(flagLength, flagDigitOnly)
	if flagOutput == "-" {
		_, err = cmd.OutOrStdout().Write([]byte(data))
	} else {
		err = atomic.WriteFile(flagOutput, strings.NewReader(data))
	}
	if err != nil {
		return err
	}
	return nil
}

func init() {
	randCmd.Flags().IntVarP(&flagLength, "length", "l", 22, "Length of the random string")
	randCmd.Flags().BoolVarP(&flagDigitOnly, "digits", "d", false, "Output digits only")
	randCmd.Flags().BoolVarP(&flagForce, "force", "", false, "Force overwriting without confirmation")
	randCmd.Flags().StringVarP(&flagOutput, "output", "o", "-", `Output file path. Use "-" for stdout`)
	cmd.RootCmd.AddCommand(randCmd)
}
