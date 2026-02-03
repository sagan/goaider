package cat

import (
	"fmt"
	"io"
	"os"

	"github.com/natefinch/atomic"
	"github.com/spf13/cobra"

	"github.com/sagan/goaider/cmd"
	"github.com/sagan/goaider/util"
	"github.com/sagan/goaider/util/helper"
)

var catCmd = &cobra.Command{
	Use:   "cat [file]...",
	Short: "Concatenate files and print on the standard output",
	Long: `Concatenate files and print on the standard output.

If no [file] is provided or [file] is -, read from stdin.

Similar to Linux "cat" utility.`,
	RunE: doCat,
}

var (
	flagForce  bool
	flagOutput string
)

func doCat(cmd *cobra.Command, args []string) (err error) {
	if flagOutput != "-" {
		if exists, err := util.FileExists(flagOutput); err != nil || (exists && !flagForce) {
			return fmt.Errorf("output file %q exists or can't access, err=%w", flagOutput, err)
		}
	}
	var filenames []string
	if len(args) > 0 {
		filenames = helper.ParseFilenameArgs(args...)
	} else {
		filenames = []string{"-"}
	}
	readers := []io.Reader{}
	for _, filename := range filenames {
		if filename == "-" {
			readers = append(readers, cmd.InOrStdin())
		} else {
			file, err := os.Open(filename)
			if err != nil {
				return err
			}
			defer file.Close()
			readers = append(readers, file)
		}
	}
	inputFiles := io.MultiReader(readers...)
	if flagOutput == "-" {
		_, err = io.Copy(cmd.OutOrStdout(), inputFiles)
	} else {
		err = atomic.WriteFile(flagOutput, inputFiles)
	}
	if err != nil {
		return err
	}
	return nil
}

func init() {
	catCmd.Flags().BoolVarP(&flagForce, "force", "", false, "Force overwriting without confirmation")
	catCmd.Flags().StringVarP(&flagOutput, "output", "o", "-", `Output file path. Use "-" for stdout. `+
		`Don't use any input file`)
	cmd.RootCmd.AddCommand(catCmd)
}
