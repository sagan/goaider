package readtext

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/saintfish/chardet"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/goaider/cmd"
	"github.com/sagan/goaider/constants"
	"github.com/sagan/goaider/util/helper"
	"github.com/sagan/goaider/util/stringutil"
)

var readtextCmd = &cobra.Command{
	Use:     "readtext {file.txt | -}",
	Aliases: []string{"rt"},
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
	err = helper.InputFileAndOutput(argInput, flagOutput, false, overwrite, func(r io.Reader, w io.Writer,
		inputName, outputName string) (err error) {
		if flagCharset == "utf-8" {
			_, err = io.Copy(w, stringutil.GetTextReader(r))
			return err
		} else if flagCharset != "" && flagCharset != constants.AUTO {
			var output io.Reader
			output, err = stringutil.DecodeInput(r, flagCharset)
			if err != nil {
				return err
			}
			_, err = io.Copy(w, stringutil.GetTextReader(output))
			return err
		}

		data, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		detector := chardet.NewTextDetector()
		charset, err := detector.DetectBest(data)
		if err != nil || charset.Confidence < flagCharsetDetectionThreshold {
			return fmt.Errorf("can not get text file encoding: guess=%v, err=%v", charset, err)
		}
		log.Printf("detected charset: %v", charset)
		data, err = stringutil.DecodeText(data, charset.Charset, flagForce)
		if err != nil {
			return err
		}
		_, err = io.Copy(w, stringutil.GetTextReader(bytes.NewReader(data)))
		return err
	})
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
