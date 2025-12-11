package sovitsgenlist

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/natefinch/atomic"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/goaider/cmd"
	"github.com/sagan/goaider/util"
	"github.com/sagan/goaider/util/stringutil"
)

var (
	flagForce   bool
	flagLang    string
	flagSpeaker string
	flagOutput  string
	flagExt     string
)

var genlistCmd = &cobra.Command{
	Use:   "sovits-genlist {dir}",
	Args:  cobra.ExactArgs(1),
	Short: "Generates a GPT-SoVITS dataset annotation dataset .list file",
	Long: `Generates a GPT-SoVITS dataset annotation dataset .list file.

The sovits-genlist command generates a dataset annotation dataset .list file
used by GPT-SoVITS (a voice synthesis and cloning model).

It reads all "<filename>.wav" files and corresponding "<filename>.txt"
transcription files from a specified directory, then generates a
dataset .list file to output file, which defaults to stdout.

Each line in the generated .list file will have the format:
  audio_filename|speaker|language|text

Example:
  clannad.wav|speaker|ja|お連れしましょうか。この街の、願いが叶う場所へ。

Notes:
- Only include a wav file record in sovits.list file if a corresponding .txt
  transcription file exists.
- If a .txt file has multiple lines, it replace newline characters ([\r\n]+)
  with a single space.`,
	RunE: runSovitsGenlist,
}

func init() {
	genlistCmd.Flags().StringVarP(&flagOutput, "output", "", "-", `Output filename. Use "-" to output to stdout`)
	genlistCmd.Flags().StringVarP(&flagLang, "lang", "", "ja",
		"The language spoken in the audio files: zh | ja | en | ko | yue")
	genlistCmd.Flags().BoolVarP(&flagForce, "force", "", false, `Force overwrite output file even if it already exists`)
	genlistCmd.Flags().StringVarP(&flagSpeaker, "speaker", "", "speaker", "Speaker name")
	genlistCmd.Flags().StringVarP(&flagExt, "ext-transcript", "", ".txt", `Transcript file extension`)
	cmd.RootCmd.AddCommand(genlistCmd)
}

func runSovitsGenlist(cmd *cobra.Command, args []string) (err error) {
	if flagOutput != "-" {
		if exists, err := util.FileExists(flagOutput); err != nil || (exists && !flagForce) {
			return fmt.Errorf("output file %q already exists or access error. err: %w", flagOutput, err)
		}
	}
	if !strings.HasPrefix(flagExt, ".") {
		flagExt = "." + flagExt
	}
	argDir := args[0]
	// Validate language
	validLangs := map[string]bool{"zh": true, "ja": true, "en": true, "ko": true, "yue": true}
	if !validLangs[flagLang] {
		return fmt.Errorf("invalid language: %q. Must be one of: zh, ja, en, ko, yue", flagLang)
	}

	// Get absolute path for the directory
	absDirPath, err := filepath.Abs(argDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for directory %q: %w", argDir, err)
	}

	// Read directory contents
	entries, err := os.ReadDir(absDirPath)
	if err != nil {
		return fmt.Errorf("failed to read directory %q: %w", absDirPath, err)
	}

	var listLines []string
	wavFiles := make(map[string]struct{}) // To keep track of found wav files

	// First pass: collect all .wav files
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".wav") {
			baseName := strings.TrimSuffix(entry.Name(), ".wav")
			wavFiles[baseName] = struct{}{}
		}
	}

	// Second pass: process .txt files that have corresponding .wav files
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), flagExt) {
			baseName := strings.TrimSuffix(entry.Name(), flagExt)

			if _, exists := wavFiles[baseName]; exists {
				txtFilePath := filepath.Join(absDirPath, entry.Name())
				content, err := os.ReadFile(txtFilePath)
				if err != nil {
					log.Printf("Warning: Failed to read transcription file %q: %v. Skipping.", txtFilePath, err)
					continue
				}

				text := stringutil.ReplaceNewLinesWithSpace(string(content))
				text = strings.TrimSpace(text)

				// Format the line
				line := fmt.Sprintf("%s.wav|%s|%s|%s", baseName, flagSpeaker, flagLang, text)
				listLines = append(listLines, line)
			}
		}
	}

	if len(listLines) == 0 {
		return fmt.Errorf("no valid wav files found")
	}

	reader, writer := io.Pipe()
	go func() {
		for _, line := range listLines {
			_, err := writer.Write([]byte(line + "\n"))
			if err != nil {
				writer.CloseWithError(fmt.Errorf("failed to write line to output: %w", err))
				return
			}
		}
		writer.Close()
	}()
	if flagOutput == "-" {
		_, err = io.Copy(cmd.OutOrStdout(), reader)
	} else {
		err = atomic.WriteFile(flagOutput, reader)
	}
	if err != nil {
		return fmt.Errorf("failed to output to %q : %w", flagOutput, err)
	}
	return nil
}
