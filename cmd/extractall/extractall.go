package extractall

import (
	"github.com/spf13/cobra"

	"github.com/sagan/goaider/cmd"
)

var (
	flagForce bool
)

// extractallCmd represents the norfilenames command
var extractallCmd = &cobra.Command{
	Use:   "extractall",
	Short: "Extract all archive files.",
	Long:  `Extract all archive files.`,
	Args:  cobra.ExactArgs(1),
	RunE:  extractall,
}

func init() {
	cmd.RootCmd.AddCommand(extractallCmd)
	extractallCmd.Flags().BoolVarP(&flagForce, "force", "", false, "Force overwriting files without confirmation")
}

func extractall(cmd *cobra.Command, args []string) error {
	return nil
}
