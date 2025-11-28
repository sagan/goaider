package sort

import (
	"encoding/csv"
	"fmt"
	"io"
	"sort"
)

// sortCsvFile reads a csv file and sorts it based on keyField.
// It writes the sorted rows to output.
func sortCsvFile(input io.Reader, keyField string, output io.Writer) error {
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
	header := records[0]
	data := records[1:]

	// 3. Find the index of the keyField
	keyIndex := -1
	for i, h := range header {
		if h == keyField {
			keyIndex = i
			break
		}
	}

	if keyIndex == -1 {
		return fmt.Errorf("keyField '%s' not found in header", keyField)
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
	if err := w.Write(header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}
	if err := w.WriteAll(data); err != nil {
		return fmt.Errorf("failed to write data: %w", err)
	}
	w.Flush()

	return w.Error()
}
