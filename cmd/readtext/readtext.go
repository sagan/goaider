package readtext

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sagan/goaider/cmd"
	"github.com/sagan/goaider/constants"
	"github.com/sagan/goaider/features/textfeature"
)

var readtextCmd = &cobra.Command{
	Use:     "readtext {file.txt | -}",
	Aliases: []string{"rt", "readtxt"},
	Short:   `Read a text file and normalize it, output UTF-8 (without BOM) & \n line break text contents`,
	Long: `Read a text file and normalize it, output UTF-8 (without BOM) & \n line break text contents.

If {file.txt} is "-", read from stdin.

By default it automatically detects the charset of input file.
Use "--charset" flag to manually specify the input file charset.

It outputs to stdout by default. Use "--output" flag to change output path;
Use "--update" flag to update (edit) input file in place.
`,
	RunE: doReadtext,
	Args: cobra.ExactArgs(1),
}

var (
	flagForce                     bool
	flagUpdate                    bool
	flagCharsetDetectionThreshold int
	flagCharset                   string
	flagOutput                    string
)

func doReadtext(cmd *cobra.Command, args []string) (err error) {
	flagCharset = strings.ToLower(flagCharset)
	argInput := args[0]
	overwrite := flagForce
	if flagUpdate {
		if flagOutput != "" && flagOutput != "-" {
			return fmt.Errorf("--update and --output flag are not compatible")
		}
		flagOutput = argInput
		overwrite = true
	}
	err = textfeature.Txt2Utf8(argInput, flagOutput, flagCharset, flagCharsetDetectionThreshold, overwrite, flagForce)
	if err != nil {
		return err
	}
	return nil
}

func init() {
	readtextCmd.Flags().BoolVarP(&flagForce, "force", "", false,
		"Force overwriting without confirmation; Ignore possible charset detection error")
	readtextCmd.Flags().BoolVarP(&flagUpdate, "update", "", false, `Update input file in place`)
	readtextCmd.Flags().IntVarP(&flagCharsetDetectionThreshold, "charset-detection-threshold", "", 100,
		`Confidence threshold for charset detection, [0-100]. `+
			`If the confidence is lower than this value, it will return an error.`)
	readtextCmd.Flags().StringVarP(&flagCharset, "charset", "c", constants.AUTO,
		`Force input file charset. If not set, it will try to detect it. Any of: "`+constants.AUTO+`", `+
			constants.HELP_CHARSETS)
	readtextCmd.Flags().StringVarP(&flagOutput, "output", "o", "-", `Output file path. Use "-" for stdout.`)
	cmd.RootCmd.AddCommand(readtextCmd)
}
