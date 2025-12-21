package watch

import (
	"fmt"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/goaider/cmd"
	"github.com/sagan/goaider/util/helper"
)

var randCmd = &cobra.Command{
	Use:   "watch {cmdline}",
	Short: `Similar to Linux "watch" utility`,
	Long: `Similar to Linux "watch" utility.

Examples:
  goaider watch -n 10 echo foo bar
`,
	Args: cobra.MinimumNArgs(1),
	RunE: doWatch,
}

var (
	flagInterval int
	flagExec     bool
	flagErrExit  bool
)

func doWatch(cmd *cobra.Command, args []string) (err error) {
	if flagInterval <= 0 {
		return fmt.Errorf("invalid interval")
	}
	ticker := time.NewTicker(time.Duration(flagInterval) * time.Second)
	defer ticker.Stop()

	cmdline := strings.Join(args, " ")

	var run = func() error {
		err := helper.RunCmdline(cmdline, !flagExec, nil, cmd.OutOrStdout(), cmd.ErrOrStderr())
		if err != nil {
			if flagErrExit {
				return err
			} else {
				log.Errorf("Command failed: %v", err)
			}
		}
		return nil
	}

	if err = run(); err != nil {
		return err
	}

	for {
		select {
		case <-ticker.C:
			if err = run(); err != nil {
				return err
			}
		case <-cmd.Context().Done():
			return nil
		}
	}
}

func init() {
	randCmd.Flags().IntVarP(&flagInterval, "interval", "n", 2, "Specify update interval (seconds)")
	randCmd.Flags().BoolVarP(&flagExec, "exec", "x", false,
		`Exec process directly instead of using "cmd /C" or "sh -c"`)
	randCmd.Flags().BoolVarP(&flagErrExit, "errexit", "", false, "Exit if command returns a non-zero exit code")
	cmd.RootCmd.AddCommand(randCmd)
}
