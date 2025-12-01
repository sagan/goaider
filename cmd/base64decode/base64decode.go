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
	"github.com/sagan/goaider/cmd"
	"github.com/sagan/goaider/util"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var base64decodeCmd = &cobra.Command{
	Use:     "base64decode [base64_string]",
	Aliases: []string{"b64d"},
	Short:   "Base64 decode",
	Long: `Base64 decode.

If <filename> is "-", read from stdin.

It automatically detects the input base64 string type (standard / URL-safe base64, with or without padding).`,
	RunE: doBase64decode,
}

var (
	flagForce  bool   // Force overwriting existing file
	flagInput  string // Read input from file. "-" for stdin.
	flagOutput string // Output file. "-" for stdout.
)

func doBase64decode(cmd *cobra.Command, args []string) (err error) {
	var inputReader io.Reader
	var outputWriter io.Writer

	if len(args) > 0 && flagInput != "" {
		return fmt.Errorf("cannot use both [base64_string] argument and --input flag")
	}

	// Determine input source
	if flagInput != "" {
		if flagInput == "-" {
			inputReader = os.Stdin
		} else {
			inputReader, err = os.Open(flagInput)
			if err != nil {
				return fmt.Errorf("failed to open input file %q: %w", flagInput, err)
			}
		}
	} else if len(args) > 0 {
		inputReader = strings.NewReader(args[0])
	} else {
		return fmt.Errorf("no input provided. Use 'base64decode [string]' or 'base64decode -i <file>'")
	}

	// Determine output destination
	if flagOutput == "-" {
		outputWriter = os.Stdout
	} else {
		outputWriter, err = os.Create(flagOutput)
		if err != nil {
			return fmt.Errorf("failed to create output file %q: %w", flagOutput, err)
		}
		defer outputWriter.(*os.File).Close()
	}

	inputBytes, err := io.ReadAll(inputReader)
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}
	if !utf8.Valid(inputBytes) {
		return fmt.Errorf("input is not valid UTF-8. Base64 input must be valid UTF-8")
	}
	inputString := string(inputBytes)
	var decodedBytes []byte
	if strings.ContainsAny(inputString, "+/") {
		decodedBytes, err = base64.StdEncoding.DecodeString(inputString)
	} else {
		decodedBytes, err = base64.URLEncoding.DecodeString(inputString)
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
		if exists, err := util.FileExists(flagOutput); err != nil || (exists && !flagForce) {
			return fmt.Errorf("output file %q exists or access failed. err: %w", flagOutput, err)
		}
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
