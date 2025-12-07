package llm

import (
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/sagan/goaider/util"
)

const (
	OPENROUTER_API_URL             = "https://openrouter.ai/api/v1"
	OPENROUTER_MODEL_PREFIX        = "openrouter/" // OpenRouter model prefix
	OPENAI_MODEL_PREFIX            = "gpt-"        // OpenAI model prefix, ignore some uncommon ones like "o3".
	GEMINI_MODEL_PREFIX            = "gemini-"     // Gemini model prefix
	OPENAI_COMPATIBLE_MODEL_PREFIX = "openai/"     // Customary OpenAI API compatible model prefix
)

// error returned by online llm service provider
type ApiError struct {
	Message   string
	Body      string
	Status    int   // http status code
	Err       error // wrapped error
	Retryable bool  // busines logic defined retryable
}

func (a *ApiError) Error() string {
	return fmt.Sprintf("status=%d: %s", a.Status, a.Message)
}

func (a *ApiError) Temporary() bool {
	return a.Retryable || a.Status == http.StatusTooManyRequests ||
		a.Status == http.StatusInternalServerError ||
		a.Status == http.StatusBadGateway ||
		a.Status == http.StatusServiceUnavailable ||
		a.Status == http.StatusGatewayTimeout ||
		(a.Err != nil && util.IsTemporaryError(a.Err))
}

func (a *ApiError) Unwrap() error {
	return a.Err
}

// Wrapper of openai & gemini
func ImageToJson[T any](apiKey string, model string, prompt string, imageBytes []byte, mimeType string) (*T, error) {
	if strings.HasPrefix(model, GEMINI_MODEL_PREFIX) {
		return GeminiImageToJson[T](apiKey, model, prompt, imageBytes, mimeType)
	} else if isOpenAiModel(model) {
		return OpenAIImageToJson[T](OPENAI_API_URL, apiKey, model, prompt, imageBytes, mimeType)
	} else if openrouterModel, ok := strings.CutPrefix(model, OPENROUTER_MODEL_PREFIX); ok {
		if !strings.ContainsRune(openrouterModel, '/') {
			openrouterModel = OPENROUTER_MODEL_PREFIX + openrouterModel
		}
		return OpenAIImageToJson[T](OPENROUTER_API_URL, apiKey, openrouterModel, prompt, imageBytes, mimeType)
	} else if strings.HasPrefix(model, OPENAI_COMPATIBLE_MODEL_PREFIX) { // "openai/model-name/http://localhost:8080/v1"
		parts := strings.SplitN(model, "/", 3)
		if len(parts) == 3 {
			return OpenAIImageToJson[T](parts[2], apiKey, parts[1], prompt, imageBytes, mimeType)
		}
		return nil, fmt.Errorf("invalid openai model %s", model)
	}
	return nil, fmt.Errorf("unsupported model %s", model)
}

func ImageToText(apiKey string, model string, prompt string, imageBytes []byte, mimeType string) (string, error) {
	if strings.HasPrefix(model, GEMINI_API_URL) {
		return GeminiImageToText(apiKey, model, prompt, imageBytes, mimeType)
	} else if isOpenAiModel(model) {
		return OpenAIImageToText(OPENAI_API_URL, apiKey, model, prompt, imageBytes, mimeType)
	} else if openrouterModel, ok := strings.CutPrefix(model, OPENROUTER_MODEL_PREFIX); ok {
		if !strings.ContainsRune(openrouterModel, '/') {
			openrouterModel = OPENROUTER_MODEL_PREFIX + openrouterModel
		}
		return OpenAIImageToText(OPENROUTER_API_URL, apiKey, openrouterModel, prompt, imageBytes, mimeType)
	} else if strings.HasPrefix(model, OPENAI_COMPATIBLE_MODEL_PREFIX) {
		parts := strings.SplitN(model, "/", 3)
		if len(parts) == 3 {
			return OpenAIImageToText(parts[2], apiKey, parts[1], prompt, imageBytes, mimeType)
		}
		return "", fmt.Errorf("invalid openai model %s", model)
	}
	return "", fmt.Errorf("unsupported model %s", model)
}

func ChatJsonResponse[T any](apiKey string, model string, prompt string) (*T, error) {
	if strings.HasPrefix(model, GEMINI_MODEL_PREFIX) {
		return GeminiJsonResponse[T](apiKey, model, prompt)
	} else if isOpenAiModel(model) {
		return OpenAIJsonResponse[T](OPENAI_API_URL, apiKey, model, prompt)
	} else if openrouterModel, ok := strings.CutPrefix(model, OPENROUTER_MODEL_PREFIX); ok {
		if !strings.ContainsRune(openrouterModel, '/') {
			openrouterModel = OPENROUTER_MODEL_PREFIX + openrouterModel
		}
		return OpenAIJsonResponse[T](OPENROUTER_API_URL, apiKey, openrouterModel, prompt)
	} else if strings.HasPrefix(model, OPENAI_COMPATIBLE_MODEL_PREFIX) {
		parts := strings.SplitN(model, "/", 3)
		if len(parts) == 3 {
			return OpenAIJsonResponse[T](parts[2], apiKey, parts[1], prompt)
		}
		return nil, fmt.Errorf("invalid openai model %s", model)
	}
	return nil, fmt.Errorf("unsupported model %s", model)
}

func Chat(apiKey string, model string, prompt string) (string, error) {
	if strings.HasPrefix(model, GEMINI_MODEL_PREFIX) {
		return GeminiChat(apiKey, model, prompt)
	} else if isOpenAiModel(model) {
		return OpenAIChat(OPENAI_API_URL, apiKey, model, prompt)
	} else if openrouterModel, ok := strings.CutPrefix(model, OPENROUTER_MODEL_PREFIX); ok {
		if !strings.ContainsRune(openrouterModel, '/') {
			openrouterModel = OPENROUTER_MODEL_PREFIX + openrouterModel
		}
		return OpenAIChat(OPENROUTER_API_URL, apiKey, openrouterModel, prompt)
	} else if strings.HasPrefix(model, OPENAI_COMPATIBLE_MODEL_PREFIX) {
		parts := strings.SplitN(model, "/", 3)
		if len(parts) == 3 {
			return OpenAIChat(parts[2], apiKey, parts[1], prompt)
		}
		return "", fmt.Errorf("invalid openai model %s", model)
	}
	return "", fmt.Errorf("unsupported model %s", model)
}

func Stream(apiKey string, model string, reqBody *OpenAIChatRequest, onChunk func(content string) error) error {
	apiEndpoint := ""
	if strings.HasPrefix(model, GEMINI_MODEL_PREFIX) {
		apiEndpoint = GEMINI_OPENAI_COMPATIBLE_API_URL
	} else if isOpenAiModel(model) {
		apiEndpoint = OPENAI_API_URL
	} else if openrouterModel, ok := strings.CutPrefix(model, OPENROUTER_MODEL_PREFIX); ok {
		if !strings.ContainsRune(openrouterModel, '/') {
			openrouterModel = OPENROUTER_MODEL_PREFIX + openrouterModel
		}
		model = openrouterModel
		apiEndpoint = OPENROUTER_API_URL
	} else if strings.HasPrefix(model, OPENAI_COMPATIBLE_MODEL_PREFIX) {
		parts := strings.SplitN(model, "/", 3)
		if len(parts) != 3 {
			return fmt.Errorf("invalid openai model %s", model)
		}
		apiEndpoint = parts[2]
		model = parts[1]
	} else {
		return fmt.Errorf("unsupported model %s", model)
	}
	reqBody.Model = model
	return CallOpenAIStream(apiEndpoint, apiKey, reqBody, onChunk)
}

// From https://platform.openai.com/docs/pricing
var openaiModels = []string{
	"codex-mini-latest",
	"computer-use-preview",
	"o1",
	"o3",
}
var openaiModelPrefixes = []string{
	"o1-",
	"o3-",
	"o4-",
}

func isOpenAiModel(model string) bool {
	return strings.HasPrefix(model, OPENAI_MODEL_PREFIX) || slices.Contains(openaiModels, model) ||
		slices.ContainsFunc(openaiModelPrefixes, func(prefix string) bool { return strings.HasPrefix(model, prefix) })
}
