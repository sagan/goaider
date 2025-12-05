package caption

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/sagan/goaider/cmd"
	"github.com/sagan/goaider/constants"
	"github.com/sagan/goaider/features/llm"
	"github.com/sagan/goaider/util"
)

// --- API and Program Constants ---

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
"
`

	maxRetries = 3 // Number of retries for API calls
)

// Flag variables to store command line arguments
var (
	flagForce    bool
	flagIdentity string
	flagModel    string
)

var captionCmd = &cobra.Command{
	Use:   "caption <dir>",
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
	captionCmd.Flags().StringVarP(&flagIdentity, "identity", "", "",
		"Optional: The trigger word (e.g., 'foobar' or 'photo of foobar') to prepend to each caption")
	captionCmd.Flags().StringVarP(&flagModel, "model", "", constants.DEFAULT_GEMINI_MODEL,
		"The model to use for captioning")
}

func caption(cmd *cobra.Command, args []string) error {
	argDir := args[0]

	// 1. Get API Key from environment
	apiKey := os.Getenv(constants.ENV_GEMINI_API_KEY)
	if apiKey == "" {
		return fmt.Errorf("GEMINI_API_KEY environment variable not set")
	}

	// 3. Read the specified directory
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
	// 4. Loop over all files and process images
	for _, file := range files {
		if file.IsDir() || !isImageFile(file.Name()) {
			continue // Skip directories and non-image files
		}

		fullPath := filepath.Join(argDir, file.Name())

		// processImage does all the work: API call, retries, and file saving
		err := processImage(fullPath, apiKey, flagForce, flagIdentity)
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
 * 3. Encodes it to base64
 * 4. Calls the Gemini API (with retries)
 * 5. Parses the response
 * 6. Prepends identity (if provided)
 * 7. Saves the caption to a .txt file
 */
func processImage(imagePath string, apiKey string, force bool, identity string) (err error) {
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

	// 2. Read image file and encode to base64
	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		return fmt.Errorf("failed to read image: %w", err)
	}
	base64Image := base64.StdEncoding.EncodeToString(imageData)
	mimeType := util.GetMimeType(imagePath)

	// 3. Construct the API request payload
	payload := &llm.GeminiRequest{
		Contents: []llm.Content{
			{
				Role: "user",
				Parts: []llm.Part{
					{Text: captionPrompt}, // The prompt to the model
					{
						InlineData: &llm.InlineData{ // The image data
							MimeType: mimeType,
							Data:     base64Image,
						},
					},
				},
			},
		},
	}

	var geminiRes *llm.GeminiResponse
	for retries := range maxRetries {
		geminiRes, err = llm.Gemini(apiKey, flagModel, payload)
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
	caption := geminiRes.Candidates[0].Content.Parts[0].Text
	finalCaption := strings.TrimSpace(caption) // Clean up any extra whitespace
	if identity != "" {
		finalCaption = identity + ", " + finalCaption
	}

	// 7. Save the caption to a .txt file
	err = os.WriteFile(txtPath, []byte(finalCaption), 0644)
	if err != nil {
		return fmt.Errorf("failed to write caption file: %w", err)
	}

	fmt.Printf("Processing %s: ✅ SUCCESS\n", baseName)
	return nil
}

// isImageFile checks if a filename has a common image extension
func isImageFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp":
		return true
	default:
		return false
	}
}
