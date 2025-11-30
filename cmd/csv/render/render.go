package render

import (
	"io"

	"github.com/spf13/cobra"

	csvCmd "github.com/sagan/goaider/cmd/csv"
	"github.com/sagan/goaider/constants"
	"github.com/sagan/goaider/util/helper"
)

var renderCmd = &cobra.Command{
	Use:     "render <input.csv | ->",
	Aliases: []string{"display", "show"},
	Short:   "Render the contents of a csv file",
	Long: `Render the contents of a csv file.

Example:
  goaider csv render input.csv --template "{{.foo}} - {{.bar}}"`,
	RunE: doRender,
	Args: cobra.ExactArgs(1),
}

var (
	flagTemplate string
	flagOneLine  bool
)

func init() {
	renderCmd.Flags().StringVarP(&flagTemplate, "template", "t", "",
		`(Required) Go template string to render the content for each csv row. E.g. "{{.foo}} {{.bar}}". `+
			constants.HELP_TEMPLATE_FLAG)
	renderCmd.Flags().BoolVarP(&flagOneLine, "one-line", "", false,
		`Force each csv row outputs only one line. This is useful when a field contains newlines. `+
			`It replaces one or more consecutive newline characters (\r, \n) with single space`)
	renderCmd.MarkFlagRequired("template")
	csvCmd.CsvCmd.AddCommand(renderCmd)
}

func doRender(cmd *cobra.Command, args []string) (err error) {
	argInput := args[0]

	err = helper.InputFileAndOutput(argInput, csvCmd.FlagOutput, csvCmd.FlagForce, func(r io.Reader,
		w io.Writer, inputName, outputNme string) error {
		return renderCsv(r, flagTemplate, csvCmd.FlagNoHeader, w, flagOneLine)
	})
	if err != nil {
		return err
	}
	return nil
}
