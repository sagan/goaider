package indexfiles

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sagan/goaider/cmd"
	"github.com/sagan/goaider/util"
)

var (
	flagPrefix     string
	flagForce      bool
	flagReadTxt    bool
	flagIncludes   []string
	flagExtensions []string
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
	indexfilesCmd.Flags().StringSliceVarP(&flagExtensions, "extensions", "", nil, "Only Index file of extensions, comma-separated")
	indexfilesCmd.Flags().BoolVarP(&flagForce, "force", "", false, "Force overwriting without confirmation")
	indexfilesCmd.Flags().BoolVarP(&flagReadTxt, "read-txt", "", false, `Read *.txt file contents and store it in data.txt`)
}

func indexfiles(cmd *cobra.Command, args []string) (err error) {
	argInput := args[0]
	argOutput := args[1]
	flagPrefix = strings.TrimSuffix(flagPrefix, "_")
	flagExtensions = util.Map(flagExtensions, func(ext string) string { return strings.TrimPrefix(ext, ".") })

	inputDir, err := filepath.Abs(argInput)
	if err != nil {
		return err
	}
	filelist, err := doIndex(inputDir, flagExtensions)
	if err != nil {
		return err
	}

	if flagReadTxt {
		for _, file := range filelist {
			if file.ExtNodot == "txt" {
				log.Printf("skip read txt for already txt file: %q", file.Path)
				continue
			}
			txtFilePath := filepath.Join(inputDir, file.DirPath, file.Base+".txt")
			contents, err := os.ReadFile(txtFilePath)
			if err != nil {
				log.Printf("failed to read txt %q: %v", txtFilePath, err)
				continue
			}
			txtFileContents := string(contents)
			file.Data["txt"] = txtFileContents
		}
	}

	var output *os.File
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
		defer output.Close()
	}

	err = filelist.SaveCsv(output, flagPrefix, flagIncludes)
	if err != nil {
		return err
	}

	return nil
}
