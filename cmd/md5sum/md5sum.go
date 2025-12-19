package md5sum

import (
	"fmt"
	"strings"

	"github.com/natefinch/atomic"
	"github.com/sagan/goaider/cmd"
	"github.com/sagan/goaider/constants"
	"github.com/sagan/goaider/util"
	"github.com/sagan/goaider/util/helper"
	"github.com/spf13/cobra"
)

var md5sumCmd = &cobra.Command{
	Use:   "md5sum [file]...",
	Short: "Calculate md5 hash of files",
	Long: `Calculate md5 hash of files.
	
If no [file] is provided, or when {file} is -, read stdin.

It outputs in same format as Linux's "md5sum" util.`,
	RunE: doMd5sum,
}

var (
	flagForce  bool
	flagOutput string
)

func doMd5sum(cmd *cobra.Command, args []string) (err error) {
	if flagOutput != "-" {
		if exists, err := util.FileExists(flagOutput); err != nil || (exists && !flagForce) {
			return fmt.Errorf("output file %q exists or can't access, err=%w", flagOutput, err)
		}
	}

	var outputBuilder strings.Builder

	if len(args) == 0 || (len(args) == 1 && args[0] == "-") {
		// Read from stdin
		hash, err := util.Hash(cmd.InOrStdin(), constants.HASH_MD5, true)
		if err != nil {
			return fmt.Errorf("failed to calculate MD5 for stdin: %w", err)
		}
		outputBuilder.WriteString(fmt.Sprintf("%s  -\n", hash))
	} else {
		filenames := helper.ParseFilenameArgs(args...)
		// Read from files
		for _, filename := range filenames {
			hash, err := util.HashFile(filename, constants.HASH_MD5, true)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "md5sum: %s: %v\n", filename, err)
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
	md5sumCmd.Flags().BoolVarP(&flagForce, "force", "", false, "Force overwriting without confirmation")
	md5sumCmd.Flags().StringVarP(&flagOutput, "output", "o", "-", `Output file path. Use "-" for stdout`)
	cmd.RootCmd.AddCommand(md5sumCmd)
}
