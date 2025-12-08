package chat

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"unicode/utf8"

	"github.com/c-bata/go-prompt"
	"github.com/invopop/jsonschema"
	jsonschemaValidator "github.com/kaptinlin/jsonschema"
	"github.com/natefinch/atomic"
	"github.com/spf13/cobra"
	"github.com/vincent-petithory/dataurl"
	"golang.org/x/term"

	"github.com/sagan/goaider/cmd"
	"github.com/sagan/goaider/config"
	"github.com/sagan/goaider/constants"
	"github.com/sagan/goaider/features/clipboard"
	"github.com/sagan/goaider/features/llm"
	"github.com/sagan/goaider/util"
)

var chatCmd = &cobra.Command{
	Use:   "chat [prompt]",
	Args:  cobra.MaximumNArgs(1),
	Short: "Chat with LLM",
	Long: `Chat with LLM..

Examples:
  goaider chat "Tell me a joke"                 # Simple chat
  goaider chat -i foo.png "Describe the photo"  # use file as part of prompt

By default it outputs to stdout. Use --output flag to output to file.

Running "goaider chat" without providing any input will open a simple interactive shell`,
	RunE: doChat,
}

var (
	flagForce       bool
	flagAutoCopy    bool // auto copy LLM response text to clipboard
	flagTemperature float64
	flagOutput      string // output file, "-" for stdout (default)
	flagModel       string
	flagModelKey    string
	flagSchema      string   // response json schema file
	flagInputs      []string // input file
)

func init() {
	chatCmd.Flags().BoolVarP(&flagForce, "force", "", false, "Force overwriting without confirmation")
	chatCmd.Flags().BoolVarP(&flagAutoCopy, "auto-copy", "C", false, `Auto copy LLM response text to clipboard. `+
		`It works on Windows only`)
	chatCmd.Flags().Float64VarP(&flagTemperature, "temperature", "T", 1.0,
		"The temperature to use for the model. Range 0.0-2.0. Lower is deterministic; Higher is creative")
	chatCmd.Flags().StringVarP(&flagOutput, "output", "o", "-", `Output file path. Use "-" for stdout`)
	chatCmd.Flags().StringVarP(&flagModel, "model", "", "", "The model to use. "+constants.HELP_MODEL)
	chatCmd.Flags().StringVarP(&flagModelKey, "model-key", "", "", constants.HELP_MODEL_KEY)
	chatCmd.Flags().StringVarP(&flagSchema, "schema", "", "",
		`Response JSON schema file. If provided, the LLM will be instructed to return JSON `+
			`that conforms to this schema. See https://json-schema.org/learn/miscellaneous-examples for examples`)
	chatCmd.Flags().StringArrayVarP(&flagInputs, "input", "i", nil,
		`Usen file as input. Use "-" for stdin. Can provide multiple input. Non-text file are used as attachment`)
	cmd.RootCmd.AddCommand(chatCmd)
}

func shellCompleter(d prompt.Document) []prompt.Suggest {
	s := []prompt.Suggest{}
	return prompt.FilterHasPrefix(s, d.GetWordBeforeCursor(), true)
}

func doChat(cmd *cobra.Command, args []string) (err error) {
	if flagModel == "" {
		flagModel = config.GetDefaultModel()
	}
	cmd.Printf("Use %q model\n", flagModel)
	argInput := ""
	if len(args) > 0 {
		argInput = args[0]
	}

	openaiReq := &llm.OpenAIChatRequest{
		Model:       flagModel,
		Temperature: flagTemperature,
	}

	if argInput == "" && len(flagInputs) == 0 {
		if !term.IsTerminal(int(os.Stdout.Fd())) {
			return fmt.Errorf("no input is provided and not in tty")
		}
		p := prompt.New(func(input string) {
			if input == "/clear" {
				openaiReq.Messages = nil
				fmt.Printf("<session cleared>\n")
				return
			}
			openaiReq.Messages = append(openaiReq.Messages, &llm.OpenAIMessage{
				Role:    "user",
				Content: input,
			})
			response := strings.Builder{}
			err := llm.Stream(flagModelKey, flagModel, openaiReq, func(content string) error {
				response.WriteString(content)
				fmt.Printf("%s", content)
				return nil
			})
			if err != nil {
				fmt.Printf("<error: %v>\n", err)
				openaiReq.Messages = openaiReq.Messages[0 : len(openaiReq.Messages)-1]
				return
			}
			openaiReq.Messages = append(openaiReq.Messages, &llm.OpenAIMessage{
				Role:    "assistant",
				Content: response,
			})
			if flagAutoCopy {
				clipboard.CopyString(response.String())
			}
			fmt.Printf("\n")
		}, shellCompleter, prompt.OptionTitle("goaider-chat"))
		// https://github.com/c-bata/go-prompt/issues/265
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
	var schemaValidator *jsonschemaValidator.Schema
	if flagSchema != "" {
		schemaBytes, err := os.ReadFile(flagSchema)
		if err != nil {
			return fmt.Errorf("failed to read schema file %q: %w", flagSchema, err)
		}
		compiler := jsonschemaValidator.NewCompiler()
		schemaValidator, err = compiler.Compile(schemaBytes)
		if err != nil {
			return fmt.Errorf("failed to validate schema %q: %w", flagSchema, err)
		}
		var schema *jsonschema.Schema
		if err := json.Unmarshal(schemaBytes, &schema); err != nil {
			return fmt.Errorf("failed to parse schema file %q: %w", flagSchema, err)
		}
		openaiReq.ResponseFormat = &llm.OpenAIResponseFormat{
			Type: "json_schema",
			JsonSchema: &llm.OpenAIJsonSchema{
				Name:   "response_schema",
				Schema: schema,
				Strict: true,
			},
		}
	}
	openaiReq.Messages = nil
	for _, inputFile := range flagInputs {
		var input io.Reader
		if inputFile == "-" {
			input = os.Stdin
		} else {
			f, err := os.Open(inputFile)
			if err != nil {
				return fmt.Errorf("failed to open input file %q: %w", inputFile, err)
			}
			defer f.Close()
			input = f
		}
		fileBytes, err := io.ReadAll(input)
		if err != nil {
			return fmt.Errorf("failed to read input file %q: %w", inputFile, err)
		}
		if utf8.Valid(fileBytes) {
			openaiReq.Messages = append(openaiReq.Messages, &llm.OpenAIMessage{
				Role:    "user",
				Content: string(fileBytes),
			})
		} else {
			openaiReq.Messages = append(openaiReq.Messages, &llm.OpenAIMessage{
				Role: "user",
				Content: []llm.OpenAIContentPart{{
					Type:     "image_url",
					ImageUrl: &llm.OpenAIImageUrl{Url: dataurl.EncodeBytes(fileBytes)}}},
			})
		}
	}
	if argInput != "" {
		openaiReq.Messages = append(openaiReq.Messages, &llm.OpenAIMessage{
			Role:    "user",
			Content: argInput,
		})
	}
	response := strings.Builder{}
	responseStr := ""
	reader, writer := io.Pipe()
	go func() {
		err := llm.Stream(flagModelKey, flagModel, openaiReq, func(content string) error {
			response.WriteString(content)
			writer.Write([]byte(content))
			return nil
		})
		if err != nil {
			writer.CloseWithError(err)
			return
		}
		responseStr = response.String()
		if schemaValidator != nil {
			if err := schemaValidator.Validate([]byte(responseStr)); err != nil {
				writer.CloseWithError(fmt.Errorf("LLM response does not conform to schema: %w", err))
			}
		}
	}()
	if flagOutput == "-" {
		_, err = io.Copy(os.Stdout, reader)
	} else {
		err = atomic.WriteFile(flagOutput, reader)
	}
	if err != nil {
		return err
	}
	if flagAutoCopy {
		clipboard.CopyString(responseStr)
	}
	return nil
}
