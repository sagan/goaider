package batchgen

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/sagan/goaider/cmd/comfyui"
	"github.com/sagan/goaider/cmd/comfyui/api"
	"github.com/sagan/goaider/features/csvfeature"
	"github.com/sagan/goaider/util"
	"github.com/sagan/goaider/util/pathutil"
)

var batchGenCmd = &cobra.Command{
	Use:   "batchgen",
	Short: "Run ComfyUI workflow with batch input",
	Long: `Run ComfyUI workflow with batch input.

Features:
- Multi-server support with load balancing.
- Fault tolerance: Retries with exponential backoff on failure.
- Resume capability: Can resume from the last interrupted prompt.

Example:
  goaider comfyui batchgen -w flux.json -s 127.0.0.1:8188 -s 127.0.0.1:8189 -v "41:0:%prompt%" -v "31:0:%rand%" -a actions.csv -c contexts.txt

actions.csv (required) example:
>>>
action,action_zh
Chasing iridescent bubbles through a sunlit field,在阳光普照的田野里追逐彩虹泡泡
Whispering secrets to a wise old tree,向一棵睿智的老树耳语秘密
Dancing with a phantom waltz partner under a moonlit sky,在月光下的天空中与幽灵般的华尔兹舞伴共舞
Painting a masterpiece on a canvas of starlight,在星光画布上绘制杰作
Building a sandcastle kingdom on an alien beach,在异星海滩上建造沙堡王国
<<<

contexts.csv (optional) example:
>>>
context,context_zh
"Ghibli-esque enchanted forest at dawn, soft volumetric lighting",黎明时分，吉卜力风格的魔法森林，柔和的立体光
"Cyberpunk neon street market at night, rain-slicked surfaces",夜幕下的赛博朋克霓虹街市，雨后湿滑的表面
"Baroque grand ballroom, golden hour light streaming through tall windows",巴洛克式大宴会厅，夕阳的余晖透过高大的窗户洒入室内。
"Steampunk airship soaring above a cloud sea, brass and leather textures",蒸汽朋克风格的飞艇翱翔于云海之上，黄铜和皮革质感。
"Ancient Egyptian tomb, torchlight flickering on hieroglyphs",古埃及陵墓，火炬的光芒在象形文字上摇曳。
"Psychedelic dreamscape, swirling vibrant colors and impossible geometry",迷幻的梦境，绚丽的色彩旋转，以及不可思议的几何图形
<<<

In this configuration (5 actions, 6 contextes, batch = 8), it will overall run the workflow 5*6*8 times.
It saves the outputs in standalone folders in output dir, using action_zh as the folder name.

If the program is interrupted for any reason, it prints the "resume token",
run the program again with "--resume <resume_token>" to resume from the last point.`,
	RunE: doBatchGen,
	Args: cobra.ExactArgs(0),
}

type Action struct {
	Action   string `json:"action,omitempty"`
	ActionZh string `json:"action_zh,omitempty"`
}

type Context struct {
	Context   string `json:"context,omitempty"`
	ContextZh string `json:"context_zh,omitempty"`
}

var (
	flagForce    bool     // force override
	flagBatch    int      // batch run
	flagWorkflow string   // workflow file
	flagActions  string   // actions csv file
	flagContext  string   // contexts file
	flagOutput   string   // output dir
	flagServer   []string // ComfyUI servers
	flagVars     []string // workflow variables
	flagResume   string   // resume token "actionIdx:contextIdx"
)

// Global state for resume token generation on interrupt
var (
	currentActionIdx  int
	currentContextIdx int
	stateMutex        sync.Mutex
)

func init() {
	batchGenCmd.Flags().BoolVarP(&flagForce, "force", "", false, "Force overwriting existing file(s)")
	batchGenCmd.Flags().IntVarP(&flagBatch, "batch", "b", 8, "Batch run N times for each prompt")
	batchGenCmd.Flags().StringVarP(&flagOutput, "output", "o", "", "(Required) Output directory")
	batchGenCmd.Flags().StringArrayVarP(&flagVars, "var", "v", nil, `Workflow variables (e.g. 41:0:%prompt%). `+
		`Special values: "%rand%" : a random seed; "%prompt%" : the generated prompt from action & context`)
	batchGenCmd.Flags().StringVarP(&flagWorkflow, "workflow", "w", "", "(Required) Workflow file path")
	batchGenCmd.Flags().StringVarP(&flagActions, "actions", "a", "", "(Required) Actions CSV file")
	batchGenCmd.Flags().StringVarP(&flagContext, "contexts", "c", "", "Contexts CSV file")
	batchGenCmd.Flags().StringArrayVarP(&flagServer, "server", "s", []string{"127.0.0.1:8188"},
		"ComfyUI server address(es)")
	batchGenCmd.Flags().StringVarP(&flagResume, "resume", "r", "", "Resume from token 'actionIdx:contextIdx'")
	batchGenCmd.MarkFlagRequired("workflow")
	batchGenCmd.MarkFlagRequired("output")
	batchGenCmd.MarkFlagRequired("actions")
	comfyui.ComfyuiCmd.AddCommand(batchGenCmd)
}

func doBatchGen(cmd *cobra.Command, args []string) error {
	// 1. Setup Signal Handling for Graceful Shutdown / Resume Info
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		printResumeToken()
		os.Exit(1)
	}()

	// 2. Parse Input Files
	actions, err := loadActions(flagActions)
	if err != nil {
		return err
	}

	contexts, err := loadContexts(flagContext)
	if err != nil {
		return err
	}

	// 3. Initialize Client Pool
	clientPool := make(chan *api.Client, len(flagServer))
	for _, addr := range flagServer {
		client, err := api.CreateAndInitComfyClient(addr)
		if err != nil {
			return fmt.Errorf("failed to init client %s: %w", addr, err)
		}
		clientPool <- client
	}

	// 4. Parse Resume Token
	startAIdx, startCIdx := 0, 0
	if flagResume != "" {
		parts := strings.Split(flagResume, ":")
		if len(parts) == 2 {
			startAIdx, _ = strconv.Atoi(parts[0])
			startCIdx, _ = strconv.Atoi(parts[1])
			log.Printf("Resuming from Action %d, Context %d", startAIdx, startCIdx)
		}
	}

	// 5. Main Execution Loop
	totalBatches := len(actions) * len(contexts)
	processedBatches := 0

	for aIdx, action := range actions {
		// Skip actions if resuming
		if aIdx < startAIdx {
			processedBatches += len(contexts)
			continue
		}

		for cIdx, context := range contexts {
			// Skip contexts if resuming within the starting action
			if aIdx == startAIdx && cIdx < startCIdx {
				processedBatches++
				continue
			}

			// Update global state for signal handler
			updateState(aIdx, cIdx)

			processedBatches++
			log.Printf("Processing batch [%d/%d] | Action: %s... | Context: %s...",
				processedBatches, totalBatches, truncate(action.Action, 20), truncate(context.Context, 20))

			// Construct Prompt and Path
			combinedPrompt := action.Action
			if context.Context != "" {
				combinedPrompt = fmt.Sprintf("%s, %s", combinedPrompt, context.Context)
			}
			subDir := util.FirstNonZeroArg(action.ActionZh, action.Action, "default")
			subDir = pathutil.CleanBasename(subDir)
			finalOutputDir := filepath.Join(flagOutput, subDir)
			if err := os.MkdirAll(finalOutputDir, 0755); err != nil {
				return fmt.Errorf("failed to create dir %s: %w", finalOutputDir, err)
			}

			// Execute the Batch (Parallel runs for this specific Prompt)
			if err := runBatchGroup(ctx, clientPool, combinedPrompt, finalOutputDir, flagBatch); err != nil {
				log.Printf("❌ Batch failed after retries: %v", err)
				printResumeToken()
				return fmt.Errorf("process aborted due to server failure")
			}
		}
	}

	log.Println("✅ All tasks completed successfully.")
	return nil
}

// runBatchGroup runs 'batchSize' tasks in parallel using the available client pool.
// It waits for all to finish. If any fails permanently, it returns an error.
func runBatchGroup(ctx context.Context, pool chan *api.Client, prompt, outputDir string, batchSize int) error {
	g, _ := errgroup.WithContext(ctx)

	for i := range batchSize {
		taskID := i + 1
		g.Go(func() error {
			return runWithRetry(pool, prompt, outputDir, taskID)
		})
	}

	return g.Wait()
}

// runWithRetry attempts to execute a single workflow run.
// It acquires a client, tries to run, and if it fails, backs off and tries another client.
func runWithRetry(pool chan *api.Client, prompt, outputDir string, taskID int) error {
	maxRetries := 5
	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Acquire client
		client := <-pool

		err := executeWorkflow(client, prompt, outputDir)

		// Release client back to pool
		pool <- client

		if err == nil {
			return nil // Success
		}

		log.Printf("⚠️ Task %d failed on %s (Attempt %d/%d): %v", taskID, client.Base, attempt+1, maxRetries+1, err)

		if attempt == maxRetries {
			return fmt.Errorf("task %d exceeded max retries: %w", taskID, err)
		}

		// Exponential Backoff with Jitter
		// We sleep *outside* holding the client, so others can use it (though likely bad)
		// but since we already put it back in the pool, we just sleep this goroutine.
		waitDuration := util.CalculateBackoff(2*time.Second, 60*time.Second, attempt)
		time.Sleep(waitDuration)
	}
	return nil
}

func executeWorkflow(client *api.Client, prompt, outputDir string) error {
	// 1. Create Graph (New instance to avoid state pollution)
	graph, err := api.NewGraph(client, flagWorkflow)
	if err != nil {
		return err
	}

	// 2. Prepare Variables
	seed := api.RandSeed()
	processedVars := make([]string, len(flagVars))
	for i, v := range flagVars {
		v = strings.ReplaceAll(v, "%prompt%", prompt)
		processedVars[i] = v
	}

	if len(processedVars) > 0 {
		if err := api.SetGraphNodeWeightValues(graph, processedVars, seed); err != nil {
			return err
		}
	}

	// 3. Prepare & Run
	if err := client.PrepareGraph(graph); err != nil {
		return err
	}

	outputs, err := client.RunWorkflow(graph)
	if err != nil {
		return err
	}

	// 4. Save
	return outputs.SaveAll(outputDir, flagForce, "")
}

// --- Helper Functions ---

func loadActions(path string) ([]*Action, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open actions: %w", err)
	}
	defer f.Close()
	return csvfeature.UnmarshalCsv[*Action](f)
}

func loadContexts(path string) ([]*Context, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open actions: %w", err)
	}
	defer f.Close()
	return csvfeature.UnmarshalCsv[*Context](f)
}

func updateState(aIdx, cIdx int) {
	stateMutex.Lock()
	currentActionIdx = aIdx
	currentContextIdx = cIdx
	stateMutex.Unlock()
}

func printResumeToken() {
	stateMutex.Lock()
	defer stateMutex.Unlock()
	fmt.Printf("\n\nTo resume from this point, use:\n")
	fmt.Printf("  --resume \"%d:%d\"\n\n", currentActionIdx, currentContextIdx)
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
