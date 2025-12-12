package run

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/goaider/cmd/comfyui"
	"github.com/sagan/goaider/cmd/comfyui/api"
)

var runCmd = &cobra.Command{
	Use:   "run {workflow.json | -}",
	Short: "Run a ComfyUI workflow and save output",
	Long: `Run a ComfyUI workflow and save output.

The {workflow.json} argument can be "-" for reading from stdin.

Example:
  goaider comfyui run flux.json -s 127.0.0.1:8188 -v "41:0:young girl, smiling" -v "31:0:%rand%"`,
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
		`Set workflow node "widgets_values" variable. Format: "node_id:index:value". E.g. "42:0:girl, smiling". `+
			`Can be specified multiple times. Special values: "%rand%" : a random seed`)
	runCmd.MarkFlagRequired("server")
	comfyui.ComfyuiCmd.AddCommand(runCmd)
}

func doRun(cmd *cobra.Command, args []string) (err error) {
	if flagOutput != "" && flagBatch > 1 {
		return fmt.Errorf("cannot use --output with --batch > 1. use --output-dir instead")
	}
	err = os.MkdirAll(flagOutputDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create output directory %q: %w", flagOutputDir, err)
	}
	argWorkflow := args[0]

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		shutdowning := false
		for {
			select {
			case <-sigChan:
				if shutdowning {
					os.Exit(1)
				} else {
					log.Warnf("Received interrupt signal, shutting down... Press ctrl+c again to force exit immediately")
					shutdowning = true
					cancel()
				}
			case <-ctx.Done():
			}
		}
	}()

	client, err := api.CreateAndInitComfyClient(flagServer)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	graph, err := api.NewGraph(client, argWorkflow)
	if err != nil {
		return fmt.Errorf("failed to create graph: %w", err)
	}

	for i := range flagBatch {
		seed := api.RandSeed()
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
		outputs, err := client.RunWorkflow(ctx, graph)
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
			err = outputs.SaveAll(flagOutputDir, flagForce, "")
		}
		if err != nil {
			return err
		}
	}

	return nil
}
