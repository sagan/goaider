package randu

import (
	"fmt"
	"os"
	"strings"

	"github.com/natefinch/atomic"
	"github.com/sagan/goaider/cmd"
	"github.com/sagan/goaider/util"
	"github.com/spf13/cobra"
)

var randCmd = &cobra.Command{
	Use:     "randu",
	Aliases: []string{"randuuid"},
	Short:   "Get a cryptographically secure random uuid",
	Long: `Get a cryptographically secure random uuid.
	
E.g. 69e4c430-8daa-bd5e-c8e4-e9a2c3d0050c`,
	RunE: doRandu,
}

var (
	flagForce  bool
	flagOutput string
)

func doRandu(cmd *cobra.Command, args []string) (err error) {
	if flagOutput != "-" {
		if exists, err := util.FileExists(flagOutput); err != nil || (exists && !flagForce) {
			return fmt.Errorf("output file %q exists or can't access, err=%w", flagOutput, err)
		}
	}

	b := util.RandBytes(16)
	s := fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])

	if flagOutput == "-" {
		_, err = os.Stdout.WriteString(s)
	} else {
		err = atomic.WriteFile(flagOutput, strings.NewReader(s))
	}
	if err != nil {
		return err
	}
	return nil
}

func init() {
	randCmd.Flags().BoolVarP(&flagForce, "force", "", false, "Force overwriting without confirmation")
	randCmd.Flags().StringVarP(&flagOutput, "output", "o", "-", `Output file path. Use "-" for stdout`)
	cmd.RootCmd.AddCommand(randCmd)
}
