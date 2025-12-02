package query

import (
	"bytes"
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/template"

	"github.com/jmoiron/sqlx"
	_ "github.com/mithrandie/csvq-driver"
	"github.com/natefinch/atomic"
	"github.com/spf13/cobra"

	csvCmd "github.com/sagan/goaider/cmd/csv"
	"github.com/sagan/goaider/constants"
	"github.com/sagan/goaider/util"
	"github.com/sagan/goaider/util/helper"
	"github.com/sagan/goaider/util/stringutil"
)

var joinCmd = &cobra.Command{
	Use:   "query <sql>",
	Short: "Run SQL query on csv files",
	Long: `Run SQL query on csv files.

It uses csvq ( https://github.com/mithrandie/csvq ).

Output query result as csv.

Examples:
- goaider csv query "select * from foo" # query "foo.csv"
` + "- goaider csv query 'select * from `foo-bar.csv` limit 10'" + `

The output csv has header line unless --no-header is set.

Note:
- If an identifier name, such as filename (table name) or column name includes any special char (like "." or "-"),
  it must be wrapped in backticks.
- In Linux bash, if query contains backticks, to prevent interpretion of backticks as cmd,
  the argument must be wrapped in single quotes.
- Filename extension can be omitted in sql table if only "<filename>.csv" but not "<filename>" file exists.
- Literal strings in sql should be wrapped in double quotes.`,
	Args: cobra.ExactArgs(1),
	RunE: query,
}

var (
	flagDir      string // csv files dir. defaults to ".".
	flagTemplate string // Go text template string.
	flagText     bool   // output plain text.
	flagOneLine  bool   // force each csv row outputs only one line
)

func query(cmd *cobra.Command, args []string) (err error) {
	if csvCmd.FlagOutput != "" && csvCmd.FlagOutput != "-" {
		if exists, err := util.FileExists(csvCmd.FlagOutput); err != nil || (exists && !csvCmd.FlagForce) {
			return fmt.Errorf("output file %q exists or can't access, err=%w", csvCmd.FlagOutput, err)
		}
	}
	if flagTemplate != "" {
		flagText = true
	}
	query := args[0]
	db, err := sqlx.Open("csvq", ".")
	if err != nil {
		return err
	}
	defer db.Close()

	if csvCmd.FlagNoHeader {
		_, err = db.Exec("SET @@NO_HEADER TO TRUE")
		if err != nil {
			return fmt.Errorf("failed to set no-header flag: %w", err)
		}
	}

	rows, err := db.Query(query)
	if err != nil {
		return err
	}

	reader, writer := io.Pipe()
	go func() {
		if flagText {
			err = writeSqlRowsToText(rows, writer, flagTemplate, flagOneLine)
		} else {
			err = writeSqlRowsToCsv(rows, writer, csvCmd.FlagNoHeader, flagOneLine)
		}
		writer.CloseWithError(err)
	}()
	if csvCmd.FlagOutput == "-" {
		_, err = io.Copy(os.Stdout, reader)
	} else {
		err = atomic.WriteFile(csvCmd.FlagOutput, reader)
	}
	if err != nil {
		return err
	}

	return nil
}

func init() {
	csvCmd.CsvCmd.AddCommand(joinCmd)
	joinCmd.Flags().StringVarP(&flagDir, "dir", "d", ".", "CSV files directory")
	joinCmd.Flags().StringVarP(&flagTemplate, "template", "t", "",
		`Go text template string to format output. If set, --text is implied. Example: '{{.col1}},{{.col2}}'. `+
			constants.HELP_TEMPLATE_FLAG)
	joinCmd.Flags().BoolVarP(&flagText, "text", "", false,
		`Output as plain text instead of CSV. No header line is written. If --template is set, this is implied. `+
			`The --template is required unless query result has only one (1) column, in which case it's written directly`)
	joinCmd.Flags().BoolVarP(&flagOneLine, "one-line", "", false,
		`Force each csv row outputs only one line. This is useful when a field contains newlines. `+
			`It replaces one or more consecutive newline characters (\r, \n) with single space`)
}

// writeSqlRowsToCsv writes the result of a sql query to a CSV writer.
// It sorts the columns alphabetically by name in the output.
func writeSqlRowsToCsv(rows *sql.Rows, csvOutput io.Writer, noHeader bool, oneLine bool) error {
	// 1. Get the original column names from the query result
	cols, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("failed to get columns: %w", err)
	}

	// 2. Create a mapping structure to track the original index of each column
	type colMap struct {
		Name      string
		OrigIndex int
	}

	mapping := make([]colMap, len(cols))
	for i, name := range cols {
		mapping[i] = colMap{
			Name:      name,
			OrigIndex: i,
		}
	}

	// 3. Sort the mapping slice alphabetically by column Name
	sort.Slice(mapping, func(i, j int) bool {
		return mapping[i].Name < mapping[j].Name
	})

	// 4. Set up the CSV writer
	writer := csv.NewWriter(csvOutput)
	defer writer.Flush()

	// 5. Write the Header (using the sorted order)
	if !noHeader {
		header := make([]string, len(cols))
		for i, m := range mapping {
			header[i] = m.Name
		}
		if err := writer.Write(header); err != nil {
			return fmt.Errorf("failed to write header: %w", err)
		}
	}

	// 6. Prepare buffers for scanning rows
	// values: holds the data scanned from the database (in original order)
	// scanArgs: pointers to 'values' required by rows.Scan
	values := make([]any, len(cols))
	scanArgs := make([]any, len(cols))
	for i := range values {
		scanArgs[i] = &values[i]
	}

	// Buffer to hold the reordered string data for the CSV writer
	rowString := make([]string, len(cols))

	// 7. Iterate through the rows
	for rows.Next() {
		if err := rows.Scan(scanArgs...); err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}

		// Map the raw data from original index -> sorted index
		for sortedIdx, m := range mapping {
			val := values[m.OrigIndex]

			// Convert interface{} to string
			var strVal string
			if val != nil {
				switch v := val.(type) {
				case []byte:
					// Handle byte slices (common for strings/blobs in some drivers)
					strVal = string(v)
				default:
					// generic string representation for ints, floats, dates, etc.
					strVal = fmt.Sprintf("%v", v)
				}
			}
			// If val is nil, strVal remains "" (empty string)

			if oneLine {
				strVal = stringutil.ReplaceNewLinesWithSpace(strVal)
			}
			rowString[sortedIdx] = strVal
		}

		// Write the sorted row to CSV
		if err := writer.Write(rowString); err != nil {
			return fmt.Errorf("failed to write row: %w", err)
		}
	}

	// Check for errors that occurred during iteration
	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating rows: %w", err)
	}

	return nil
}

// writeSqlRowsToText writes rows to output as plain text.
// Each row is a line formatted by the provided template string.
// If the rendered line is empty (after trimming), the row is skipped.
// Special case: if rows has 1 column and template is empty, the column value is written directly.
func writeSqlRowsToText(rows *sql.Rows, output io.Writer, templateStr string, oneLine bool) error {
	cols, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("failed to get columns: %w", err)
	}

	// Special case: 1 column and empty template -> just output the value directly
	isSingleColRaw := len(cols) == 1 && templateStr == ""

	var t *template.Template
	if !isSingleColRaw {
		if templateStr != "" {
			t, err = helper.GetTemplate(templateStr, true)
			if err != nil {
				return fmt.Errorf("failed to parse template: %w", err)
			}
		} else {
			return fmt.Errorf("template must be set for non single column rows")
		}
	}

	values := make([]any, len(cols))
	scanArgs := make([]any, len(cols))
	for i := range values {
		scanArgs[i] = &values[i]
	}

	for rows.Next() {
		if err := rows.Scan(scanArgs...); err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}

		var line string

		if isSingleColRaw {
			val := values[0]
			if val != nil {
				switch v := val.(type) {
				case []byte:
					line = string(v)
				default:
					line = fmt.Sprintf("%v", v)
				}
			}
		} else {
			if t == nil {
				// Multiple columns but no template, or empty template provided
				// The result is effectively empty, so we skip as per rules.
				continue
			}

			// Create map for template context
			rowMap := make(map[string]any, len(cols))
			for i, colName := range cols {
				val := values[i]
				// Convert []byte to string for easier template usage
				if b, ok := val.([]byte); ok {
					rowMap[colName] = string(b)
				} else {
					rowMap[colName] = val
				}
			}

			var buf bytes.Buffer
			if err := t.Execute(&buf, rowMap); err != nil {
				return fmt.Errorf("failed to execute template: %w", err)
			}
			line = buf.String()
		}

		if oneLine {
			line = stringutil.ReplaceNewLinesWithSpace(line)
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if _, err := fmt.Fprintln(output, line); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating rows: %w", err)
	}

	return nil
}
