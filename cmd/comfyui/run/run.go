package run

import (
	"fmt"
	"log"
	"path/filepath"

	"github.com/spf13/cobra"

	_ "github.com/richinsley/comfy2go/client"
	_ "github.com/richinsley/comfy2go/graphapi"

	"github.com/sagan/goaider/cmd/comfyui"
	"github.com/sagan/goaider/cmd/comfyui/api"
)

var runCmd = &cobra.Command{
	Use:   "run <workflow.json | ->",
	Short: "Run a ComfyUI workflow and save output",
	Long: `Run a ComfyUI workflow and save output.

Example:
  goaider comfyui run flux.json -s http://127.0.0.1:8888/ -v "41:0:young girl, smiling" -v "31:0:%seed%"`,
	RunE: doRun,
	Args: cobra.ExactArgs(1),
}

var (
	flagForce     bool     // force override
	flagBatch     int      // batch run
	flagOutput    string   // output filename for saving generated image / video.
	flagOutputDir string   // output dir for saving generated image / video.
	flagServer    string   // ComfyUI server, can or "http://ip:port" or "ip:port".
	flagVars      []string // workflow variables
)

func init() {
	runCmd.Flags().BoolVarP(&flagForce, "force", "", false, "Force overwriting existing file(s)")
	runCmd.Flags().IntVarP(&flagBatch, "batch", "b", 1, "Batch run N times")
	runCmd.Flags().StringVarP(&flagOutput, "output", "o", "",
		`Output filename for saving generated image / video. If not set, a random name (e.g. "cu-*.png")is generated`)
	runCmd.Flags().StringVarP(&flagOutputDir, "output-dir", "O", ".",
		"Output directory for saving generated image / video")
	runCmd.Flags().StringVarP(&flagServer, "server", "s", "127.0.0.1:8188",
		`ComfyUI server, can be either "http://ip:port" or "ip:port".`)
	runCmd.Flags().StringArrayVarP(&flagVars, "var", "v", nil,
		`Set workflow node "widgets_values" variable. Format: "node_id:index:value". E.g. "42:0:girl, young child, smiling". `+
			`Can be specified multiple times. Special values: "%seed%" : a random seed`)
	runCmd.MarkFlagRequired("server")
	comfyui.ComfyuiCmd.AddCommand(runCmd)
}

func doRun(cmd *cobra.Command, args []string) (err error) {
	if flagOutput != "" && flagBatch > 1 {
		return fmt.Errorf("cannot use --output with --batch > 1. Use --output-dir instead")
	}
	argWorkflow := args[0]

	for i := range flagBatch {
		seed := api.RandSeed()

		// comfy2go bug: must re-init every time otherwise it's dead lock.
		client, err := api.CreateAndInitComfyClient(flagServer)
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}

		graph, err := api.NewGraph(client, argWorkflow)
		if err != nil {
			return fmt.Errorf("failed to create graph: %w", err)
		}
		log.Printf("%d/%d run workflow (seed=%d)", i+1, flagBatch, seed)

		if len(flagVars) > 0 {
			if err := api.SetGraphNodeWeightValues(graph, flagVars, seed); err != nil {
				return fmt.Errorf("failed to set graph node widget values: %w", err)
			}
		}
		err = client.PrepareGraph(graph)
		if err != nil {
			return fmt.Errorf("failed to prepare graph: %w", err)
		}
		outputs, err := client.RunWorkflow(graph)
		if err != nil {
			return err
		}

		if flagOutput != "" {
			outputPath := flagOutput
			if outputPath != "-" {
				outputPath = filepath.Join(flagOutputDir, outputPath)
			}
			err = outputs.Save(outputPath, flagForce)
		} else {
			err = outputs.SaveAll(flagOutputDir, flagForce)
		}
		if err != nil {
			return err
		}
	}

	return nil
}
