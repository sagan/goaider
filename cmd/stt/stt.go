package stt

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/goaider/cmd"
	"github.com/sagan/goaider/config"
	"github.com/sagan/goaider/constants"
	"github.com/sagan/goaider/features/llm"
	"github.com/sagan/goaider/util"
)

var (
	flagForce       bool
	flagTemperature float64
	flagModel       string
	flagModelKey    string
)

// sttCmd represents the stt command
var sttCmd = &cobra.Command{
	Use:   "stt {dir}",
	Args:  cobra.ExactArgs(1),
	Short: "Generates speech-to-text transcripts for audio files",
	Long: `Generates speech-to-text transcripts for audio files.

It uses LLM to process a directory of audio files (.wav, .mp3, .m4a, .flac, .ogg),
and generates a corresponding .txt file for each one using the Google Gemini API.

Implements exponential backoff to handle rate limiting (e.g., 10 RPM).

Requires the GEMINI_API_KEY environment variable to be set.`,
	// This is the main function that runs when the command is called
	RunE: stt,
}

func init() {
	cmd.RootCmd.AddCommand(sttCmd)
	sttCmd.Flags().BoolVarP(&flagForce, "force", "", false, "Overwrite existing .txt transcript files")
	sttCmd.Flags().Float64VarP(&flagTemperature, "temperature", "T", 0.4, constants.HELP_TEMPERATURE_FLAG)
	sttCmd.Flags().StringVarP(&flagModel, "model", "", "", "The model to use. "+constants.HELP_MODEL)
	sttCmd.Flags().StringVarP(&flagModelKey, "model-key", "", "", constants.HELP_MODEL_KEY)
}

func stt(cmd *cobra.Command, args []string) error {
	if flagModel == "" {
		flagModel = config.GetDefaultModel()
	}
	argDir := args[0]
	log.Printf("Using model: %s", flagModel)

	// Read all files in the directory
	files, err := os.ReadDir(argDir)
	if err != nil {
		return fmt.Errorf("error reading directory %q: %w", argDir, err)
	}

	errorCnt := 0
	for _, file := range files {
		if file.IsDir() {
			continue // Skip subdirectories
		}

		fileName := file.Name()
		fileExt := filepath.Ext(fileName)
		mimeType := util.GetMimeType(fileExt)

		if !strings.HasPrefix(mimeType, "audio/") {
			// log.Printf("Skipping non-audio file: %s", fileName)
			continue
		}

		// Define input and output paths
		audioFilePath := filepath.Join(argDir, fileName)
		outputTxtPath := strings.TrimSuffix(audioFilePath, fileExt) + ".txt"

		// Check if output file exists
		if !flagForce {
			if _, err := os.Stat(outputTxtPath); err == nil {
				log.Printf("Skipping (exists): %s", fileName)
				continue
			}
		}

		// Process the file
		log.Printf("Processing: %s", fileName)

		// 1. Read audio file
		audioData, err := os.ReadFile(audioFilePath)
		if err != nil {
			log.Printf("Error reading audio file %s: %v", fileName, err)
			errorCnt++
			continue
		}

		// 2. Call Gemini API
		transcript, err := getTranscript(flagModelKey, flagModel, audioData, mimeType)
		if err != nil {
			log.Printf("Error generating transcript for %s: %v", fileName, err)
			errorCnt++
			continue
		}

		// 3. Write transcript to .txt file
		err = os.WriteFile(outputTxtPath, []byte(transcript), 0644)
		if err != nil {
			log.Printf("Error writing transcript file %s: %v", outputTxtPath, err)
			errorCnt++
			continue
		}

		log.Printf("Generated: %s", filepath.Base(outputTxtPath))
	}

	log.Printf("Processing complete.")
	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}

const PROMPT = "Generate a transcript of this audio. Only output the transcribed text in it's original language."

// getTranscript calls the Gemini API with retry logic
func getTranscript(apiKey, modelName string, audioData []byte, mimeType string) (transcript string, err error) {
	for attempt := range 5 {
		transcript, err = llm.ImageToText(apiKey, modelName, PROMPT, audioData, mimeType, flagTemperature)
		if err == nil {
			return transcript, nil
		}
		if util.IsTemporaryError(err) {
			wait := util.CalculateBackoff(llm.GeminiApiBaseBackoff, llm.GeminiApiMaxBackoff, attempt)
			log.Printf("Attempt %d: error (%v). Retrying in %v...", attempt, err, wait)
			time.Sleep(wait)
			continue
		} else {
			return "", err
		}
	}
	return "", fmt.Errorf("all attempts failed. Last error: %w", err)
}
