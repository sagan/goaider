package exec

// read csv file, execute a cmd for each line of the csv.
// the cmdling to exeute is generated from user provided --template flag,
// which is a Go text template, e.g. "mycmd {{.foo}} {{.bar}}",
// the context is the map[string]any data of each csv row.

import (
	csvCmd "github.com/sagan/goaider/cmd/csv"
	"github.com/spf13/cobra"
)

var execCmd = &cobra.Command{
	Use:   "exec <input.csv | ->",
	Short: "Execute a command for each line of a CSV file",
	RunE:  doExec,
}

var (
	flagDryRun   bool
	flagTemplate string
)

func init() {
	execCmd.Flags().StringVarP(&flagTemplate, "template", "t", "",
		`Go template string to build the command to be executed for each row. E.g. "mycmd {{.foo}} {{.bar}}"`)
	execCmd.Flags().BoolVarP(&flagDryRun, "dry-run", "", false, "Print the commands instead of executing them")
	execCmd.MarkFlagRequired("template")
	csvCmd.CsvCmd.AddCommand(execCmd)
}

func doExec(cmd *cobra.Command, args []string) error {
	return nil
}
