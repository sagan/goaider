package sort

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/sagan/goaider/cmd/csv"
	"github.com/sagan/goaider/util/helper"
)

var sortCmd = &cobra.Command{
	Use:   "sort --key <key_field> <input.csv | ->",
	Short: "sort csv file based on key field.",
	Long: `sort csv file based on key field.

Output to stdout by default. If --inplace is set, update input file in place.
Use "-" as input arg to read from stdin.`,
	Args: cobra.ExactArgs(1),
	RunE: sortFunc,
}

var (
	flagForce   bool   // force overwrite existing file
	flagKey     string // key field
	flagInplace bool   // update input file in place
	flagOutput  string // output file, set to "-" to output to stdout
)

func sortFunc(cmd *cobra.Command, args []string) (err error) {
	argInput := args[0]

	if flagInplace {
		if flagOutput != "-" {
			return fmt.Errorf("--inplace and --output flags are NOT compatible")
		}
		if argInput == "-" {
			return fmt.Errorf("stdin input is NOT compatible with --inplace")
		}
		flagOutput = argInput
		flagForce = true // implied overwrite
	}

	err = helper.InputFileAndOutput(argInput, flagOutput, flagForce, func(r io.Reader, w io.Writer,
		inputName, outputNme string) error {
		return sortCsvFile(r, flagKey, w)
	})

	if err != nil {
		return err
	}
	return nil
}

func init() {
	sortCmd.Flags().BoolVarP(&flagForce, "force", "", false, "Force overwriting without confirmation.")
	sortCmd.Flags().StringVarP(&flagOutput, "output", "o", "-", `Output file path. Use "-" for stdout.`)
	sortCmd.Flags().BoolVarP(&flagInplace, "inplace", "", false, `Update input file in place.`)
	sortCmd.Flags().StringVarP(&flagKey, "key", "", "", `(Required) Key field.`)
	sortCmd.MarkFlagRequired("key")
	csv.CsvCmd.AddCommand(sortCmd)
}
