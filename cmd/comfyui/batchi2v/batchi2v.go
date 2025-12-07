package batchi2v

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/sagan/goaider/cmd/comfyui"
	"github.com/sagan/goaider/cmd/comfyui/api"
	"github.com/sagan/goaider/constants"
	"github.com/sagan/goaider/features/llm"
	"github.com/sagan/goaider/util"
)

const DEFAULT_PROMPT = `Analyze this image that
1. Write a detailed cinematic prompt for video generation (focus on motion).
2. Write a very short 3-5 word Simplified Chinese summary for the filename of generated video.
`

// Define the structure we want Gemini to return
type I2VResponse struct {
	// The detailed prompt for the video generation model
	Prompt string `json:"prompt" jsonschema:"description=A detailed and cinematic description of the movement and scene for video generation that's under 50 words."`
	// A short, filename-friendly description (e.g., "cat_running_grass")
	ShortDescriptionZh string `json:"short_description_zh" jsonschema:"description=A very short (3-5 word) Simplified Chinese summary suitable for a filename."`
}

var batchI2VCmd = &cobra.Command{
	Use:   "batchi2v",
	Short: "Batch Image-to-Video generation using ComfyUI and LLM",
	Long: `Batch convert images to videos using a ComfyUI workflow (e.g., Wan2.2, SVD).

It scans a directory for images, uses Gemini to generate a prompt describing the image -> action,
and executes the ComfyUI workflow.

Example:
  goaider comfyui batchi2v -i images/ -o videos/ -w wan2.2_i2v.json \
	  -v "10:0:%image%" -v "20:0:%prompt%" -v "30:0:%rand%"

Resume Example:
  goaider comfyui batchi2v ... --resume "image_basename"
`,
	RunE: doBatchI2V,
	Args: cobra.ExactArgs(0),
}

var (
	flagForce      bool
	flagNoPrompt   bool
	flagBatch      int
	flagWorkflow   string
	flagInput      string
	flagOutput     string
	flagModel      string
	flagModelKey   string
	flagPromptTmpl string
	flagResume     string
	flagServer     []string
	flagVars       []string
)

func init() {
	batchI2VCmd.Flags().BoolVar(&flagForce, "force", false, "Force overwrite existing videos")
	batchI2VCmd.Flags().IntVarP(&flagBatch, "batch", "b", 1, "Number of variations to generate per image")
	batchI2VCmd.Flags().StringVarP(&flagWorkflow, "workflow", "w", "", "(Required) Workflow JSON file")
	batchI2VCmd.Flags().StringVarP(&flagInput, "input", "i", "", "(Required) Directory containing input images")
	batchI2VCmd.Flags().StringVarP(&flagOutput, "output", "o", "", "(Required) Directory to save generated videos")
	batchI2VCmd.Flags().StringArrayVarP(&flagServer, "server", "s", []string{"127.0.0.1:8188"},
		"ComfyUI server address(es)")
	batchI2VCmd.Flags().StringArrayVarP(&flagVars, "var", "v", nil,
		`Set variables. Use "%image%" for input image, "%prompt%" for LLM text, "%rand%" for random seed.`)
	batchI2VCmd.Flags().StringVarP(&flagModel, "model", "", constants.DEFAULT_MODEL,
		"The Model to use. "+constants.HELP_MODEL)
	batchI2VCmd.Flags().StringVarP(&flagModelKey, "model-key", "", "", constants.HELP_MODEL_KEY)
	batchI2VCmd.Flags().BoolVarP(&flagNoPrompt, "no-prompt", "", false, "Skip LLM prompt generation")
	batchI2VCmd.Flags().StringVarP(&flagPromptTmpl, "prompt-template", "", DEFAULT_PROMPT,
		"Instruction prompt for the LLM")
	batchI2VCmd.Flags().StringVarP(&flagResume, "resume", "", "",
		"Resume from this image basename (skips alphabetically previous images)")
	batchI2VCmd.MarkFlagRequired("workflow")
	batchI2VCmd.MarkFlagRequired("input")
	batchI2VCmd.MarkFlagRequired("output")
	comfyui.ComfyuiCmd.AddCommand(batchI2VCmd)
}

type i2vTask struct {
	imagePath   string
	baseName    string
	imgIndex    int
	batchIndex  int
	totalImages int
}

// Global thread-safe tracker for completion
type ProgressTracker struct {
	sync.Mutex
	completedFiles map[string]int // counts completed batches per file
	fileList       []string       // sorted list of all files to process
	batchSize      int
}

func (p *ProgressTracker) MarkDone(baseName string) {
	p.Lock()
	defer p.Unlock()
	p.completedFiles[baseName]++
}

func (p *ProgressTracker) GetResumeString() string {
	p.Lock()
	defer p.Unlock()
	// Find the first file in the sorted list that hasn't completed all batches
	for _, f := range p.fileList {
		baseName := strings.TrimSuffix(f, filepath.Ext(f))
		if count, ok := p.completedFiles[baseName]; !ok || count < p.batchSize {
			return baseName
		}
	}
	return "" // All done
}

func doBatchI2V(cmd *cobra.Command, args []string) error {
	// 1. Setup Context with Signal Cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle Ctrl+C to print resume string
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		shutdowning := false
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
	}()

	files, _ := os.ReadDir(flagInput)
	var validFiles []string
	supportedExts := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true}

	for _, f := range files {
		if !f.IsDir() && supportedExts[strings.ToLower(filepath.Ext(f.Name()))] {
			validFiles = append(validFiles, f.Name())
		}
	}

	// 2. Sort files to ensure deterministic order for resuming
	sort.Strings(validFiles)

	// 3. Initialize Progress Tracker
	tracker := &ProgressTracker{
		completedFiles: make(map[string]int),
		fileList:       validFiles,
		batchSize:      flagBatch,
	}

	// 4. Defer the Resume String printing
	defer func() {
		resumePoint := tracker.GetResumeString()
		if resumePoint != "" {
			fmt.Println("\n---------------------------------------------------------")
			fmt.Printf("⚠️  Process incomplete. To resume from the current position:\n")
			fmt.Printf("   goaider comfyui batchi2v ... --resume \"%s\"\n", resumePoint)
			fmt.Println("---------------------------------------------------------")
		} else {
			log.Println("✅ All tasks completed successfully.")
		}
	}()

	// Filter tasks based on resume flag
	tasksToProcess := 0
	if flagResume != "" {
		log.Printf("⏩ Resuming from: %s (skipping previous files)", flagResume)
	}

	totalTasks := len(validFiles) * flagBatch // Rough upper bound
	taskChan := make(chan i2vTask, totalTasks)

	// Fill Channel
	go func() {
		defer close(taskChan)
		for i, fName := range validFiles {
			baseName := strings.TrimSuffix(fName, filepath.Ext(fName))

			// Resume Logic: Skip if baseName is lexicographically smaller than resume string
			if flagResume != "" {
				if strings.Compare(baseName, flagResume) < 0 {
					// Mark as done in tracker so we don't suggest resuming from it
					for b := 0; b < flagBatch; b++ {
						tracker.MarkDone(baseName)
					}
					continue
				}
			}

			absPath := filepath.Join(flagInput, fName)
			tasksToProcess++

			for b := 1; b <= flagBatch; b++ {
				select {
				case <-ctx.Done():
					return
				case taskChan <- i2vTask{
					imagePath:   absPath,
					baseName:    baseName,
					imgIndex:    i + 1,
					batchIndex:  b,
					totalImages: len(validFiles),
				}:
				}
			}
		}
	}()

	if tasksToProcess == 0 && flagResume != "" {
		log.Warnf("No files found after resume point '%s'. Check if filename is correct.", flagResume)
		return nil
	}

	clientPool := make(chan *api.Client, len(flagServer))
	for _, addr := range flagServer {
		c, err := api.CreateAndInitComfyClient(addr)
		if err != nil {
			return err
		}
		clientPool <- c
	}

	g, gCtx := errgroup.WithContext(ctx)
	// Start Workers
	for i := 0; i < len(flagServer); i++ {
		g.Go(func() error { return i2vWorker(gCtx, clientPool, taskChan, tracker) })
	}

	return g.Wait()
}

func i2vWorker(ctx context.Context, pool chan *api.Client, tasks <-chan i2vTask, tracker *ProgressTracker) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case task, ok := <-tasks:
			if !ok {
				return nil
			}
			if err := processI2V(ctx, pool, task); err != nil {
				log.Printf("❌ Failed task [%s #%d]: %v", task.baseName, task.batchIndex, err)
				if ctx.Err() != nil {
					return ctx.Err()
				}
				// Note: We do NOT mark done here, so resume will pick it up
			} else {
				// Success
				tracker.MarkDone(task.baseName)
			}
		}
	}
}

func processI2V(ctx context.Context, pool chan *api.Client, task i2vTask) error {
	log.Printf("[Img %d/%d | Batch %d/%d] Processing %s...",
		task.imgIndex, task.totalImages, task.batchIndex, flagBatch, task.baseName)

	llmResp := &I2VResponse{}

	// A. Generate Structured Prompt via LLM
	if !flagNoPrompt {
		imgData, err := os.ReadFile(task.imagePath)
		if err != nil {
			return fmt.Errorf("read image: %w", err)
		}

		mimeType := util.GetMimeType(task.imagePath)

		err = retry(3, func() error {
			var lErr error
			llmResp, lErr = llm.ImageToJson[I2VResponse](
				flagModelKey,
				flagModel,
				flagPromptTmpl, // Template should instruct to fill the JSON fields
				imgData,
				mimeType,
			)
			log.Printf("llm request %s, err=%v, response: %s", flagPromptTmpl, lErr, util.ToJson(llmResp))
			return lErr
		})

		if err != nil {
			return fmt.Errorf("LLM generation failed: %w", err)
		}
		log.Printf("    📝 [%s] Desc: %s", task.baseName, llmResp.ShortDescriptionZh)
	} else {
		// Default if no prompt
		llmResp.ShortDescriptionZh = "batch"
	}

	// B. ComfyUI Execution
	maxRetries := 5
	baseDelay := 2 * time.Second

	for attempt := 0; attempt <= maxRetries; attempt++ {
		var client *api.Client
		select {
		case client = <-pool:
		case <-ctx.Done():
			return ctx.Err()
		}

		// Pass the whole llmResp struct to execute function
		err := executeI2VWorkflow(ctx, client, task, llmResp)

		pool <- client

		if err == nil {
			return nil
		}

		log.Printf("    ⚠️ Attempt %d failed on %s: %v", attempt+1, client.Origin, err)

		if attempt == maxRetries {
			return fmt.Errorf("max retries exceeded: %w", err)
		}

		sleepDuration := time.Duration(float64(baseDelay) * math.Pow(2, float64(attempt)))
		select {
		case <-time.After(sleepDuration):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func executeI2VWorkflow(ctx context.Context, client *api.Client, task i2vTask, llmResp *I2VResponse) error {
	graph, err := api.NewGraph(client, flagWorkflow)
	if err != nil {
		return err
	}

	absPath, err := filepath.Abs(task.imagePath)
	if err != nil {
		return err
	}

	seed := api.RandSeed()
	processedVars := make([]string, len(flagVars))
	for i, v := range flagVars {
		v = strings.ReplaceAll(v, "%image%", absPath)
		v = strings.ReplaceAll(v, "%prompt%", llmResp.Prompt) // Use the detailed prompt
		processedVars[i] = v
	}

	if len(processedVars) > 0 {
		if err := api.SetGraphNodeWeightValues(graph, processedVars, seed); err != nil {
			return err
		}
	}

	if err := client.PrepareGraph(graph); err != nil {
		return fmt.Errorf("prepare graph: %w", err)
	}

	outputs, err := client.RunWorkflow(ctx, graph)
	if err != nil {
		return err
	}

	// Pass ShortDescription as the 3rd argument (prefix)
	return outputs.SaveAll(flagOutput, flagForce, llmResp.ShortDescriptionZh)
}

func retry(attempts int, fn func() error) error {
	var err error
	for attempt := range attempts {
		if err = fn(); err == nil {
			return nil
		}
		time.Sleep(util.CalculateBackoff(2*time.Second, 60*time.Second, attempt))
	}
	return err
}
