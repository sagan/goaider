package llm

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/invopop/jsonschema"
	"github.com/vincent-petithory/dataurl"

	"github.com/sagan/goaider/constants"
)

// =================================================================================
// OpenAI Compatible API Structures
// =================================================================================

const (
	// Default OpenAI Base URL
	OPENAI_API_URL = "https://api.openai.com/v1"
)

type OpenAIChatRequest struct {
	Model          string                `json:"model"`
	Messages       []*OpenAIMessage      `json:"messages"`
	Temperature    float64               `json:"temperature"`
	ResponseFormat *OpenAIResponseFormat `json:"response_format,omitempty"`
	Stream         bool                  `json:"stream,omitempty"`
}

type OpenAIMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // Can be string or []OpenAIContentPart
}

type OpenAIContentPart struct {
	Type     string          `json:"type"` // text, image_url
	Text     string          `json:"text,omitempty"`
	ImageUrl *OpenAIImageUrl `json:"image_url,omitempty"`
}

type OpenAIImageUrl struct {
	Url string `json:"url"` // "data:image/jpeg;base64,{base64_image}"
}

// OpenAI Structured Output / JSON Mode
type OpenAIResponseFormat struct {
	Type       string            `json:"type"`                  // "json_object" or "json_schema"
	JsonSchema *OpenAIJsonSchema `json:"json_schema,omitempty"` // For "json_schema" type
}

type OpenAIJsonSchema struct {
	Name   string             `json:"name"`
	Schema *jsonschema.Schema `json:"schema"`
	Strict bool               `json:"strict"`
}

type OpenAIResponse struct {
	ID      string         `json:"id"`
	Choices []OpenAIChoice `json:"choices"`
	Error   *OpenAIError   `json:"error,omitempty"` // Sometimes returned in 200 OK by proxies
}

type OpenAIChoice struct {
	Index        int           `json:"index"`
	Message      OpenAIMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

// OpenAI specific error structure inside the JSON body
type OpenAIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    any    `json:"code"`
}

// =================================================================================
// Streaming Specific Structures
// =================================================================================
type OpenAIStreamChunk struct {
	ID      string               `json:"id"`
	Choices []OpenAIStreamChoice `json:"choices"`
}

type OpenAIStreamChoice struct {
	Index        int         `json:"index"`
	Delta        OpenAIDelta `json:"delta"`
	FinishReason string      `json:"finish_reason"`
}

type OpenAIDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

func getOpenAIApiKeyFromEnv(baseUrl string) (apiKey string, err error) {
	switch baseUrl {
	case OPENAI_API_URL:
		apiKey = os.Getenv(constants.ENV_OPENAI_API_KEY)
		if apiKey == "" {
			err = fmt.Errorf("OpenAI api key or %s env not set", constants.ENV_OPENAI_API_KEY)
		}
	case OPENROUTER_API_URL:
		apiKey = os.Getenv(constants.ENV_OPENROUTER_API_KEY)
		if apiKey == "" {
			err = fmt.Errorf("OpenRouter api key or %s env not set", constants.ENV_OPENROUTER_API_KEY)
		}
	case GEMINI_OPENAI_COMPATIBLE_API_URL:
		apiKey = os.Getenv(constants.ENV_GEMINI_API_KEY)
		if apiKey == "" {
			err = fmt.Errorf("Gemini api key or %s env not set", constants.ENV_GEMINI_API_KEY)
		}
	default:
		apiKey = os.Getenv(constants.ENV_MODEL_KEY)
	}
	return apiKey, err
}

// CallOpenAI is the base function for OpenAI compatible APIs.
// baseUrl: e.g., "https://api.openai.com/v1" or "http://localhost:11434/v1"
func CallOpenAI(baseUrl string, apiKey string, reqBody *OpenAIChatRequest) (apiResp *OpenAIResponse, err error) {
	if apiKey == "" {
		apiKey, err = getOpenAIApiKeyFromEnv(baseUrl)
		if err != nil {
			return nil, err
		}
	}
	reqBody.Stream = false

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Handle trailing slash consistency
	baseUrl = strings.TrimRight(baseUrl, "/")
	url := fmt.Sprintf("%s/chat/completions", baseUrl)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read body to handle errors or decode
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Handle non-200 HTTP statuses
	if resp.StatusCode != 200 {
		return nil, &ApiError{
			Status:    resp.StatusCode,
			Body:      string(bodyBytes),
			Message:   fmt.Sprintf("OpenAI API returned status %d", resp.StatusCode),
			Retryable: resp.StatusCode == 429 || resp.StatusCode >= 500,
		}
	}

	if err := json.Unmarshal(bodyBytes, &apiResp); err != nil {
		return nil, &ApiError{Message: "failed to decode response", Err: err, Retryable: true}
	}

	// Handle logic-level errors (API returned 200 but body contains error)
	if apiResp.Error != nil {
		return nil, &ApiError{Message: fmt.Sprintf("api error: %s", apiResp.Error.Message), Body: string(bodyBytes)}
	}

	if len(apiResp.Choices) == 0 {
		return nil, &ApiError{Message: "no choices returned by API", Retryable: true}
	}

	return apiResp, nil
}

// CallOpenAIStream handles streaming responses (SSE).
// onChunk: A callback function invoked for every text token received.
// Return an error from onChunk to stop streaming immediately.
func CallOpenAIStream(baseUrl, apiKey string, reqBody *OpenAIChatRequest,
	onChunk func(content string) error) (err error) {
	if apiKey == "" {
		apiKey, err = getOpenAIApiKeyFromEnv(baseUrl)
		if err != nil {
			return err
		}
	}
	// Force stream to true
	reqBody.Stream = true

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	baseUrl = strings.TrimRight(baseUrl, "/")
	url := fmt.Sprintf("%s/chat/completions", baseUrl)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	req.Header.Set("Accept", "text/event-stream") // Standard for SSE

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Handle non-200 errors (API errors usually come as JSON, but not streamed)
	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return &ApiError{
			Status:    resp.StatusCode,
			Body:      string(bodyBytes),
			Message:   fmt.Sprintf("OpenAI API returned status %d", resp.StatusCode),
			Retryable: resp.StatusCode == 429 || resp.StatusCode >= 500,
		}
	}

	// Read the stream line by line
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()

		// SSE lines start with "data: "
		// Sometimes we get empty lines (keep-alives), ignore them
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		dataStr := strings.TrimPrefix(line, "data: ")
		dataStr = strings.TrimSpace(dataStr)

		// Check for the [DONE] sentinel
		if dataStr == "[DONE]" {
			break
		}

		var chunk OpenAIStreamChunk
		if err := json.Unmarshal([]byte(dataStr), &chunk); err != nil {
			// If we can't unmarshal a specific chunk, we might log it but continue,
			// or fail. Here we fail to ensure integrity.
			return fmt.Errorf("failed to unmarshal stream chunk: %w", err)
		}

		if len(chunk.Choices) > 0 {
			content := chunk.Choices[0].Delta.Content
			// Only trigger callback if there is actual content
			if content != "" {
				if err := onChunk(content); err != nil {
					return err // User requested to stop
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading stream: %w", err)
	}

	return nil
}

// OpenAIChat performs a simple text-in, text-out conversation.
func OpenAIChat(baseUrl string, apiKey string, model string, promptText string, temperature float64) (string, error) {
	reqBody := &OpenAIChatRequest{
		Model:       model,
		Messages:    []*OpenAIMessage{{Role: "user", Content: promptText}},
		Temperature: temperature,
	}

	resp, err := CallOpenAI(baseUrl, apiKey, reqBody)
	if err != nil {
		return "", err
	}

	// Content can be string or objects, for simple chat it's usually string.
	// To be safe, we cast or check, but usually the response message content is a string.
	contentStr, ok := resp.Choices[0].Message.Content.(string)
	if !ok {
		// Fallback: marshal it if it's complex structure
		b, _ := json.Marshal(resp.Choices[0].Message.Content)
		return string(b), nil
	}

	return strings.TrimSpace(contentStr), nil
}

// OpenAIJsonResponse enforces a JSON Schema response.
// Note: Not all "OpenAI compatible" endpoints support "response_format: json_schema".
// This implementation uses the strict "json_schema" format standardized by OpenAI.
func OpenAIJsonResponse[T any](baseUrl string, apiKey string, model string,
	promptText string, temperature float64) (*T, error) {
	schema := jsonschema.Reflect(new(T))

	// OpenAI requires strict schema adherence
	reqBody := &OpenAIChatRequest{
		Model:       model,
		Messages:    []*OpenAIMessage{{Role: "user", Content: promptText}},
		Temperature: temperature, // Lower temp for structured data
		ResponseFormat: &OpenAIResponseFormat{
			Type: "json_schema",
			JsonSchema: &OpenAIJsonSchema{
				Name:   "response_schema",
				Schema: schema,
				Strict: true,
			},
		},
	}

	resp, err := CallOpenAI(baseUrl, apiKey, reqBody)
	if err != nil {
		return nil, err
	}

	rawJsonString, ok := resp.Choices[0].Message.Content.(string)
	if !ok {
		return nil, fmt.Errorf("unexpected content format in response")
	}
	rawJsonString = StripJsonWrap(rawJsonString)

	result := new(T)
	if err := json.Unmarshal([]byte(rawJsonString), &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal internal JSON: %w", err)
	}

	return result, nil
}

// OpenAIImageToText handles Vision capabilities.
func OpenAIImageToText(apiKey string, baseUrl string, model string, promptText string,
	imageBytes []byte, mimeType string, temperature float64) (string, error) {
	dataUrl := ""
	if mimeType != "" {
		dataUrl = dataurl.New(imageBytes, mimeType).String()
	} else {
		dataUrl = dataurl.EncodeBytes(imageBytes)
	}

	contentParts := []OpenAIContentPart{
		{Type: "text", Text: promptText},
		{Type: "image_url", ImageUrl: &OpenAIImageUrl{Url: dataUrl}},
	}

	reqBody := &OpenAIChatRequest{
		Model:       model,
		Messages:    []*OpenAIMessage{{Role: "user", Content: contentParts}},
		Temperature: temperature,
	}

	resp, err := CallOpenAI(baseUrl, apiKey, reqBody)
	if err != nil {
		return "", err
	}

	contentStr, ok := resp.Choices[0].Message.Content.(string)
	if !ok {
		return "", fmt.Errorf("unexpected content format")
	}
	return strings.TrimSpace(contentStr), nil
}

// OpenAIImageToJson combines Vision and Structured Outputs.
func OpenAIImageToJson[T any](baseUrl string, apiKey string, model string, promptText string,
	imageBytes []byte, mimeType string, temperature float64) (*T, error) {
	schema := jsonschema.Reflect(new(T))

	dataUrl := ""
	if mimeType != "" {
		dataUrl = dataurl.New(imageBytes, mimeType).String()
	} else {
		dataUrl = dataurl.EncodeBytes(imageBytes)
	}

	contentParts := []OpenAIContentPart{
		{Type: "text", Text: promptText},
		{Type: "image_url", ImageUrl: &OpenAIImageUrl{Url: dataUrl}},
	}

	reqBody := &OpenAIChatRequest{
		Model:       model,
		Messages:    []*OpenAIMessage{{Role: "user", Content: contentParts}},
		Temperature: temperature,
		ResponseFormat: &OpenAIResponseFormat{
			Type: "json_schema",
			JsonSchema: &OpenAIJsonSchema{
				Name:   "result_schema",
				Schema: schema,
				Strict: true,
			},
		},
	}

	resp, err := CallOpenAI(baseUrl, apiKey, reqBody)
	if err != nil {
		return nil, err
	}

	rawJsonString, ok := resp.Choices[0].Message.Content.(string)
	if !ok {
		return nil, fmt.Errorf("unexpected content format in response")
	}
	rawJsonString = StripJsonWrap(rawJsonString)

	result := new(T)
	if err := json.Unmarshal([]byte(rawJsonString), &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal internal JSON: %w", err)
	}

	return result, nil
}
