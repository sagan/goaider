package render

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/natefinch/atomic"
	"github.com/spf13/cobra"

	"github.com/sagan/goaider/cmd"
	"github.com/sagan/goaider/util"
	"github.com/sagan/goaider/util/helper"
	"github.com/sagan/goaider/util/stringutil"
)

var renderCmd = &cobra.Command{
	Use:   "render {template} -v name=value",
	Short: "Render a Go text template",
	Long: `Render a Go text template.

The {template} is Go text template string, if it starts with "@", treat it (everything after "@") as filename
and load template contents from file instead.
Alternatively, you can also use --input flag to specify the template file name.

See https://pkg.go.dev/text/template for help about Go text template.
All sprout functions are supported, see https://github.com/go-sprout/sprout .

The data of template is map[string]any which has these internal fields:
- env : current process environment variables map[string]string.
Additional data fields can be provided by "--variable" flag.

Example:
  goaider render "{{.name}}" -v name=foo

It outputs to stdout by default.`,
	RunE: doRender,
	Args: cobra.MaximumNArgs(1),
}

var (
	flagForce     bool
	flagLoose     bool
	flagOutput    string
	flagInput     string
	flagVariables []string
)

func doRender(cmd *cobra.Command, args []string) (err error) {
	if flagOutput != "-" {
		if exists, err := util.FileExists(flagOutput); err != nil || (exists && !flagForce) {
			return fmt.Errorf("output file %q exists or can't access, err=%w", flagOutput, err)
		}
	}
	template := ""
	if flagInput != "" {
		template = "@" + flagInput
	} else if len(args) > 0 {
		template = args[0]
	} else {
		return fmt.Errorf("no input")
	}
	tpl, err := helper.GetTemplate(template, !flagLoose)
	if err != nil {
		return err
	}

	data := map[string]any{}
	for _, v := range flagVariables {
		parts := strings.SplitN(v, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf(`invalid variable format: %q. Expected "name=value"`, v)
		}
		name := parts[0]
		value := parts[1]
		var parsedValue any
		if strings.HasPrefix(value, "@") {
			filename := value[1:]
			ext := filepath.Ext(filename)
			file, err := os.Open(filename)
			if err != nil {
				return fmt.Errorf("read %q variable file error: %v", filename, err)
			}
			defer file.Close()
			reader, contentType, err := util.DetectContentType(file)
			if err != nil {
				return fmt.Errorf("%q: %v", filename, err)
			}
			if !strings.HasPrefix(contentType, "text/") {
				return fmt.Errorf("%q: content type %q is not text", filename, contentType)
			}
			reader = stringutil.GetTextReader(reader)
			if ext == ".json" || ext == ".yaml" || ext == ".yml" || ext == ".toml" {
				parsedValue, err = util.Unmarshal(reader, ext)
				if err != nil {
					return fmt.Errorf("%q: %v", filename, err)
				}
			} else {
				var bdata []byte
				bdata, err = io.ReadAll(reader)
				if err != nil {
					return fmt.Errorf("%q: %v", filename, err)
				}
				parsedValue = string(bdata)
			}
		} else if err := json.Unmarshal([]byte(value), &parsedValue); err != nil {
			// If not JSON, treat as string
			parsedValue = value
		}
		data[name] = parsedValue
	}
	if _, ok := data["env"]; !ok {
		data["env"] = util.GetEnvMap()
	}

	output, err := tpl.Exec(data)
	if err != nil {
		return fmt.Errorf("template execute error: %w", err)
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
	renderCmd.Flags().BoolVarP(&flagLoose, "loose", "", false,
		"Allow loose template parsing (e.g., ignore missing map keys)")
	renderCmd.Flags().BoolVarP(&flagForce, "force", "", false, "Force overwriting without confirmation")
	renderCmd.Flags().StringVarP(&flagOutput, "output", "o", "-", `Output file path. Use "-" for stdout`)
	renderCmd.Flags().StringVarP(&flagInput, "input", "i", "", `Input template filename. Use "-" for stdin`)
	renderCmd.Flags().StringArrayVarP(&flagVariables, "variable", "v", nil,
		`Variables to pass to the template. Format: "name=value". Can be set multiple times. `+
			`Values are parsed as JSON if possible, otherwise as string. `+
			`Specially, if value starts with "@", treat it (everything after "@") as filename and read it's contents; `+
			`if file extension is json / yaml (yml) / toml, parse file contents as structured data`)
	cmd.RootCmd.AddCommand(renderCmd)
}
