package uniq

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/spf13/cobra"

	"github.com/sagan/goaider/cmd/csv"
	"github.com/sagan/goaider/util/helper"
)

var uniqCmd = &cobra.Command{
	Use:   "uniq --key <key_field> <input.csv | ->",
	Short: "uniquify csv file to remove duplicate rows.",
	Long: `uniquify csv file to remove duplicate rows.

Output to stdout by default. If --inplace is set, update input file in place.

Two rows are considerred duplicate if they have the same key field value.
All duplicate rows except the first one are removed from the output.

Use "-" as input arg to read from stdin.`,
	Args: cobra.ExactArgs(1),
	RunE: uniq,
}

var (
	flagCheck   bool   // check mode, do not output anything, return 0 if csv has no duplicate rows
	flagInplace bool   // update input file in place
	flagKey     string // key field
)

func uniq(cmd *cobra.Command, args []string) (err error) {
	argInput := args[0]

	if flagCheck {
		var input *os.File
		if argInput == "-" {
			input = os.Stdin
		} else {
			input, err = os.Open(argInput)
			if err != nil {
				return err
			}
			defer input.Close()
		}
		duplicates, err := uniqCsvFile(input, flagKey, io.Discard, nil, csv.FlagNoHeader)
		if err != nil {
			return err
		}
		log.Printf("%q: %d duplicates", argInput, duplicates)
		if duplicates > 0 {
			return fmt.Errorf("%d duplicates found", duplicates)
		}
		return nil
	}

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

	err = helper.InputFileAndOutput(argInput, csv.FlagOutput, csv.FlagForce, func(r io.Reader, w io.Writer,
		inputName, outputNme string) error {
		duplicates, err := uniqCsvFile(r, flagKey, w, nil, csv.FlagNoHeader)
		if err == nil {
			if inputName != outputNme {
				log.Printf("%q => %q : %d duplicates removed", inputName, outputNme, duplicates)
			} else {
				log.Printf("%q : %d duplicates removed", inputName, duplicates)
			}
		}
		return err
	})

	if err != nil {
		return err
	}

	return nil
}

func init() {
	uniqCmd.Flags().BoolVarP(&flagCheck, "check", "c", false,
		"Check mode: do not output anything, exit 0 if csv has no duplicate rows, 1 if has duplicate rows")
	uniqCmd.Flags().StringVarP(&flagKey, "key", "k", "", `(Required) Key field`)
	uniqCmd.Flags().BoolVarP(&flagInplace, "inplace", "", false, `Update input file in place.`)
	uniqCmd.MarkFlagRequired("key")
	csv.CsvCmd.AddCommand(uniqCmd)
}
