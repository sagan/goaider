package genlist

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/natefinch/atomic"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/goaider/cmd/comfyui"
	"github.com/sagan/goaider/cmd/comfyui/api"
	"github.com/sagan/goaider/config"
	"github.com/sagan/goaider/constants"
	"github.com/sagan/goaider/features/csvfeature"
	"github.com/sagan/goaider/util"
)

const (
	ACTIONS_FILE  = "actions.csv"
	CONTEXTS_FILE = "contexts.csv"
)

var genCmd = &cobra.Command{
	Use:   "genlist",
	Short: "Generate prompts list(s)",
	Long: `Generate prompts list(s).
	
It outputs actions.csv and contexts.csv file that can be used as input in "goaider comfyui batchgen" cmd.`,
	RunE: doGen,
	Args: cobra.ExactArgs(0),
}

var (
	flagForce       bool
	flagTemperature float64
	flagModel       string
	flagModelKey    string
	flatOutput      string // output dir
	flagSubject     string
	flagTheme       string // optional theme, e.g. "daily life", "fairy tale world"
)

func init() {
	genCmd.Flags().BoolVarP(&flagForce, "force", "", false, "Force overwriting without confirmation")
	genCmd.Flags().Float64VarP(&flagTemperature, "temperature", "T", 1.2, constants.HELP_TEMPERATURE_FLAG)
	genCmd.Flags().StringVarP(&flatOutput, "output", "o", ".", `Output dir`)
	genCmd.Flags().StringVarP(&flagModel, "model", "", "", "The model to use. "+constants.HELP_MODEL)
	genCmd.Flags().StringVarP(&flagModelKey, "model-key", "", "", constants.HELP_MODEL_KEY)
	genCmd.Flags().StringVarP(&flagSubject, "subject", "s", "a person", "The subject of the generated prompts")
	genCmd.Flags().StringVarP(&flagTheme, "theme", "t", "",
		`Optional theme for the generated prompts, e.g., "daily life", "fairy tale world"`)

	comfyui.ComfyuiCmd.AddCommand(genCmd)
}

func doGen(cmd *cobra.Command, args []string) (err error) {
	if flagModel == "" {
		flagModel = config.GetDefaultModel()
	}
	actionsFile := filepath.Join(flatOutput, ACTIONS_FILE)
	contextsFile := filepath.Join(flatOutput, CONTEXTS_FILE)
	if exists, err := util.FileExists(actionsFile); err != nil || (exists && !flagForce) {
		return fmt.Errorf("actions file %q exists or can't access, err=%w", actionsFile, err)
	}
	if exists, err := util.FileExists(contextsFile); err != nil || (exists && !flagForce) {
		return fmt.Errorf("contexts file %q exists or can't access, err=%w", contextsFile, err)
	}

	lists, err := api.GenerateActionList(flagModelKey, flagModel, flagSubject, flagTheme, flagTemperature)
	if err != nil {
		return err
	}
	cmd.Printf("lists: %s\n", util.ToJson(lists))

	reader, writer := io.Pipe()
	go func() {
		err := csvfeature.WriteListsToCsv(writer, []string{"action", "action_zh"}, lists.Actions, lists.ActionsZh)
		writer.CloseWithError(err)
	}()
	err1 := atomic.WriteFile(actionsFile, reader)
	log.Printf("Save actions file to %q (err=%v)", actionsFile, err1)

	reader, writer = io.Pipe()
	go func() {
		err := csvfeature.WriteListsToCsv(writer, []string{"context", "context_zh"}, lists.Contexts, lists.ContextsZh)
		writer.CloseWithError(err)
	}()
	err2 := atomic.WriteFile(contextsFile, reader)
	log.Printf("Save actions file to %q (err=%v)", contextsFile, err1)

	if err1 != nil || err2 != nil {
		return fmt.Errorf("files write error: %w, %w", err1, err2)
	}

	return nil
}
