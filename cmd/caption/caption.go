package caption

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/sagan/goaider/cmd"
	"github.com/sagan/goaider/config"
	"github.com/sagan/goaider/constants"
	"github.com/sagan/goaider/features/llm"
	"github.com/sagan/goaider/util"
)

const (
	// This prompt is optimized for LoRa training captions
	captionPrompt = `Generate a simple, comma-separated caption for this image, optimized for LoRa training.

RULES:
1.  Focus ONLY on the main subject.
2.  Describe visible attributes: clothing (e.g., "pink jacket"), hairstyle (e.g., "ponytail"), pose (e.g., "crouching", "standing"), and expression (e.g., "smiling").
3.  You can describe an object that the main subject is interacting with (e.g., "holding a toy").

CRITICAL:
* DO NOT use general category words like "girl", "boy", "child", "woman", "man", or "person".
* DO NOT describe the background, environment, or location (e.g., AVOID "in a room", "child's room", "indoor", "outside", "at home").
* DO NOT describe artistic style, lighting, camera quality, or effects.

Good example: "pink puffer jacket, ponytail, hair clips, crouching, holding toy".

Bad example: "young girl, pink puffer jacket, fur collar, black pants, slippers, pink bunny hair clips, ponytail, pink bobbles, crouching, holding a pink plastic toy, child's room, pink desk, pink chair, toys, curtains, wooden floor".
`

	maxRetries = 3
)

var (
	flagForce       bool
	flagTemperature float64
	flagIdentity    string
	flagModel       string
	flagModelKey    string
)

var captionCmd = &cobra.Command{
	Use:   "caption {dir}",
	Short: "Generate captions for images in a directory",
	Long: `This command generates captions for all images in a specified directory using the Gemini API.

It requires GEMINI_API_KEY env.

It saves the caption of each image file to <filename>.txt file of the same dir.`,
	Args: cobra.ExactArgs(1),
	RunE: caption,
}

func init() {
	cmd.RootCmd.AddCommand(captionCmd)
	captionCmd.Flags().BoolVarP(&flagForce, "force", "", false,
		"Optional: Force re-generation of all captions, even if .txt files exist")
	captionCmd.Flags().Float64VarP(&flagTemperature, "temperature", "T", 0.4, constants.HELP_TEMPERATURE_FLAG)
	captionCmd.Flags().StringVarP(&flagIdentity, "identity", "", "",
		"Optional: The trigger word (e.g., 'foobar' or 'photo of foobar') to prepend to each caption")
	captionCmd.Flags().StringVarP(&flagModel, "model", "", "", "The model to use. "+constants.HELP_MODEL)
	captionCmd.Flags().StringVarP(&flagModelKey, "model-key", "", "", constants.HELP_MODEL_KEY)
}

func caption(cmd *cobra.Command, args []string) error {
	if flagModel == "" {
		flagModel = config.GetDefaultModel()
	}
	argDir := args[0]

	files, err := os.ReadDir(argDir)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", argDir, err)
	}

	fmt.Printf("Starting captioning for images in: %s\n", argDir)
	if flagForce {
		fmt.Printf("FORCE flag set: Re-generating all captions.\n")
	}
	if flagIdentity != "" {
		fmt.Printf("IDENTITY set: Prepending %q to all new captions.\n", flagIdentity)
	}

	errorCnt := 0
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		mimeType := util.GetMimeType(file.Name())
		if !strings.HasPrefix(mimeType, "image/") {
			continue
		}
		fullPath := filepath.Join(argDir, file.Name())
		err := processImage(fullPath, mimeType, flagTemperature, flagModelKey, flagForce, flagIdentity)
		if err != nil {
			fmt.Printf("Processing %s: ❌ FAILED (%v)\n", file.Name(), err)
			errorCnt++
		}
	}
	fmt.Printf("Captioning complete.\n")
	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}

/**
 * processImage handles the full logic for a single image:
 * 1. Checks if caption file exists (and skips if -force is not set)
 * 2. Reads the image file
 * 3. Calls the LLM API (with retries)
 * 4. Prepends identity (if provided)
 * 5. Saves the caption to a .txt file
 */
func processImage(imagePath string, mimeType string, temperature float64,
	apiKey string, force bool, identity string) (err error) {
	// 1. Check for existing .txt file before doing any work
	baseName := filepath.Base(imagePath)
	ext := filepath.Ext(baseName)
	txtFileName := strings.TrimSuffix(baseName, ext) + ".txt"
	txtPath := filepath.Join(filepath.Dir(imagePath), txtFileName)

	if !force {
		if _, err := os.Stat(txtPath); err == nil {
			// File exists, skip processing
			fmt.Printf("Processing %s: ⏩ SKIPPED (caption already exists)\n", baseName)
			return nil
		}
	}

	fmt.Printf("Processing %s: ⏳ GENERATING...\n", baseName)
	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		return fmt.Errorf("failed to read image: %w", err)
	}

	var caption string
	for retries := range maxRetries {
		caption, err = llm.ImageToText(apiKey, flagModel, captionPrompt, imageData, mimeType, temperature)
		if err == nil {
			break
		}
		if util.IsTemporaryError(err) {
			wait := util.CalculateBackoff(llm.GeminiApiBaseBackoff, llm.GeminiApiMaxBackoff, retries)
			fmt.Printf("  .. error (%v), retrying in %v\n", err, wait)
			time.Sleep(wait)
			continue
		} else {
			break
		}
	}
	if err != nil {
		return err
	}

	finalCaption := caption
	if identity != "" {
		finalCaption = identity + ", " + finalCaption
	}
	err = os.WriteFile(txtPath, []byte(finalCaption), 0644)
	if err != nil {
		return fmt.Errorf("failed to write caption file: %w", err)
	}

	fmt.Printf("Processing %s: ✅ SUCCESS\n", baseName)
	return nil
}
