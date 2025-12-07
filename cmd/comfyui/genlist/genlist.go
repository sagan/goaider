package genlist

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/natefinch/atomic"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/goaider/cmd/comfyui"
	"github.com/sagan/goaider/cmd/comfyui/api"
	"github.com/sagan/goaider/constants"
	"github.com/sagan/goaider/features/csvfeature"
	"github.com/sagan/goaider/features/llm"
	"github.com/sagan/goaider/util"
	"github.com/sagan/goaider/util/stringutil"
)

const (
	ACTIONS_FILE  = "actions.csv"
	CONTEXTS_FILE = "contexts.txt"
)

var genCmd = &cobra.Command{
	Use:   "genlist",
	Short: "Generate prompts list(s)",
	Long:  `Generate prompts list(s).`,
	RunE:  doGen,
	Args:  cobra.ExactArgs(0),
}

var (
	flagForce    bool
	flagModel    string
	flagModelKey string
	flatOutput   string // output dir
)

func init() {
	genCmd.Flags().BoolVarP(&flagForce, "force", "", false, "Force overwriting without confirmation")
	genCmd.Flags().StringVarP(&flatOutput, "output", "o", ".", `Output dir`)
	genCmd.Flags().StringVarP(&flagModel, "model", "", constants.DEFAULT_MODEL,
		"The model to use. "+constants.HELP_MODEL)
	genCmd.Flags().StringVarP(&flagModelKey, "model-key", "", "", constants.HELP_MODEL_KEY)
	comfyui.ComfyuiCmd.AddCommand(genCmd)
}

func doGen(cmd *cobra.Command, args []string) (err error) {
	actionsFile := filepath.Join(flatOutput, ACTIONS_FILE)
	contextsFile := filepath.Join(flatOutput, CONTEXTS_FILE)
	if exists, err := util.FileExists(actionsFile); err != nil || (exists && !flagForce) {
		return fmt.Errorf("actions file %q exists or can't access, err=%w", actionsFile, err)
	}
	if exists, err := util.FileExists(contextsFile); err != nil || (exists && !flagForce) {
		return fmt.Errorf("contexts file %q exists or can't access, err=%w", contextsFile, err)
	}

	lists, err := llm.ChatJsonResponse[api.CreativeLists](flagModelKey, flagModel, api.PromptActionList("a person"))
	if err != nil {
		return err
	}

	fmt.Printf("lists: %s\n", util.ToJson(lists))

	reader, writer := io.Pipe()
	go func() {
		err := csvfeature.WriteListsToCsv(writer, []string{"action", "action_zh"}, lists.Actions, lists.ActionsZh)
		writer.CloseWithError(err)
	}()
	err1 := atomic.WriteFile(actionsFile, reader)
	log.Printf("Save actions file to %q (err=%v)", actionsFile, err1)

	contextsFileContents := strings.Join(util.Map(lists.Contexts, stringutil.ReplaceNewLinesWithSpace), "\n")
	err2 := atomic.WriteFile(contextsFile, strings.NewReader(contextsFileContents))
	log.Printf("Save contexts file to %q (err=%v)", contextsFile, err1)

	if err1 != nil || err2 != nil {
		return fmt.Errorf("files write error: %w, %w", err1, err2)
	}

	return nil
}
