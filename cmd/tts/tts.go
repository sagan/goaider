package tts

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/natefinch/atomic"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/sagan/goaider/cmd"
	"github.com/sagan/goaider/constants"
	"github.com/sagan/goaider/features/speaker"
	"github.com/sagan/goaider/features/translation"
	"github.com/sagan/goaider/util"
	"github.com/sagan/goaider/util/stringutil"
)

var ttsCmd = &cobra.Command{
	Use:     "tts [text]",
	Aliases: []string{"say", "speak"},
	Short:   "Convert text to speech and play audio. Windows only",
	Long: `Convert text to speech and play audio. Windows only.

It uses Google Translate public TTS api, e.g. :
  http://translate.google.com/translate_tts?ie=UTF-8&q=bonjour&client=t&tl=fr .

It caches generated audio files in CACHE_DIR/` + speaker.CACHE_FOLDER_NAME + ` . Where CACHE_DIR is:
- Linux: $XDG_CACHE_HOME or $HOME/.cache .
- Darwin: $HOME/Library/Caches .
- Windows: %LocalAppData% .

To clean cache, run with "--clean" flag.
`,
	RunE: doTts,
	Args: cobra.MaximumNArgs(1),
}

var (
	flagForce  bool
	flagClean  bool
	flagLang   string
	flagInput  string
	flagOutput string
)

func init() {
	cmd.RootCmd.AddCommand(ttsCmd)
	ttsCmd.Flags().BoolVarP(&flagClean, "clean", "c", false, "Clean up all cached audio files and exit")
	ttsCmd.Flags().BoolVarP(&flagForce, "force", "", false, "Force overwriting existing file")
	ttsCmd.Flags().StringVarP(&flagLang, "lang", "l", "en", `Language to use for TTS. Any of: `+constants.HELP_LANGS)
	ttsCmd.Flags().StringVarP(&flagInput, "input", "i", "", `Optional: read text from file instead. Use "-" for stdin`)
	ttsCmd.Flags().StringVarP(&flagOutput, "output", "o", "", `Optional: save generated speech audio (mp3) to file`)
}

func doTts(cmd *cobra.Command, args []string) error {
	if flagOutput != "" && flagOutput != "-" {
		if exists, err := util.FileExists(flagOutput); err != nil || (exists && !flagForce) {
			return fmt.Errorf("output file %q exists or access failed. err: %w", flagOutput, err)
		}
	}
	if flagOutput == "-" && term.IsTerminal(int(os.Stdout.Fd())) {
		return fmt.Errorf("output is tty. Use pipe, file path or omit --output to play directly")
	}
	if _, exists := translation.LanguageTags[flagLang]; !exists {
		return fmt.Errorf("unsupported lang %s", flagLang)
	}

	if flagClean {
		return speaker.CleanCacheDir()
	}

	var input io.Reader
	if flagInput != "" && len(args) > 0 {
		return fmt.Errorf("--input flag and {text} arg are not compatible")
	}
	if len(args) > 0 {
		input = strings.NewReader(args[0])
	} else if flagInput == "-" {
		input = cmd.InOrStdin()
	} else if flagInput != "" {
		f, err := os.Open(flagInput)
		if err != nil {
			return err
		}
		defer f.Close()
		input = f
	} else {
		return fmt.Errorf("no input")
	}
	input = stringutil.GetTextReader(input)
	data, err := io.ReadAll(input)
	if err != nil {
		return err
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return nil
	}

	speaker, err := speaker.GetSpeaker(flagLang)
	if err != nil {
		return err
	}
	filename, err := speaker.GenerateAndSpeak(text)
	if err != nil {
		return err
	}

	if flagOutput != "" {
		file, err := os.Open(filename)
		if err != nil {
			return err
		}
		defer file.Close()
		if flagOutput == "-" {
			_, err = io.Copy(cmd.OutOrStdout(), file)
		} else {
			err = atomic.WriteFile(flagOutput, file)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
