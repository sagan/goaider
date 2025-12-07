package base64decode

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/natefinch/atomic"
	"github.com/spf13/cobra"
	"github.com/vincent-petithory/dataurl"
	"golang.org/x/term"

	"github.com/sagan/goaider/cmd"
	"github.com/sagan/goaider/util"
	"github.com/sagan/goaider/util/stringutil"
)

var base64decodeCmd = &cobra.Command{
	Use:     "base64decode [base64_string]",
	Aliases: []string{"b64d"},
	Short:   "Base64 decode",
	Long: `Base64 decode.

Either [base64_string] argument or --input flag is used as source data.
If --input is "-", read from stdin.

It automatically detects the input base64 string type (standard / URL-safe base64, with or without padding).
It's also able to decode a "data:" url and output payload data`,
	RunE: doBase64decode,
}

var (
	flagForce  bool   // Force overwriting existing file
	flagInput  string // Read input from file. "-" for stdin.
	flagOutput string // Output file. "-" for stdout.
)

func doBase64decode(cmd *cobra.Command, args []string) (err error) {
	if len(args) > 0 && flagInput != "" {
		return fmt.Errorf("cannot use both [base64_string] argument and --input flag")
	}
	if flagOutput != "-" {
		if exists, err := util.FileExists(flagOutput); err != nil || (exists && !flagForce) {
			return fmt.Errorf("output file %q exists or access failed. err: %w", flagOutput, err)
		}
	}

	var input io.Reader
	if flagInput != "" {
		if flagInput == "-" {
			input = os.Stdin
		} else {
			f, err := os.Open(flagInput)
			if err != nil {
				return fmt.Errorf("failed to open input file %q: %w", flagInput, err)
			}
			defer f.Close()
			input = f
		}
	} else if len(args) > 0 {
		input = strings.NewReader(args[0])
	} else {
		return fmt.Errorf("no input provided. Use 'base64decode [string]' or 'base64decode -i <file>'")
	}
	input = stringutil.GetTextReader(input)

	inputBytes, err := io.ReadAll(input)
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}
	if !utf8.Valid(inputBytes) {
		return fmt.Errorf("input is not valid UTF-8. Base64 input must be valid UTF-8")
	}
	inputString := string(inputBytes)
	var decodedBytes []byte
	if strings.HasPrefix(inputString, "data:") {
		dataURL, err := dataurl.DecodeString(inputString)
		if err != nil {
			return fmt.Errorf("failed to decode data URL: %w", err)
		}
		decodedBytes = dataURL.Data
	} else if strings.ContainsAny(inputString, "+/=") {
		decodedBytes, err = base64.StdEncoding.DecodeString(inputString)
	} else {
		decodedBytes, err = base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(inputString)
	}
	if err != nil {
		return fmt.Errorf("failed to decode base64: %w", err)
	}

	if flagOutput == "-" {
		if !utf8.Valid(decodedBytes) && !term.IsTerminal(int(os.Stdout.Fd())) && !flagForce {
			return fmt.Errorf("decoded data is %d bytes binary but output is tty. use --force to ignore",
				len(decodedBytes))
		}
		_, err = os.Stdout.Write(decodedBytes)
	} else {
		reader := bytes.NewReader(decodedBytes)
		err = atomic.WriteFile(flagOutput, reader)
	}
	if err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	return nil
}

func init() {
	base64decodeCmd.Flags().BoolVarP(&flagForce, "force", "", false,
		"Force overwriting existing file / Force output binary to tty")
	base64decodeCmd.Flags().StringVarP(&flagInput, "input", "i", "",
		`Input file path. Use "-" for stdin. If not set, use args[0] as base64 string`)
	base64decodeCmd.Flags().StringVarP(&flagOutput, "output", "o", "-", `Output file path. Use "-" for stdout`)
	cmd.RootCmd.AddCommand(base64decodeCmd)
}
