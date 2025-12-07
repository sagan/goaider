package sort

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/sagan/goaider/cmd/csv"
	"github.com/sagan/goaider/util/helper"
)

var sortCmd = &cobra.Command{
	Use:   "sort --key <key_field> {input.csv | -}",
	Short: "Sort csv file based on key field",
	Long: `Sort csv file based on key field.

The {input.csv} argument can be "-" for reading from stdin.

Output to stdout by default. If --inplace is set, update input file in place.
Use "-" as input arg to read from stdin.`,
	Args: cobra.ExactArgs(1),
	RunE: sortFunc,
}

var (
	flagKey     string // key field
	flagInplace bool   // update input file in place
)

func sortFunc(cmd *cobra.Command, args []string) (err error) {
	argInput := args[0]

	if flagInplace {
		if csv.FlagOutput != "-" {
			return fmt.Errorf("--inplace and --output flags are NOT compatible")
		}
		if argInput == "-" {
			return fmt.Errorf("stdin input is NOT compatible with --inplace")
		}
		csv.FlagOutput = argInput
		csv.FlagForce = true // implied overwrite
	}

	err = helper.InputTextFileAndOutput(argInput, csv.FlagOutput, csv.FlagForce, func(r io.Reader, w io.Writer,
		inputName, outputNme string) error {
		return sortCsvFile(r, flagKey, w, csv.FlagNoHeader)
	})

	if err != nil {
		return err
	}
	return nil
}

func init() {
	sortCmd.Flags().BoolVarP(&flagInplace, "inplace", "", false, `Update input file in place`)
	sortCmd.Flags().StringVarP(&flagKey, "key", "", "", `(Required) Key field`)
	sortCmd.MarkFlagRequired("key")
	csv.CsvCmd.AddCommand(sortCmd)
}
