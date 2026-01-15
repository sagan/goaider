package mustrun

import (
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/sagan/goaider/cmd"
	"github.com/sagan/goaider/util"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var mustrunCmd = &cobra.Command{
	Use:   "mustrun {cmdline}",
	Short: `Run a cmdline until it successed`,
	Long: `Run a cmdline until it successed.

It runs the command repeatedly until it returns a zero exit code.
Between each failed attempt, it waits for a duration that increases exponentially (up to 30 seconds).
Its's recommanded to put a "--" in the beginning of the cmdline to avoid issues with argument parsing.
The command should be idempotent.

By default the cmdline is parsed by "sh -c" / "cmd /C", use "--exec" flag to execute the cmdline directly.

Examples:
  goaider mustrun -- curl https://example.com -o test.html
`,
	Args: cobra.MinimumNArgs(1),
	RunE: doMustrun,
}

var (
	flagExec bool
)

func doMustrun(cmd *cobra.Command, args []string) (err error) {
	for i := 0; ; i++ {
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
		log.Printf("No. %d try: %v", i+1, command.Args)
		err := command.Run()
		if err == nil {
			log.Printf("Success")
			return nil
		}
		log.Errorf("Command failed: %v", err)
		time.Sleep(util.CalculateBackoff(1*time.Second, 30*time.Second, i))
	}
}

func init() {
	mustrunCmd.Flags().BoolVarP(&flagExec, "exec", "x", false,
		`Exec process directly instead of using "cmd /C" or "sh -c"`)
	cmd.RootCmd.AddCommand(mustrunCmd)
}
