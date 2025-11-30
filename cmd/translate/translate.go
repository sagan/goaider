package translate

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"cloud.google.com/go/translate"
	"github.com/c-bata/go-prompt"
	"github.com/natefinch/atomic"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"golang.org/x/text/language"

	"github.com/sagan/goaider/cmd"
	"github.com/sagan/goaider/features/clipboard"
	"github.com/sagan/goaider/features/translation"
	"github.com/sagan/goaider/util"
)

var translateCmd = &cobra.Command{
	Use:     "translate [text]",
	Aliases: []string{"tr", "trans"},
	Args:    cobra.MaximumNArgs(1),
	Short:   "Translate text using Google Cloud Translation API",
	Long: `Translate text using Google Cloud Translation API.

It requires GOOGLE_APPLICATION_CREDENTIALS env to be the path to Google Cloud API service account key.json file,
permissions required: Cloud Translation API User .

Usage:
- goaider translate "Bonjour" # translate text
- goaider translate - # Read from stdin and translate
- goaider translate --input foo.txt # read from file and translate

By default it outputs to stdout. Use --output flag to output to file.

Running "goaider translate" without providing any input will open a simple translation shell.`,
	// This is the main function that runs when the command is called
	RunE: doTranslate,
}

var (
	flagForce      bool
	flagAutoCopy   bool   // auto copy translated text to clipboard
	flagInput      string // input file
	flagTargetLang string // Target lang. Any of: "ja", "fr", "ru", "es", "de", "en", "zh", "zh-cn", "zh-tw", "chs", "cht"
	flagSourceLang string // source lang
	flagOutput     string // output file, "-" for stdout (default)
)

func init() {
	translateCmd.Flags().BoolVarP(&flagForce, "force", "", false, "Force overwriting without confirmation")
	translateCmd.Flags().BoolVarP(&flagAutoCopy, "auto-copy", "", false, `Auto copy translated text to clipboard. `+
		`It works on Windows only`)
	translateCmd.Flags().StringVarP(&flagTargetLang, "target", "t", "en",
		`Target language. Any of: "ja", "fr", "ru", "es", "de", "en", "zh", "zh-cn", "zh-tw", "chs", "cht"`)
	translateCmd.Flags().StringVarP(&flagSourceLang, "source", "s", "auto",
		`Source language. Any of: "auto", "ja", "fr", "ru", "es", "de", "en", "zh", "zh-cn", "zh-tw", "chs", "cht"`)
	translateCmd.Flags().StringVarP(&flagInput, "input", "i", "", `Read text from input file. Use "-" for stdin`)
	translateCmd.Flags().StringVarP(&flagOutput, "output", "o", "-", `Output file path. Use "-" for stdout`)
	cmd.RootCmd.AddCommand(translateCmd)
}

func shellCompleter(d prompt.Document) []prompt.Suggest {
	s := []prompt.Suggest{}
	return prompt.FilterHasPrefix(s, d.GetWordBeforeCursor(), true)
}

func doTranslate(cmd *cobra.Command, args []string) (err error) {
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
		p := prompt.New(func(s string) {
			if tag, ok := translation.LanguageTags[s]; ok {
				flagTargetLang = s
				targetLang = tag
				return
			}
			translatedText, err := translation.Trans(ctx, client, s, targetLang, sourceLang)
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to translate: %v", err)
			}
			if flagAutoCopy {
				clipboard.CopyString(translatedText)
			}
			fmt.Printf("%s\n", translatedText)
		}, shellCompleter, prompt.OptionLivePrefix(func() (prefix string, useLivePrefix bool) {
			return fmt.Sprintf("(%s) > ", flagTargetLang), true
		}), prompt.OptionTitle("goaider-translate"))
		p.Run()
		return nil
	}

	if flagOutput != "" {
		if exists, err := util.FileExists(flagOutput); err != nil || (exists && !flagForce) {
			return fmt.Errorf("output file %q exists or access failed. err: %w", flagOutput, err)
		}
	}

	inputText := ""
	if argInput == "-" || flagInput == "-" {
		if contents, err := io.ReadAll(os.Stdin); err != nil {
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
	inputText = strings.TrimSpace(inputText)
	if len(inputText) == 0 {
		log.Warnf("input is empty")
		return nil
	}

	translatedText, err := translation.Trans(ctx, client, inputText, targetLang, sourceLang)
	if err != nil {
		return fmt.Errorf("failed to translate: %w", err)
	}
	if flagAutoCopy {
		clipboard.CopyString(translatedText)
	}
	if flagOutput == "-" {
		fmt.Printf("%s\n", translatedText)
	} else {
		err = atomic.WriteFile(flagOutput, strings.NewReader(translatedText))
		if err != nil {
			return err
		}
	}

	return nil
}
