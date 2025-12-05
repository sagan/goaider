package txt2csv

import (
	"fmt"
	"io"
	"os"

	_ "github.com/mithrandie/csvq-driver"
	"github.com/natefinch/atomic"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	csvCmd "github.com/sagan/goaider/cmd/csv"
	"github.com/sagan/goaider/features/csvfeature"
	"github.com/sagan/goaider/util"
	"github.com/sagan/goaider/util/stringutil"
)

var txt2csvCmd = &cobra.Command{
	Use:   "txt2csv {text_file} [additional_text_file]...",
	Short: "Create csv from one or more text files",
	Long: `Create csv from one or more text files.

{text_file} can be "-" to read from stdin.
Each provided text file is treated as a column of output csv, each line is a csv column value.
The column names of output csv defaults to "c1", "c2"..., set them through --columns flag.
If --no-header flag is set, the output csv doesn't have header row.`,
	Args: cobra.MinimumNArgs(1),
	RunE: doText2csv,
}

var (
	flagColumns []string // column names
)

func doText2csv(cmd *cobra.Command, args []string) (err error) {
	if csvCmd.FlagOutput != "" && csvCmd.FlagOutput != "-" {
		if exists, err := util.FileExists(csvCmd.FlagOutput); err != nil || (exists && !csvCmd.FlagForce) {
			return fmt.Errorf("output file %q exists or can't access, err=%w", csvCmd.FlagOutput, err)
		}
	}
	var allLines [][]string
	for _, filePath := range args {
		var content []byte
		if filePath == "-" {
			content, err = io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("failed to read from stdin: %w", err)
			}
		} else {
			content, err = os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("failed to read file %q: %w", filePath, err)
			}
		}
		lines := stringutil.SplitLines(string(content))
		allLines = append(allLines, lines)
		if len(allLines) > 1 && len(lines) != len(allLines[0]) {
			log.Warnf("Warning: file %q has %d lines, which is different from previous files (%d lines)\n",
				filePath, len(lines), len(allLines[0]))
		}
	}

	var columnNames []string
	if !csvCmd.FlagNoHeader {
		if len(flagColumns) > 0 {
			if len(flagColumns) != len(allLines) {
				return fmt.Errorf("number of column names (%d) does not match number of input files (%d)", len(flagColumns), len(allLines))
			}
			columnNames = flagColumns
		} else {
			columnNames = make([]string, len(allLines))
			for i := range allLines {
				columnNames[i] = fmt.Sprintf("c%d", i+1)
			}
		}
	}

	reader, writer := io.Pipe()
	go func() {
		err = csvfeature.WriteListsToCsv(writer, columnNames, allLines...)
		writer.CloseWithError(err)
	}()
	if csvCmd.FlagOutput == "-" {
		_, err = io.Copy(os.Stdout, reader)
	} else {
		err = atomic.WriteFile(csvCmd.FlagOutput, reader)
	}
	if err != nil {
		return fmt.Errorf("failed to write CSV: %w", err)
	}

	return nil
}

func init() {
	csvCmd.CsvCmd.AddCommand(txt2csvCmd)
	txt2csvCmd.Flags().StringSliceVarP(&flagColumns, "columns", "c", nil,
		`Comma-seperated column names for the output CSV. If not provided, defaults to "c1,c2..."`)
}
