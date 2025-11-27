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

func init() {
	cmd.RootCmd.AddCommand(CsvCmd)
}
