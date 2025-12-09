package jq

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/natefinch/atomic"
	"github.com/sagan/goaider/cmd"
	"github.com/sagan/goaider/constants"
	"github.com/sagan/goaider/util"
	"github.com/sagan/goaider/util/helper"
	"github.com/sagan/goaider/util/stringutil"
	"github.com/spf13/cobra"
)

var jqCmd = &cobra.Command{
	Use:     "jq {file.json | -} -t {filter}",
	Aliases: []string{"jsonquery"},
	Short:   "Query json file, similar to jq but use Go text template to write filter",
	Long: `Query json file, similar to jq but use Go text template to write filter.

If {file.json} is "-", read from stdin.
It outputs to stdout by default.`,
	RunE: doJq,
}

var (
	flagForce    bool
	flagLines    bool
	flagLoose    bool
	flagOutput   string
	flagTemplate string
)

func doJq(cmd *cobra.Command, args []string) (err error) {
	if flagOutput != "-" {
		if exists, err := util.FileExists(flagOutput); err != nil || (exists && !flagForce) {
			return fmt.Errorf("output file %q exists or can't access, err=%w", flagOutput, err)
		}
	}
	argFilename := args[0]
	var file io.Reader
	if argFilename == "-" {
		file = os.Stdin
	} else {
		f, err := os.Open(argFilename)
		if err != nil {
			return err
		}
		defer f.Close()
		file = f
	}
	file = stringutil.GetTextReader(file)
	tpl, err := helper.GetTemplate(flagTemplate, !flagLoose)
	if err != nil {
		return err
	}
	contents, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}

	if flagLines {
		var sb strings.Builder
		lines := stringutil.SplitLines(string(contents))
		for _, line := range lines {
			if line == "" {
				continue
			}
			var data any
			if err := json.Unmarshal([]byte(line), &data); err != nil {
				return fmt.Errorf("failed to unmarshal json line: %w", err)
			}
			output, err := util.ExecTemplate(tpl, data)
			if err != nil {
				return fmt.Errorf("template execute error: %w", err)
			}
			sb.WriteString(output)
			sb.WriteString("\n")
		}
		if flagOutput == "-" {
			_, err = os.Stdout.WriteString(sb.String())
		} else {
			err = atomic.WriteFile(flagOutput, strings.NewReader(sb.String()))
		}
		if err != nil {
			return err
		}
		return nil
	}

	var data any
	if err := json.Unmarshal(contents, &data); err != nil {
		return fmt.Errorf("failed to unmarshal json: %w", err)
	}
	output, err := util.ExecTemplate(tpl, data)
	if err != nil {
		return fmt.Errorf("template execute error: %w", err)
	}
	if flagOutput == "-" {
		_, err = os.Stdout.WriteString(output)
	} else {
		err = atomic.WriteFile(flagOutput, strings.NewReader(output))
	}
	if err != nil {
		return err
	}

	return nil
}

func init() {
	jqCmd.Flags().BoolVarP(&flagLoose, "loose", "", false,
		"Allow loose template parsing (e.g., ignore missing map keys)")
	jqCmd.Flags().BoolVarP(&flagForce, "force", "", false, "Force overwriting without confirmation")
	jqCmd.Flags().BoolVarP(&flagLines, "lines", "L", false,
		`Lines mode. Each line of input is a valid json. Output will also be line by line`)
	jqCmd.Flags().StringVarP(&flagOutput, "output", "o", "-", `Output file path. Use "-" for stdout`)
	jqCmd.Flags().StringVarP(&flagTemplate, "template", "t", "", `Template to format the output. `+
		constants.HELP_TEMPLATE_FLAG)
	jqCmd.MarkFlagRequired("template")
	cmd.RootCmd.AddCommand(jqCmd)
}
