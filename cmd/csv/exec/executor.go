package exec

import "io"

// read csv file, execute a cmd for each line of the csv.
// the cmdline to exeute is generated from template argument,
// which is a Go text template, e.g. "mycmd {{.foo}} {{.bar}}",
// the context is the map[string]any data of each csv row.
func execCsv(input io.Reader, template string) (cmdlineErrors int, err error) {
	return 0, nil
}
