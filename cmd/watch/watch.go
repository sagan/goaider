package watch

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/goaider/cmd"
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

	for {
		select {
		case <-ticker.C:
			var command *exec.Cmd
			if flagExec {
				command = exec.Command(args[0], args[1:]...)
			} else {
				if runtime.GOOS == "windows" {
					command = exec.Command("cmd", "/C", strings.Join(args, " "))
				} else {
					command = exec.Command("sh", "-c", strings.Join(args, " "))
				}
			}
			command.Stdout = cmd.OutOrStdout()
			command.Stderr = cmd.ErrOrStderr()
			err := command.Run()
			if err != nil {
				if flagErrExit {
					return err
				} else {
					log.Errorf("Command failed: %v", err)
				}
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
	randCmd.Flags().BoolVarP(&flagErrExit, "errexit", "e", false, "Exit if command returns a non-zero exit code")
	cmd.RootCmd.AddCommand(randCmd)
}
