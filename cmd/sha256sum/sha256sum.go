package sha256sum

import (
	"fmt"
	"os"
	"strings"

	"github.com/natefinch/atomic"
	"github.com/sagan/goaider/cmd"
	"github.com/sagan/goaider/util"
	"github.com/spf13/cobra"
)

var sha256sumCmd = &cobra.Command{
	Use:   "sha256sum [file]...",
	Short: "Calculate sha256 hash of files",
	Long: `Calculate sha256 hash of files.
	
If no [file] is provided, or when {file} is -, read stdin.

It outputs in same format as Linux's "sha256sum" util.`,
	RunE: doSha256sum,
}

var (
	flagForce  bool
	flagOutput string
)

func doSha256sum(cmd *cobra.Command, args []string) (err error) {
	if flagOutput != "-" {
		if exists, err := util.FileExists(flagOutput); err != nil || (exists && !flagForce) {
			return fmt.Errorf("output file %q exists or can't access, err=%w", flagOutput, err)
		}
	}

	var outputBuilder strings.Builder

	if len(args) == 0 || (len(args) == 1 && args[0] == "-") {
		// Read from stdin
		hash, err := util.Sha256sum(cmd.InOrStdin(), true)
		if err != nil {
			return fmt.Errorf("failed to calculate SHA256 for stdin: %w", err)
		}
		outputBuilder.WriteString(fmt.Sprintf("%s  -\n", hash))
	} else {
		// Read from files
		for _, filename := range args {
			hash, err := util.Sha256sumFile(filename, true)
			if err != nil {
				// Print error and continue for other files, similar to standard sha256sum
				fmt.Fprintf(os.Stderr, "sha256sum: %s: %v\n", filename, err)
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
	sha256sumCmd.Flags().BoolVarP(&flagForce, "force", "", false, "Force overwriting without confirmation")
	sha256sumCmd.Flags().StringVarP(&flagOutput, "output", "o", "-", `Output file path. Use "-" for stdout`)
	cmd.RootCmd.AddCommand(sha256sumCmd)
}
