package sort

import (
	"encoding/csv"
	"fmt"
	"io"
	"sort"
)

// sortCsvFile reads a csv file and sorts it based on keyField.
// It writes the sorted rows to output.
// If noHeader is true, input files are treated as having no header row; columns are named c1, c2, c3...
func sortCsvFile(input io.Reader, keyField string, output io.Writer, noHeader bool) error {
	// 1. Read all records
	r := csv.NewReader(input)
	records, err := r.ReadAll()
	if err != nil {
		return fmt.Errorf("failed to read csv content: %w", err)
	}

	if len(records) == 0 {
		return nil
	}

	// 2. Separate header and data
	var header []string
	var data [][]string

	if noHeader {
		// Treat all records as data
		data = records
		// Generate implicit headers based on the first row's length
		if len(data) > 0 {
			cols := len(data[0])
			header = make([]string, cols)
			for i := 0; i < cols; i++ {
				header[i] = fmt.Sprintf("c%d", i+1)
			}
		}
	} else {
		// Standard behavior: first row is header
		header = records[0]
		data = records[1:]
	}

	// 3. Find the index of the keyField
	keyIndex := -1
	for i, h := range header {
		if h == keyField {
			keyIndex = i
			break
		}
	}

	if keyIndex == -1 {
		return fmt.Errorf("keyField '%s' not found in header (available: %v)", keyField, header)
	}

	// 4. Sort the data
	sort.Slice(data, func(i, j int) bool {
		// Safety check for row length
		if keyIndex >= len(data[i]) || keyIndex >= len(data[j]) {
			return false
		}
		return data[i][keyIndex] < data[j][keyIndex]
	})

	// 5. Write to output
	w := csv.NewWriter(output)

	if !noHeader {
		if err := w.Write(header); err != nil {
			return fmt.Errorf("failed to write header: %w", err)
		}
	}
	if err := w.WriteAll(data); err != nil {
		return fmt.Errorf("failed to write data: %w", err)
	}
	w.Flush()

	return w.Error()
}
