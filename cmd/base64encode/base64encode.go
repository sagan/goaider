package base64encode

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vincent-petithory/dataurl"

	"github.com/natefinch/atomic"
	"github.com/sagan/goaider/cmd"
	"github.com/sagan/goaider/util"
)

var base64encodeCmd = &cobra.Command{
	Use:     "base64encode [text]",
	Aliases: []string{"b64e"},
	Short:   "Base64 encode",
	Long:    `Base64 encode.`,
	RunE:    doBase64encode,
}

var (
	flagForce   bool   // Force overwriting existing file
	flagUrl     bool   // Output in URL-safe BASE64 (without padding) encoding instead of standard base64
	flatDataUrl bool   // Output encoded "data:" url. If input is text, assume "text/plain" mime.
	flagOutput  string // Output file. "-" for stdout.
	flagMime    string // Used with --output-url. Set the mime manually.
	flagInput   string // Read from input file instead. "-" for stdin
)

func doBase64encode(cmd *cobra.Command, args []string) (err error) {
	if flagOutput != "-" {
		if exists, err := util.FileExists(flagOutput); err != nil || (exists && !flagForce) {
			return fmt.Errorf("output file %q exists or access failed. err: %w", flagOutput, err)
		}
	}
	if len(args) > 0 && flagInput != "" {
		return fmt.Errorf("cannot use both [text] argument and --input flag")
	}

	var input []byte
	if len(args) > 0 {
		input = []byte(args[0])
	} else if flagInput == "-" {
		input, err = io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
	} else if flagInput != "" {
		input, err = os.ReadFile(flagInput)
		if err != nil {
			return err
		}
	} else {
		return fmt.Errorf("no input provided. Use [text] argument or --input flag")
	}

	var encoded string
	if flatDataUrl {
		if flagMime != "" {
			encoded = dataurl.New(input, flagMime).String()
		} else {
			encoded = dataurl.EncodeBytes(input)
		}
	} else if flagUrl {
		encoded = base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(input)
	} else {
		encoded = base64.StdEncoding.EncodeToString(input)
	}

	if flagOutput == "-" {
		_, err = os.Stdout.WriteString(encoded)
	} else {
		reader := strings.NewReader(encoded)
		err = atomic.WriteFile(flagOutput, reader)
	}
	if err != nil {
		return err
	}
	return nil
}

func init() {
	base64encodeCmd.Flags().BoolVarP(&flagForce, "force", "", false, "Force overwriting existing file")
	base64encodeCmd.Flags().BoolVarP(&flagUrl, "url", "u", false,
		"Output in URL-safe BASE64 (without padding) encoding instead of standard base64")
	base64encodeCmd.Flags().BoolVarP(&flatDataUrl, "data-url", "U", false,
		`Output encoded "data:" url. If input is not from file and is text, assume "text/plain" mime.`)
	base64encodeCmd.Flags().StringVarP(&flagInput, "input", "i", "", `Read input from file. Use "-" for stdin`)
	base64encodeCmd.Flags().StringVarP(&flagOutput, "output", "o", "-", `Output file path. Use "-" for stdout`)
	base64encodeCmd.Flags().StringVarP(&flagMime, "mime", "", "",
		`Used with --data-url. Set the output "data:" url mime manually`)
	cmd.RootCmd.AddCommand(base64encodeCmd)
}
