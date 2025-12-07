package cmd

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/natefinch/atomic"
	"github.com/spf13/cobra"
	"github.com/xxr3376/gtboard/pkg/ingest"

	"github.com/sagan/goaider/cmd"
	"github.com/sagan/goaider/util"
)

var (
	flagForce         bool
	flagHumanReadable bool
	flagOutput        string
)

// Parse an TensorBoard event file
var sttCmd = &cobra.Command{
	Use:     "parsetfef {filename}",
	Aliases: []string{"parsetfevents"},
	Short:   "Parse TensorFlow event file",
	Long: `Parse TensorFlow event file.
	
The original TensorFlow event filename is usually events.out.tfevents.{timestamp}.{hostname}.{pid}.{global_uid} format,
which is often used by TensorBoard for visualization.

By defaults it outputs result in csv format and outputs to stdout.
If --human-readable flag is set, it outputs in a human friendly column format.
Use --output flag to specify the output file.`,
	Args: cobra.ExactArgs(1),
	RunE: parsetfef,
}

func init() {
	sttCmd.Flags().BoolVarP(&flagForce, "force", "", false, "Force overwriting without confirmation")
	sttCmd.Flags().StringVarP(&flagOutput, "output", "o", "-", `Output file path. Use "-" for stdout`)
	sttCmd.Flags().BoolVarP(&flagHumanReadable, "human-readable", "H", false,
		"Output in a human friendly column format instead of CSV")
	cmd.RootCmd.AddCommand(sttCmd)
}

func parsetfef(cmd *cobra.Command, args []string) (err error) {
	if flagOutput != "-" {
		if exists, err := util.FileExists(flagOutput); err != nil || (exists || !flagForce) {
			return fmt.Errorf("output file %q exists or access failed. err: %w", flagOutput, err)
		}
	}
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

	reader, writer := io.Pipe()
	go func() {
		var err error
		if flagHumanReadable {
			err = util.PrintScalarsTable(writer, run.Scalars)
		} else {
			err = util.SaveScalarsToCSV(writer, run.Scalars)
		}
		writer.CloseWithError(err)
	}()
	if flagOutput == "-" {
		_, err = io.Copy(os.Stdout, reader)
	} else {
		err = atomic.WriteFile(flagOutput, reader)
	}
	if err != nil {
		return err
	}
	return nil
}
