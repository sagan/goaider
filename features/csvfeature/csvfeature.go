package main

import (
	"encoding/csv"
	"fmt"
	"io"
)

// WriteListsToCsv combines multiple lists of columns and outputs them as CSV.
// It zips the lists horizontally.
// lists[0] is the data for the first set of columns, lists[1] for the next, etc.
func WriteListsToCsv(output io.Writer, columnNames []string, lists ...[][]string) error {
	writer := csv.NewWriter(output)

	// 1. Write the Header (if provided)
	if columnNames != nil {
		if err := writer.Write(columnNames); err != nil {
			return fmt.Errorf("failed to write header: %w", err)
		}
	}

	// 2. Determine the maximum number of rows across all lists
	// to ensures we process all data even if lists have uneven lengths.
	maxRows := 0
	for _, list := range lists {
		if len(list) > maxRows {
			maxRows = len(list)
		}
	}

	// 3. Iterate row by row (zipping the lists)
	for i := 0; i < maxRows; i++ {
		var fullRow []string

		// Collect columns from each list for the current row index
		for _, list := range lists {
			if i < len(list) {
				// Append the columns from this list to the current row
				fullRow = append(fullRow, list[i]...)
			} else {
				// Handle case where this specific list is shorter than maxRows.
				// Note: In a strict CSV structure, you might want to pad with empty strings here.
				// Since we don't know the width of the missing rows, we skip.
			}
		}

		if err := writer.Write(fullRow); err != nil {
			return fmt.Errorf("failed to write row %d: %w", i, err)
		}
	}

	// 4. Flush the buffer
	writer.Flush()
	if err := writer.Error(); err != nil {
		return fmt.Errorf("error flushing csv writer: %w", err)
	}

	return nil
}
