package exec

// read csv file, execute a cmd for each line of the csv.
// the cmdling to exeute is generated from user provided --template flag,
// which is a Go text template, e.g. "mycmd {{.foo}} {{.bar}}",
// the context is the map[string]any data of each csv row.

import (
	"fmt"
	"io"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	csvCmd "github.com/sagan/goaider/cmd/csv"
	"github.com/sagan/goaider/constants"
	"github.com/sagan/goaider/util/stringutil"
)

var execCmd = &cobra.Command{
	Use:   "exec {input.csv | -}",
	Short: "Execute a command for each line of a CSV file",
	Long: `Execute a command for each line of a CSV file.

Example:
  goaider csv exec input.csv --template "mycmd {{.foo}}"`,
	RunE: doExec,
	Args: cobra.ExactArgs(1),
}

var (
	flagDryRun          bool
	flagContinueOnError bool
	flagTemplate        string
)

func init() {
	execCmd.Flags().StringVarP(&flagTemplate, "template", "t", "",
		`(Required) Template to build the cmdline to be executed for each row. E.g. "mycmd {{.foo}} {{.bar}}". `+
			constants.HELP_TEMPLATE_FLAG)
	execCmd.Flags().BoolVarP(&flagDryRun, "dry-run", "d", false, "Print the commands instead of executing them")
	execCmd.Flags().BoolVarP(&flagContinueOnError, "continue-on-error", "c", false,
		"Continue executing even if an error occurs for a row "+
			"(template render error, command execution error, or non-zero exit code)")
	execCmd.MarkFlagRequired("template")
	csvCmd.CsvCmd.AddCommand(execCmd)
}

func doExec(cmd *cobra.Command, args []string) (err error) {
	argInput := args[0]
	var input io.Reader
	if argInput == "-" {
		input = os.Stdin
	} else {
		f, err := os.Open(argInput)
		if err != nil {
			return err
		}
		defer f.Close()
		input = f
	}
	input = stringutil.GetTextReader(input)
	successRows, skipRows, errorRows, err := execCsv(input, flagTemplate, csvCmd.FlagNoHeader,
		flagContinueOnError, flagDryRun)
	log.Printf("Complete: success / skip / error rows: %d / %d / %d", successRows, skipRows, errorRows)
	if err != nil {
		return err
	}
	if errorRows > 0 {
		return fmt.Errorf("%d rows failed", errorRows)
	}
	return nil
}
