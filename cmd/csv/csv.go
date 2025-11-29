package csv

import (
	"github.com/sagan/goaider/cmd"
	"github.com/spf13/cobra"
)

var CsvCmd = &cobra.Command{
	Use:   "csv",
	Short: "csv file operations",
	Long:  `csv file operations.`,
}

var (
	FlagForce    bool   // force overwrite existing file
	FlagOutput   string // output file, set to "-" to output to stdout
	FlagNoHeader bool   // treat input csv files as no header row. Columns implicit to "c1", "c2"...
)

func init() {
	cmd.RootCmd.AddCommand(CsvCmd)
	CsvCmd.PersistentFlags().BoolVarP(&FlagNoHeader, "no-header", "n", false,
		`Treat input csv files as no header row. Columns implicit to "c1", "c2"...`)
	CsvCmd.PersistentFlags().BoolVarP(&FlagForce, "force", "", false, "Force overwriting files without confirmation.")
	CsvCmd.PersistentFlags().StringVarP(&FlagOutput, "output", "o", "-",
		`Output file path (if applicable). Use "-" for stdout.`)
}
