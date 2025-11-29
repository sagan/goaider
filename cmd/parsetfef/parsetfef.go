package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/xxr3376/gtboard/pkg/ingest"

	"github.com/sagan/goaider/cmd"
	"github.com/sagan/goaider/util"
)

var (
	flagOutput string
)

// Parse an TensorBoard event file
var sttCmd = &cobra.Command{
	Use:     "parsetfef <filename>",
	Aliases: []string{"parsetfevents"},
	Short:   "Parse TensorFlow event file",
	Long: `Parse TensorFlow event file.
	
The original filename is usually events.out.tfevents.{timestamp}.{hostname}.{pid}.{global_uid} format,
which is often used by TensorBoard for visualization.

It displays parsed result to stdout in a human-readable style; if --output is set, it also saves result to it as csv`,
	Args: cobra.ExactArgs(1),
	RunE: parsetfef,
}

func init() {
	sttCmd.Flags().StringVarP(&flagOutput, "output", "o", "", "Save the parsed result to a CSV file")
	cmd.RootCmd.AddCommand(sttCmd)
}

func parsetfef(cmd *cobra.Command, args []string) error {
	r, err := ingest.NewIngester("file", args[0])
	if err != nil {
		return err
	}
	defer r.Close()

	_, err = r.FetchUpdates(context.Background())
	if err != nil {
		return err
	}

	run := r.GetRun()

	util.PrintScalarsTable(run.Scalars)

	if flagOutput != "" {
		err := util.SaveScalarsToCSV(run.Scalars, flagOutput)
		if err != nil {
			return err
		}
	}

	return nil
}
