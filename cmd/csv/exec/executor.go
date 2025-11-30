package exec

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	osexec "os/exec" // Renamed to avoid conflict with current package name 'exec'
	"strings"

	"github.com/google/shlex"
	"github.com/sagan/goaider/util/helper"
)

// Read csv file, execute a cmd for each line of the csv.
// the cmdline to exeute is generated from template argument,
// which is a Go text template, e.g. "mycmd {{.foo}} {{.bar}}".
// The context is the map[string]string data of each csv row.
// The rendered cmdline is trim spaced and parsed by shlex.
// If a renderred cmdline is empty string (after trim spaced), skip that row.
// If noHeader is true, the input csv has no header, columns implicit to "c1", "c2"... .
// If continueOnError is true, continue executing even if an row execution fails,
// which includes template render error, cmd execution error, or cmdline exits non-zero;
// If continueOnError is false, stop the whole flow if any error occurs.
// If dryRun is true, print the cmdline of each row to stderr instead of executing it.
func execCsv(input io.Reader, templateStr string, noHeader bool,
	continueOnError bool, dryRun bool) (successRows, skipRows, errorRows int, err error) {
	// 1. Parse the template initially to ensure it is valid.
	tmpl, err := helper.GetTemplate(templateStr, true)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid template: %w", err)
	}

	reader := csv.NewReader(input)
	var headers []string

	// 2. Handle Header Logic
	if !noHeader {
		// Read the first row as headers
		headers, err = reader.Read()
		if err != nil {
			if err == io.EOF {
				return 0, 0, 0, nil // Empty file, no work to do
			}
			return 0, 0, 0, fmt.Errorf("failed to read header: %w", err)
		}
	}

	// 3. Process Rows
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			if continueOnError {
				errorRows++
				continue
			}
			return successRows, skipRows, errorRows, fmt.Errorf("csv read error: %w", err)
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

		// Execute Template
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			if continueOnError {
				errorRows++
				continue
			}
			return successRows, skipRows, errorRows, fmt.Errorf("template execute error: %w", err)
		}

		// Clean up command string
		cmdLine := strings.TrimSpace(buf.String())
		if cmdLine == "" {
			skipRows++
			continue
		}

		if dryRun {
			fmt.Fprintf(os.Stderr, "Dry run: %s\n", cmdLine)
			successRows++
			continue
		}

		// Parse command arguments using shlex (handling quotes correctly)
		args, err := shlex.Split(cmdLine)
		if err != nil {
			if continueOnError {
				errorRows++
				continue
			}
			return successRows, skipRows, errorRows, fmt.Errorf("shlex parsing error: %w", err)
		}

		if len(args) == 0 {
			skipRows++
			continue
		}

		// Execute Command
		// args[0] is the command, args[1:] are the arguments
		cmd := osexec.Command(args[0], args[1:]...)

		// Connect stdout/stderr to parent process so the user can see output
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			if continueOnError {
				errorRows++
				continue
			}
			return successRows, skipRows, errorRows, fmt.Errorf("command execution failed (%s): %w", cmdLine, err)
		}

		successRows++
	}

	return successRows, skipRows, errorRows, nil
}
