package excel2csv

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"

	"github.com/natefinch/atomic"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/xuri/excelize/v2"

	csvCmd "github.com/sagan/goaider/cmd/csv"
	"github.com/sagan/goaider/util"
)

const (
	// progressReportInterval defines how often (in number of rows) to report progress.
	progressReportInterval = 1000
)

var excel2csvCmd = &cobra.Command{
	Use:     "excel2csv {foo.xlsx | -}",
	Aliases: []string{"xlsx2csv"},
	Short:   "Convert Excel (.xlsx) file to csv",
	Long: `Convert Excel (.xlsx) file to csv.

The {foo.xlsx} argument can be "-" to read from stdin.`,
	Args: cobra.ExactArgs(1),
	RunE: doExcel2Csv,
}

var (
	flagSheedIndex int
)

func doExcel2Csv(cmd *cobra.Command, args []string) (err error) {
	argInput := args[0]
	if csvCmd.FlagOutput != "" && csvCmd.FlagOutput != "-" {
		if exists, err := util.FileExists(csvCmd.FlagOutput); err != nil || (exists && !csvCmd.FlagForce) {
			return fmt.Errorf("output file %q exists or can't access, err=%w", csvCmd.FlagOutput, err)
		}
	}
	var input io.Reader
	if argInput == "-" {
		input = cmd.InOrStdin()
	} else {
		f, err := os.Open(argInput)
		if err != nil {
			return fmt.Errorf("failed to open input file %q: %w", argInput, err)
		}
		defer f.Close()
		input = f
	}

	xlsxFile, err := excelize.OpenReader(input)
	if err != nil {
		return fmt.Errorf("failed to open Excel input: %v", err)
	}
	defer func() {
		if err := xlsxFile.Close(); err != nil {
			log.Printf("Error closing Excel file: %v\n", err)
		}
	}()
	sheetList := xlsxFile.GetSheetList()
	if flagSheedIndex < 0 || flagSheedIndex >= len(sheetList) {
		return fmt.Errorf("sheet-index %d is out of bounds. The workbook has %d sheets (indices 0 to %d)",
			flagSheedIndex, len(sheetList), len(sheetList)-1)
	}
	sheetName := sheetList[flagSheedIndex]
	rows, err := xlsxFile.Rows(sheetName)
	if err != nil {
		return fmt.Errorf("failed to get rows iterator for sheet '%s': %v", sheetName, err)
	}
	reader, writer := io.Pipe()
	go func() {
		csvWriter := csv.NewWriter(writer)
		log.Printf("Converting sheet %q (index %d) %s => %s...", sheetName, flagSheedIndex, argInput, csvCmd.FlagOutput)
		var header []string
		var columnCount int
		rowCount := 0

		// --- Read Header and Determine Column Count ---
		if rows.Next() {
			header, err = rows.Columns()
			if err != nil {
				writer.CloseWithError(fmt.Errorf("failed to read header row: %v", err))
				return
			}
			columnCount = len(header)
			if err := csvWriter.Write(header); err != nil {
				writer.CloseWithError(fmt.Errorf("error writing header to CSV: %v", err))
				return
			}
			rowCount++
		} else {
			log.Warnf("Warning: Sheet %q is empty.", sheetName)
			writer.Close()
			return
		}

		// --- Process Remaining Rows ---
		for rows.Next() {
			row, err := rows.Columns()
			if err != nil {
				log.Printf("Error reading row %d: %v", rowCount+1, err)
				continue
			}
			// Normalize row length to match the header.
			if len(row) < columnCount {
				paddedRow := make([]string, columnCount)
				copy(paddedRow, row)
				row = paddedRow
			} else if len(row) > columnCount {
				log.Printf("Warning: Row %d has %d columns, more than header's %d. Truncating.",
					rowCount+1, len(row), columnCount)
				row = row[:columnCount]
			}
			if err := csvWriter.Write(row); err != nil {
				log.Printf("Error writing row %d to CSV: %v", rowCount+1, err)
			}
			rowCount++
			// Report progress to stderr.
			if rowCount%progressReportInterval == 0 {
				log.Printf("... processed %d rows", rowCount)
				csvWriter.Flush()
			}
		}
		if err := rows.Close(); err != nil {
			log.Errorf("Error closing row iterator: %v", err)
		}
		// Finalization ---
		if err := csvWriter.Error(); err != nil {
			writer.CloseWithError(fmt.Errorf("an error occurred during CSV writing: %v", err))
			return
		}
		csvWriter.Flush()
		log.Printf("Conversion complete! Successfully wrote %d rows.", rowCount)
		writer.Close()
	}()

	if csvCmd.FlagOutput == "-" {
		_, err = io.Copy(cmd.OutOrStdout(), reader)
	} else {
		err = atomic.WriteFile(csvCmd.FlagOutput, reader)
	}
	if err != nil {
		return err
	}

	return nil
}

func init() {
	excel2csvCmd.Flags().IntVarP(&flagSheedIndex, "sheet-index", "s", 0, "0-based index of the Excel sheet to convert")
	csvCmd.CsvCmd.AddCommand(excel2csvCmd)
}
