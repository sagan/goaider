package genprompts

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	_ "github.com/richinsley/comfy2go/client"
	_ "github.com/richinsley/comfy2go/graphapi"

	"github.com/sagan/goaider/cmd/comfyui"
	"github.com/sagan/goaider/cmd/comfyui/api"
	"github.com/sagan/goaider/constants"
	"github.com/sagan/goaider/features/llm"
	"github.com/sagan/goaider/util"
)

var runCmd = &cobra.Command{
	Use:   "genprompts ",
	Short: "Generate prompt list",
	Long:  `Generate prompt list.`,
	RunE:  doGen,
	Args:  cobra.ExactArgs(0),
}

var (
	flagModel string
)

func init() {
	runCmd.Flags().StringVarP(&flagModel, "model", "", constants.DEFAULT_GEMINI_MODEL, "The model to use")
	comfyui.ComfyuiCmd.AddCommand(runCmd)
}

func doGen(cmd *cobra.Command, args []string) (err error) {
	apiKey := os.Getenv(constants.ENV_GEMINI_API_KEY)
	if apiKey == "" {
		return fmt.Errorf("GEMINI_API_KEY environment variable not set")
	}
	lists, err := llm.GeminiJsonResponse[api.CreativeLists](apiKey, flagModel, api.PromptActionList("a person"))
	if err != nil {
		return err
	}
	fmt.Print(util.ToJson(lists))
	return nil
}
