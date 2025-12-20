package translate

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"cloud.google.com/go/translate"
	"github.com/c-bata/go-prompt"
	"github.com/natefinch/atomic"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"golang.org/x/text/language"

	"github.com/sagan/goaider/cmd"
	"github.com/sagan/goaider/constants"
	"github.com/sagan/goaider/features/clipboard"
	"github.com/sagan/goaider/features/speaker"
	"github.com/sagan/goaider/features/translation"
	"github.com/sagan/goaider/util"
	"github.com/sagan/goaider/util/helper"
	"github.com/sagan/goaider/util/stringutil"
)

var translateCmd = &cobra.Command{
	Use:     "translate [text]",
	Aliases: []string{"tr", "trans"},
	Args:    cobra.MaximumNArgs(1),
	Short:   "Translate text using Google Cloud Translation API",
	Long: `Translate text using Google Cloud Translation API.

It requires GOOGLE_APPLICATION_CREDENTIALS env to be the path to Google Cloud API service account key.json file,
permissions required for the service account: Cloud Translation API User.

Usage:
- goaider translate "Bonjour" # translate text
- goaider translate -i foo.txt # read from file and translate
- goaider translate -i - # Read from stdin and translate

By default it outputs to stdout. Use --output flag to output to file.

Running "goaider translate" without providing any input will open a simple interactive shell
that translate each input line and output the result.`,
	RunE: doTranslate,
}

var (
	flagForce          bool
	flagLines          bool   // Used with --input file. Treat each line of the file as standalone input
	flagOutputPrefix   string // prefix string when generating output text
	flagOutputTemplate string // template string for generating output text
	flagAutoCopy       bool   // auto copy translated text to clipboard
	flagAutoCopyOnly   bool
	flagPlay           bool
	flagPlaySource     bool
	flagInput          string // input file
	flagTargetLang     string // Target lang. Any of: "ja", "fr", "ru", "es", "de", "en", "zh", "zh-cn", "zh-tw", "chs", "cht"
	flagSourceLang     string // source lang
	flagOutput         string // output file, "-" for stdout (default)
)

func init() {
	translateCmd.Flags().BoolVarP(&flagLines, "lines", "L", false,
		`Translate each line of source text as a separate input. Output will also be line by line`)
	translateCmd.Flags().BoolVarP(&flagForce, "force", "", false, "Force overwriting without confirmation")
	translateCmd.Flags().BoolVarP(&flagAutoCopy, "auto-copy", "c", false, `Auto copy translated text to clipboard. `+
		`It works on Windows only`)
	translateCmd.Flags().BoolVarP(&flagAutoCopyOnly, "auto-copy-only", "C", false,
		`Mute output and only copy translated text to clipboard. It works on Windows only`)
	translateCmd.Flags().BoolVarP(&flagPlay, "play", "p", false,
		`Play translated text as speech. It works on Windows only`)
	translateCmd.Flags().BoolVarP(&flagPlaySource, "play-source", "P", false,
		`Play source text as speech. It works on Windows only`)
	translateCmd.Flags().StringVarP(&flagTargetLang, "target", "t", "en",
		`Target language. Any of: `+constants.HELP_LANGS)
	translateCmd.Flags().StringVarP(&flagSourceLang, "source", "s", "auto",
		`Source language. Any of: "auto", `+constants.HELP_LANGS)
	translateCmd.Flags().StringVarP(&flagInput, "input", "i", "", `Read text from input file. Use "-" for stdin`)
	translateCmd.Flags().StringVarP(&flagOutput, "output", "o", "-", `Output file path. Use "-" for stdout`)
	translateCmd.Flags().StringVarP(&flagOutputPrefix, "output-prefix", "", "",
		`Prepend this prefix to translated text to generate response text`)
	translateCmd.Flags().StringVarP(&flagOutputTemplate, "output-template", "T", "",
		`Template to generate response text. `+
			`Context: {text: "translated text", original: "original text", target: "en", source: "ja"}, `+
			`where target / source is the language of translated / original text. `+constants.HELP_TEMPLATE_FLAG)
	cmd.RootCmd.AddCommand(translateCmd)
}

func shellCompleter(d prompt.Document) []prompt.Suggest {
	s := []prompt.Suggest{}
	return prompt.FilterHasPrefix(s, d.GetWordBeforeCursor(), true)
}

func doTranslate(cmd *cobra.Command, args []string) (err error) {
	if flagAutoCopyOnly {
		flagAutoCopy = true
	}
	if flagAutoCopy {
		clipboard.Init()
	}
	var targetLang, sourceLang language.Tag
	if tag, ok := translation.LanguageTags[flagTargetLang]; !ok {
		return fmt.Errorf("unsupported target lang %s", flagTargetLang)
	} else {
		targetLang = tag
	}
	if flagSourceLang != "" && flagSourceLang != "auto" {
		if tag, ok := translation.LanguageTags[flagSourceLang]; !ok {
			return fmt.Errorf("unsupported source lang %s", flagSourceLang)
		} else {
			sourceLang = tag
		}
	}
	argInput := ""
	if len(args) > 0 {
		argInput = args[0]
	}

	var outputTemplate *helper.Template
	if flagOutputTemplate != "" {
		outputTemplate, err = helper.GetTemplate(flagOutputTemplate, true)
		if err != nil {
			return fmt.Errorf("invalid copy template: %w", err)
		}
	}

	ctx := context.Background()
	// Google Cloud Console - Service Account - Key
	// Permissions required: Cloud Translation API User
	// set env:
	// export GOOGLE_APPLICATION_CREDENTIALS="/path/to/your-service-account-key.json"
	// os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", `C:\Users\root\.config\goaider\goaider_google_application_credentials.json`)

	// 1. Create a client
	// It automatically uses GOOGLE_APPLICATION_CREDENTIALS for auth
	client, err := translate.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer client.Close()

	if cnt := util.CountNonZeroVariables(flagInput, argInput); cnt > 1 {
		return fmt.Errorf("--file flag and [text] arg can NOT be both set")
	} else if cnt == 0 {
		if !term.IsTerminal(int(os.Stdout.Fd())) {
			return fmt.Errorf("no input is provided and not in tty")
		}
		p := prompt.New(func(input string) {
			if tag, ok := translation.LanguageTags[input]; ok {
				flagTargetLang = input
				targetLang = tag
				return
			}
			translatedText, detectedSource, err := translation.Trans(ctx, client, input, targetLang, sourceLang)
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to translate: %v", err)
			}
			response, err := render(outputTemplate, flagOutputPrefix, translatedText, input, flagTargetLang, detectedSource)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error generating response: %v", err)
				return
			}
			if flagAutoCopy {
				clipboard.CopyString(response)
			}
			if flagPlaySource || flagPlay {
				go func(sourceLang, source, lang, text string) {
					if flagPlaySource {
						speaker.Play(sourceLang, source)
					}
					if flagPlay {
						speaker.Play(lang, text)
					}
				}(detectedSource, input, flagTargetLang, translatedText)
			}
			fmt.Printf("%s\n", response)
		}, shellCompleter, prompt.OptionLivePrefix(func() (prefix string, useLivePrefix bool) {
			return fmt.Sprintf("(%s) > ", flagTargetLang), true
		}), prompt.OptionTitle("goaider-translate"))
		if runtime.GOOS != "windows" {
			defer exec.Command("reset").Run()
		}
		p.Run()
		return nil
	}

	if flagOutput != "-" {
		if exists, err := util.FileExists(flagOutput); err != nil || (exists && !flagForce) {
			return fmt.Errorf("output file %q exists or access failed. err: %w", flagOutput, err)
		}
	}

	inputText := ""
	if flagInput == "-" {
		if contents, err := io.ReadAll(cmd.InOrStdin()); err != nil {
			return fmt.Errorf("failed to read stdin: %w", err)
		} else {
			inputText = string(contents)
		}
	} else if flagInput != "" {
		if contents, err := os.ReadFile(flagInput); err != nil {
			return fmt.Errorf("failed to read file %q: %w", flagInput, err)
		} else {
			inputText = string(contents)
		}
	} else {
		inputText = argInput
	}

	if flagLines {
		lines := stringutil.SplitLines(inputText)
		translatedLines, detectedSources, err := translation.TransBatch(ctx, client, lines, targetLang, sourceLang)
		if err != nil {
			return err
		}
		var finalOutput strings.Builder
		for i, line := range translatedLines {
			originalLine := lines[i]
			detectedSource := detectedSources[i]
			response, err := render(outputTemplate, flagOutputPrefix, line, originalLine, flagTargetLang, detectedSource)
			if err != nil {
				return fmt.Errorf("error generating response for line %d: %w", i+1, err)
			}
			finalOutput.WriteString(response)
			finalOutput.WriteString("\n")
		}
		if flagAutoCopy {
			clipboard.CopyString(finalOutput.String())
		}
		if !flagAutoCopyOnly {
			if flagOutput == "-" {
				_, err = fmt.Print(finalOutput.String())
			} else {
				err = atomic.WriteFile(flagOutput, strings.NewReader(finalOutput.String()))
			}
			if err != nil {
				return err
			}
		}
		if flagPlay || flagPlaySource {
			for i := range lines {
				if flagPlaySource {
					speaker.Play(detectedSources[i], lines[i])
				}
				if flagPlay {
					speaker.Play(flagTargetLang, translatedLines[i])
				}
			}
		}
		return nil
	}

	inputText = strings.TrimSpace(inputText)
	if len(inputText) == 0 {
		log.Warnf("input is empty")
		return nil
	}

	translatedText, detectedSource, err := translation.Trans(ctx, client, inputText, targetLang, sourceLang)
	if err != nil {
		return fmt.Errorf("failed to translate: %w", err)
	}
	response, err := render(outputTemplate, flagOutputPrefix, translatedText, inputText, flagTargetLang, detectedSource)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error generating response: %v", err)
		return
	}
	if flagAutoCopy {
		clipboard.CopyString(response)
	}
	if !flagAutoCopyOnly {
		if flagOutput == "-" {
			_, err = fmt.Print(response)
		} else {
			err = atomic.WriteFile(flagOutput, strings.NewReader(translatedText))
		}
		if err != nil {
			return err
		}
	}
	if flagPlaySource {
		speaker.Play(detectedSource, inputText)
	}
	if flagPlay {
		speaker.Play(flagTargetLang, translatedText)
	}

	return nil
}

// render generated the response from translated text, optionally with a prefix or using a template.
// It returns an error if template execution fails.
func render(tpl *helper.Template, prefix, text, original, target, source string) (response string, err error) {
	if tpl != nil {
		data := map[string]string{
			"text":     text,
			"original": original,
			"target":   target,
			"source":   source,
		}
		response, err = tpl.Exec(data)
		if err != nil {
			return "", fmt.Errorf("failed to execute copy template: %w", err)
		}
	} else {
		response = text
	}
	return strings.TrimSpace(prefix + response), nil
}
