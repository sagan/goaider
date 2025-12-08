package parsemeta

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/natefinch/atomic"
	"github.com/spf13/cobra"

	"github.com/sagan/goaider/cmd/comfyui"
	"github.com/sagan/goaider/cmd/comfyui/api"
	"github.com/sagan/goaider/constants"
	"github.com/sagan/goaider/util"
	"github.com/sagan/goaider/util/helper"
)

var parseMetaCmd = &cobra.Command{
	Use:   "parsemeta {filename | -}",
	Short: "Parse meta (workflow & prompt) info from ComfyUI generated .png image file",
	Long: `Parse meta (workflow & prompt) info from ComfyUI generated .png image file.

It extracts the 'workflow' and 'prompt' data embedded in the tEXt chunks of the PNG file.
By default, it outputs the extracted whole metadata {workflow, prompt} as a JSON string to stdout.
Use --output flag to specify the output file.
Use --template flag to format the output. The template can access ".workflow" and ".prompt" fields.

If {filename} is "-", read from stdin.

Examples:
  goaider comfyui parsemeta input.png
  goaider comfyui parsemeta input.png -o output.json
  goaider comfyui parsemeta input.png -t "{{.prompt.6.inputs.text}}"
  goaider comfyui parsemeta input.png -t "{{toJSON .workflow}}"
`,
	Args: cobra.ExactArgs(1),
	RunE: doParseMeta,
}

var (
	flagForce    bool // override existing file
	flagTemplate string
	flagOutput   string
)

func init() {
	parseMetaCmd.Flags().BoolVarP(&flagForce, "force", "", false, "Override existing file")
	parseMetaCmd.Flags().StringVarP(&flagTemplate, "template", "t", "", `Template to format the output. `+
		constants.HELP_TEMPLATE_FLAG)
	parseMetaCmd.Flags().StringVarP(&flagOutput, "output", "o", "-", `Output file path. Use "-" for stdout`)
	comfyui.ComfyuiCmd.AddCommand(parseMetaCmd)
}

func doParseMeta(cmd *cobra.Command, args []string) (err error) {
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
	meta, err := api.ExtractComfyMetadata(file)
	if err != nil {
		return fmt.Errorf("error reading metadata: %v", err)
	}

	var output string
	if flagTemplate != "" {
		tmpl, err := helper.GetTemplate(flagTemplate, true)
		if err != nil {
			return fmt.Errorf("invalid template: %w", err)
		}
		output, err = util.ExecTemplate(tmpl, map[string]any{
			"workflow": meta.Workflow,
			"prompt":   meta.Prompt,
		})
		if err != nil {
			return fmt.Errorf("template execute error: %w", err)
		}
	} else {
		output = util.ToJson(meta)
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
