package uniq

import (
	"encoding/csv"
	"fmt"
	"io"
)

// uniqCsvFile reads a csv file and uniquifies it based on keyField.
// It writes unique rows to output and duplicates to duplicateRowsOutput (if not nil).
// If noHeader is true, the input csv is treated as having no header row; columns are named c1, c2...
func uniqCsvFile(csvInput io.Reader, keyField string, output, duplicateRowsOutput io.Writer, noHeader bool) (duplicatesCnt int, err error) {
	// 1. Setup Reader
	r := csv.NewReader(csvInput)

	// 2. Read first record to determine structure or get header
	firstRecord, err := r.Read()
	if err != nil {
		if err == io.EOF {
			return 0, nil // Empty file
		}
		return 0, fmt.Errorf("failed to read first record: %w", err)
	}

	var header []string
	var processFirstRecord bool

	if noHeader {
		// Treat first record as data
		processFirstRecord = true
		// Generate implicit headers based on the first row's length
		cols := len(firstRecord)
		header = make([]string, cols)
		for i := 0; i < cols; i++ {
			header[i] = fmt.Sprintf("c%d", i+1)
		}
	} else {
		// Treat first record as header
		header = firstRecord
		processFirstRecord = false
	}

	// 3. Find the index of the keyField
	keyIndex := -1
	for i, h := range header {
		// specific logic can be added here to handle case-insensitivity or trimming if needed
		if h == keyField {
			keyIndex = i
			break
		}
	}

	if keyIndex == -1 {
		return 0, fmt.Errorf("keyField '%s' not found in header: %v", keyField, header)
	}

	// 4. Setup Writers
	wOut := csv.NewWriter(output)
	// Write header to main output
	if err := wOut.Write(header); err != nil {
		return 0, fmt.Errorf("failed to write header to output: %w", err)
	}

	var wDup *csv.Writer
	if duplicateRowsOutput != nil {
		wDup = csv.NewWriter(duplicateRowsOutput)
		// Write header to duplicates output
		if err := wDup.Write(header); err != nil {
			return 0, fmt.Errorf("failed to write header to duplicate output: %w", err)
		}
	}

	// 5. Process records
	seen := make(map[string]struct{}) // Empty struct uses 0 bytes

	// Helper closure to process a single record
	processRow := func(record []string) error {
		// Safety check for malformed rows (optional, but good practice)
		if keyIndex >= len(record) {
			return nil // skip malformed row
		}

		keyVal := record[keyIndex]

		if _, exists := seen[keyVal]; exists {
			// It is a duplicate
			duplicatesCnt++
			if wDup != nil {
				if err := wDup.Write(record); err != nil {
					return fmt.Errorf("error writing to duplicate output: %w", err)
				}
			}
		} else {
			// It is unique
			seen[keyVal] = struct{}{}
			if err := wOut.Write(record); err != nil {
				return fmt.Errorf("error writing to output: %w", err)
			}
		}
		return nil
	}

	// If noHeader was true, the first read was actually data, so process it now.
	if processFirstRecord {
		if err := processRow(firstRecord); err != nil {
			return 0, err
		}
	}

	// Loop through the rest of the file
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, fmt.Errorf("error reading csv record: %w", err)
		}

		if err := processRow(record); err != nil {
			return 0, err
		}
	}

	// 6. Flush writers and check for write errors
	wOut.Flush()
	if err := wOut.Error(); err != nil {
		return 0, fmt.Errorf("error flushing output writer: %w", err)
	}

	if wDup != nil {
		wDup.Flush()
		if err := wDup.Error(); err != nil {
			return 0, fmt.Errorf("error flushing duplicate writer: %w", err)
		}
	}

	return duplicatesCnt, nil
}
