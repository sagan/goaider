package indexfiles

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sagan/goaider/cmd"
	"github.com/sagan/goaider/util"
)

var (
	flagPrefix   string
	flagForce    bool
	flagIncludes []string
)

// indexfilesCmd represents the norfilenames command
var indexfilesCmd = &cobra.Command{
	Use:   "indexfiles <dir> <output_file>",
	Short: "Index files in a directory.",
	Long: `Index files in a directory.

<output_file> : set to "-" to output to stdout.`,
	Args: cobra.ExactArgs(2),
	RunE: indexfiles,
}

func init() {
	cmd.RootCmd.AddCommand(indexfilesCmd)
	indexfilesCmd.Flags().StringVarP(&flagPrefix, "prefix", "", "", `Output data fields name prefix`)
	indexfilesCmd.Flags().StringSliceVarP(&flagIncludes, "includes", "", nil, "Includes fields, comma-separated")
	indexfilesCmd.Flags().BoolVarP(&flagForce, "force", "", false, "Force overwriting without confirmation")
}

func indexfiles(cmd *cobra.Command, args []string) (err error) {
	argInput := args[0]
	argOutput := args[1]
	flagPrefix = strings.TrimSuffix(flagPrefix, "_")

	inputDir, err := filepath.Abs(argInput)
	if err != nil {
		return err
	}
	filelist, err := doIndex(inputDir)
	if err != nil {
		return err
	}

	var output io.Writer
	if argOutput == "-" {
		output = os.Stdout
	} else {
		if exists, err := util.FileExists(argOutput); err != nil || (exists && !flagForce) {
			return fmt.Errorf("output file %q exists or can't access, err=%w", argOutput, err)
		}
		output, err = os.Create(argOutput)
		if err != nil {
			return err
		}
	}

	err = filelist.SaveCsv(output, flagPrefix, flagIncludes)
	if err != nil {
		return err
	}

	return nil
}
