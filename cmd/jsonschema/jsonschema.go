package jsonschema

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/JLugagne/jsonschema-infer"
	"github.com/natefinch/atomic"
	"github.com/spf13/cobra"

	"github.com/sagan/goaider/cmd"
	"github.com/sagan/goaider/util"
	"github.com/sagan/goaider/util/helper"
	"github.com/sagan/goaider/util/stringutil"
)

var jsonschemaCmd = &cobra.Command{
	Use:   "jsonschema {file.json | -}",
	Short: "Generate json schema of a json file",
	Long: `Generate json schema of a json file.

The input {file.json} file is used as sample json to infer the schema. Use "-" for stdin.
You can provide multiple sample json files.

Example input json:
  {"name": "foo", "size": 11}

Example output schema:
  {"$schema":"http://json-schema.org/draft-07/schema#","type":"object","properties":{"name":{"type":"string"},"size":{"type":"integer"}},"required":["name","size"]}
`,
	RunE: doJsonschema,
	Args: cobra.MinimumNArgs(1),
}

var (
	flagForce  bool
	flagOutput string
)

func doJsonschema(cmd *cobra.Command, args []string) (err error) {
	if flagOutput != "-" {
		if exists, err := util.FileExists(flagOutput); err != nil || (exists && !flagForce) {
			return fmt.Errorf("output file %q exists or can't access, err=%w", flagOutput, err)
		}
	}

	generator := jsonschema.New()
	files := helper.ParseFilenameArgs(args...)
	for _, file := range files {
		var input io.Reader
		if file == "-" {
			input = cmd.InOrStdin()
		} else {
			f, err := os.Open(file)
			if err != nil {
				return err
			}
			defer f.Close()
			input = f
		}
		input = stringutil.GetTextReader(input)
		contents, err := io.ReadAll(input)
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}
		generator.AddSample(string(contents))
	}

	output, err := generator.Generate()
	if err != nil {
		return err
	}
	if flagOutput == "-" {
		_, err = cmd.OutOrStdout().Write([]byte(output))
	} else {
		err = atomic.WriteFile(flagOutput, strings.NewReader(output))
	}
	if err != nil {
		return err
	}

	return nil
}

func init() {
	jsonschemaCmd.Flags().BoolVarP(&flagForce, "force", "", false, "Force overwriting without confirmation")
	jsonschemaCmd.Flags().StringVarP(&flagOutput, "output", "o", "-", `Output file path. Use "-" for stdout`)
	cmd.RootCmd.AddCommand(jsonschemaCmd)
}
