package comfyui

import (
	"github.com/sagan/goaider/cmd"
	"github.com/spf13/cobra"
)

var ComfyuiCmd = &cobra.Command{
	Use:     "comfyui",
	Aliases: []string{"comfy", "cu"},
	Short:   "ComfyUI related actions",
	Long:    `ComfyUI related actions.`,
}

func init() {
	cmd.RootCmd.AddCommand(ComfyuiCmd)
}
