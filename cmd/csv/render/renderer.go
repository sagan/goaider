package render

import (
	"encoding/csv"
	"fmt"
	"io" // Renamed to avoid conflict with current package name 'exec'

	"github.com/sagan/goaider/util/helper"
	"github.com/sagan/goaider/util/stringutil"
)

func renderCsv(input io.Reader, templateStr string, noHeader bool, output io.Writer, oneLine bool) (err error) {
	// 1. Parse the template initially to ensure it is valid.
	tpl, err := helper.GetTemplate(templateStr, true)
	if err != nil {
		return fmt.Errorf("invalid template: %w", err)
	}

	reader := csv.NewReader(input)
	var headers []string

	// 2. Handle Header Logic
	if !noHeader {
		// Read the first row as headers
		headers, err = reader.Read()
		if err != nil {
			if err == io.EOF {
				return nil // Empty file, no work to do
			}
			return fmt.Errorf("failed to read header: %w", err)
		}
	}

	// 3. Process Rows
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("csv read error: %w", err)
		}

		// Prepare data context for the template
		data := make(map[string]string)

		if noHeader {
			// Implicit headers: c1, c2, c3...
			for i, val := range record {
				key := fmt.Sprintf("c%d", i+1)
				data[key] = val
			}
		} else {
			// Map specific headers to values
			// Note: csv.Reader ensures record length matches header length unless configured otherwise.
			// We add safety checks just in case fields per record is variable.
			for i, header := range headers {
				if i < len(record) {
					data[header] = record[i]
				}
			}
		}

		renderResult, err := tpl.Exec(data)
		if err != nil {
			return fmt.Errorf("template execute error: %w", err)
		}
		if renderResult == "" {
			continue
		}
		if oneLine {
			renderResult = stringutil.ReplaceNewLinesWithSpace(renderResult)
		}
		if _, err := fmt.Fprintln(output, renderResult); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
	}

	return nil
}
