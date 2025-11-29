package join

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
)

// A container for pre-processed CSV data, used internally.
type csvContent struct {
	header  []string
	data    [][]string
	prefix  string
	joinKey string
	joinIdx int
}

// rightRow is used in the join map to track usage for full joins.
type rightRow struct {
	data []string
	used bool
}

// readCsv reads a CSV file, finds the join column index, and applies a prefix to headers.
// If noHeader is true, headers are generated as "c1", "c2"... and all rows are treated as data.
func readCsv(filePath, joinKey, prefix string, noHeader bool) (*csvContent, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("could not open file %s: %w", filePath, err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	allRecords, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("could not read CSV data from %s: %w", filePath, err)
	}

	if len(allRecords) == 0 {
		return nil, fmt.Errorf("file %s is empty", filePath)
	}

	var header []string
	var data [][]string

	if noHeader {
		// Treat all records as data
		data = allRecords
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
		header = allRecords[0]
		data = allRecords[1:]
	}

	content := &csvContent{
		header:  header,
		data:    data,
		prefix:  prefix,
		joinKey: joinKey,
	}

	// Find the index of the join key
	// Note: If noHeader is true, joinKey must be passed as "c1", "c2", etc.
	content.joinIdx = -1 // Initialize to invalid index
	for i, h := range content.header {
		if h == joinKey {
			content.joinIdx = i
			break
		}
	}

	if content.joinIdx == -1 {
		return nil, fmt.Errorf("join key column '%s' not found in header of %s (headers: %v)", joinKey, filePath, content.header)
	}

	// Apply prefix to all headers
	if prefix != "" {
		for i := range content.header {
			content.header[i] = prefix + content.header[i]
		}
	}

	return content, nil
}

// Read two csv files, "join" them and output a new csv.
// It's similar to SQL left join semantic, but for each left csv row,
// at most one (1) matched right csv row is allowed; if multiple right row match, use the first one.
// The result csv contains all rows & columns of left join, along with columns of right csv
// when leftCsv.leftOn field == rightCsv.rightOn field (must not be empty string).
// If a same column name exists in both left and right csv, use the left version as output csv column value.
// If leftPrefix / rightPrefix is not empty, Prefix columns of left / right csv with this string in output csv.
// If allJoin is true, do a "full join" instead of "left join": include all rows of both csv in output,
// if all right csv column names are "masked" by left csv, return an error instead.
// If noHeader is true, input files are treated as having no header row; columns are named c1, c2, c3...
func joinCsvFiles(leftCsvFile, rightCsvFile string, output io.Writer,
	leftOn, rightOn, leftPrefix, rightPrefix string, allJoin, noHeader bool) (err error) {

	if leftOn == "" || rightOn == "" {
		return fmt.Errorf("join key parameters (leftOn and rightOn) must not be empty")
	}

	// 1. Read and preprocess both CSV files
	left, err := readCsv(leftCsvFile, leftOn, leftPrefix, noHeader)
	if err != nil {
		return fmt.Errorf("failed to process left CSV: %w", err)
	}

	right, err := readCsv(rightCsvFile, rightOn, rightPrefix, noHeader)
	if err != nil {
		return fmt.Errorf("failed to process right CSV: %w", err)
	}

	// 2. Build the right-side map for efficient lookups (key: join value)
	rightMap := make(map[string][]*rightRow)
	for _, row := range right.data {
		if len(row) <= right.joinIdx {
			// Skip rows that are shorter than expected based on the header
			continue
		}
		joinKey := row[right.joinIdx]
		if joinKey == "" {
			// Skip rows with empty join keys
			continue
		}

		rRow := &rightRow{data: row, used: false}
		rightMap[joinKey] = append(rightMap[joinKey], rRow)
	}

	// 3. Determine the final output header
	outputHeader := make([]string, 0, len(left.header)+len(right.header))
	outputHeader = append(outputHeader, left.header...)

	// Tracks columns already added from the left side (prefixed names)
	leftHeaderSet := make(map[string]struct{})
	for _, h := range left.header {
		leftHeaderSet[h] = struct{}{}
	}

	// Add right-side columns, avoiding duplication (left columns take precedence)
	rightOnlyHeaders := make([]string, 0, len(right.header))
	rightOnlyHeaderMap := make(map[string]int) // Store index in original right header

	for i, rH := range right.header {
		if _, exists := leftHeaderSet[rH]; !exists {
			outputHeader = append(outputHeader, rH)
			rightOnlyHeaders = append(rightOnlyHeaders, rH)
			rightOnlyHeaderMap[rH] = i
		}
	}

	// 4. Handle "Masking" check if allJoin is true (FULL JOIN)
	// Masking: Check if all right-side columns (excluding the join key) are also present in the left header.
	if allJoin {
		maskedCount := 0
		for _, rH := range right.header {
			if _, exists := leftHeaderSet[rH]; exists {
				// The right column is "masked" by a left column (same name after prefixing)
				maskedCount++
			}
		}
		// If every column in the right header is also in the left header, return error.
		if maskedCount == len(right.header) {
			return fmt.Errorf("full join requested (allJoin=true), but all right CSV columns are masked/duplicated by left CSV columns, which indicates a configuration error")
		}
	}

	// 5. Perform the join
	joinedRows := make([][]string, 0)
	rightPadding := make([]string, len(rightOnlyHeaders)) // Empty strings for unmatched fields

	// LEFT JOIN part: Iterate over the left side
	for _, lRow := range left.data {
		if len(lRow) <= left.joinIdx {
			continue // Skip incomplete left rows
		}
		joinKey := lRow[left.joinIdx]

		if matches, ok := rightMap[joinKey]; ok && joinKey != "" {
			// Matched rows: create a joined row for each match
			for i, rRow := range matches {
				if i > 0 {
					break // only use the first right match row
				}
				rRow.used = true // Mark as used for full join logic

				joinedRow := append([]string{}, lRow...) // Start with left data

				// Append only the non-masked columns from the right
				for _, rH := range rightOnlyHeaders {
					originalIdx := rightOnlyHeaderMap[rH]
					if originalIdx < len(rRow.data) {
						joinedRow = append(joinedRow, rRow.data[originalIdx])
					} else {
						joinedRow = append(joinedRow, "") // Should not happen if data is rectangular
					}
				}
				joinedRows = append(joinedRows, joinedRow)
			}
		} else {
			// Unmatched left row: Left data + right padding
			joinedRow := append([]string{}, lRow...)
			joinedRow = append(joinedRow, rightPadding...)
			joinedRows = append(joinedRows, joinedRow)
		}
	}

	// FULL JOIN part: Add unmatched right rows (if allJoin is true)
	if allJoin {
		leftPadding := make([]string, len(left.header)) // Empty strings for unmatched fields

		for _, matches := range rightMap {
			for _, rRow := range matches {
				if !rRow.used {
					// Unmatched right row: Left padding + Right data (only non-masked columns)
					joinedRow := append([]string{}, leftPadding...)

					// Append only the non-masked columns from the right
					for _, rH := range rightOnlyHeaders {
						originalIdx := rightOnlyHeaderMap[rH]
						if originalIdx < len(rRow.data) {
							joinedRow = append(joinedRow, rRow.data[originalIdx])
						} else {
							joinedRow = append(joinedRow, "")
						}
					}
					joinedRows = append(joinedRows, joinedRow)
				}
			}
		}
	}

	// 6. Write the final CSV output
	writer := csv.NewWriter(output)

	// Write header
	// If noHeader is true, we usually still write the header to the output because
	// the columns from two files might be mixed/joined, making the structure ambiguous without headers.
	// However, if strict "no header output" is desired, this line could be conditional.
	// Based on standard "join" tool behavior, the result usually gains a header.
	if err := writer.Write(outputHeader); err != nil {
		return fmt.Errorf("failed to write output header: %w", err)
	}

	// Write all joined rows
	if err := writer.WriteAll(joinedRows); err != nil {
		return fmt.Errorf("failed to write joined rows: %w", err)
	}

	writer.Flush()
	return writer.Error()
}
