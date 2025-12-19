package sha1sum

import (
	"fmt"
	"strings"

	"github.com/natefinch/atomic"
	"github.com/spf13/cobra"

	"github.com/sagan/goaider/cmd"
	"github.com/sagan/goaider/constants"
	"github.com/sagan/goaider/util"
	"github.com/sagan/goaider/util/helper"
)

var sha1sumCmd = &cobra.Command{
	Use:   "sha1sum [file]...",
	Short: "Calculate sha1 hash of files",
	Long: `Calculate sha1 hash of files.
	
If no [file] is provided, or when {file} is -, read stdin.

It outputs in same format as Linux's "sha1sum" util.`,
	RunE: doSha1sum,
}

var (
	flagForce  bool
	flagOutput string
)

func doSha1sum(cmd *cobra.Command, args []string) (err error) {
	if flagOutput != "-" {
		if exists, err := util.FileExists(flagOutput); err != nil || (exists && !flagForce) {
			return fmt.Errorf("output file %q exists or can't access, err=%w", flagOutput, err)
		}
	}

	var outputBuilder strings.Builder

	if len(args) == 0 || (len(args) == 1 && args[0] == "-") {
		// Read from stdin
		hash, err := util.Hash(cmd.InOrStdin(), constants.HASH_SHA1, true)
		if err != nil {
			return fmt.Errorf("failed to calculate SHA1 for stdin: %w", err)
		}
		outputBuilder.WriteString(fmt.Sprintf("%s  -\n", hash))
	} else {
		filenames := helper.ParseFilenameArgs(args...)
		for _, filename := range filenames {
			hash, err := util.HashFile(filename, constants.HASH_SHA1, true)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "sha1sum: %s: %v\n", filename, err)
				continue
			}
			outputBuilder.WriteString(fmt.Sprintf("%s  %s\n", hash, filename))
		}
	}

	output := outputBuilder.String()

	if flagOutput == "-" {
		_, err = fmt.Print(output)
	} else {
		err = atomic.WriteFile(flagOutput, strings.NewReader(output))
	}
	if err != nil {
		return err
	}

	return nil
}

func init() {
	sha1sumCmd.Flags().BoolVarP(&flagForce, "force", "", false, "Force overwriting without confirmation")
	sha1sumCmd.Flags().StringVarP(&flagOutput, "output", "o", "-", `Output file path. Use "-" for stdout`)
	cmd.RootCmd.AddCommand(sha1sumCmd)
}
