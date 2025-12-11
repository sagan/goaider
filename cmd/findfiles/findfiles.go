package findfiles

import (
	"fmt"
	"io"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/goaider/cmd"
	"github.com/sagan/goaider/features/csvfeature"
	"github.com/sagan/goaider/util"
	"github.com/sagan/goaider/util/stringutil"
)

var findfilesCmd = &cobra.Command{
	Use:   "findfiles",
	Short: "Find files of list file (each line a filename) in input dir and hardlink found files to output dir",
	Long: `Find files of list file (each line a filename) in input dir and hardlink found files to output dir.

It searchs in input dir recursively. If multiple files in input dir found, only first found one is used.

Example:
  goaider findfiles -i input -o output -l filelist.txt  

It outputs missing file list to stdout.`,
	RunE: doFindfiles,
	Args: cobra.ExactArgs(0),
}

var (
	flagDryRun    bool
	flagForce     bool
	flagListFile  string
	flagInput     string // input dir
	flagOutput    string // output dir
	flagCsvColumn string // treat list file as csv, use this column as filename
)

func init() {
	findfilesCmd.Flags().BoolVarP(&flagDryRun, "dry-run", "d", false, "Dry run")
	findfilesCmd.Flags().BoolVarP(&flagForce, "force", "", false, "Force overwriting without confirmation")
	findfilesCmd.Flags().StringVarP(&flagInput, "input", "i", "", `(Required) Input dir`)
	findfilesCmd.Flags().StringVarP(&flagOutput, "output", "o", "",
		`(Required) Output dir. It will be created if not exists`)
	findfilesCmd.Flags().StringVarP(&flagListFile, "list", "l", "",
		`(Required) File contains list of filenames to find, each line is a filename. Use "-" to read from stdin`)
	findfilesCmd.Flags().StringVarP(&flagCsvColumn, "csv-column", "c", "",
		`Optional: Treat --list as a CSV file, specify the column name to use as filename`)
	findfilesCmd.MarkFlagRequired("input")
	findfilesCmd.MarkFlagRequired("output")
	findfilesCmd.MarkFlagRequired("list")
	cmd.RootCmd.AddCommand(findfilesCmd)
}

func doFindfiles(cmd *cobra.Command, args []string) (err error) {
	err = os.MkdirAll(flagOutput, 0755)
	if err != nil {
		return fmt.Errorf("failed to create output directory %q: %w", flagOutput, err)
	}

	var listFile io.Reader
	if flagListFile == "-" {
		listFile = cmd.InOrStdin()
	} else {
		f, err := os.Open(flagListFile)
		if err != nil {
			return fmt.Errorf("failed to open list file: %w", err)
		}
		defer f.Close()
		listFile = f
	}
	listFile = stringutil.GetTextReader(listFile)
	if flagCsvColumn != "" {
		csvData, err := csvfeature.UnmarshalCsv[map[string]string](listFile)
		if err != nil {
			return fmt.Errorf("failed to unmarshal CSV: %w", err)
		}
		var b strings.Builder
		for _, row := range csvData {
			if val, ok := row[flagCsvColumn]; ok {
				fmt.Printf("row: %s\n", util.ToJson(row))
				b.WriteString(val)
				b.WriteByte('\n')
			} else {
				fmt.Printf("invalid row: %s\n", util.ToJson(row))
				return fmt.Errorf("CSV column %q not found in some rows", flagCsvColumn)
			}
		}
		listFile = strings.NewReader(b.String())
	}
	foundFiles, missingFiles, err := HardLinkFiles(listFile, flagInput, flagOutput, flagForce, flagDryRun)
	if err != nil {
		return err
	}
	log.Printf("Found %d files", len(foundFiles))
	if len(missingFiles) > 0 {
		log.Printf("Warning: The following files were not found:\n")
		for _, file := range missingFiles {
			fmt.Printf("%s\n", file)
		}
		return fmt.Errorf("%d files missing", len(missingFiles))
	}
	return nil
}
