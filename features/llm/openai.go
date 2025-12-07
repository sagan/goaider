package llm

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/invopop/jsonschema"
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
	Messages       []OpenAIMessage       `json:"messages"`
	Temperature    float64               `json:"temperature"`
	ResponseFormat *OpenAIResponseFormat `json:"response_format,omitempty"`
	Stream         bool                  `json:"stream,omitempty"`
}

type OpenAIMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // Can be string or []OpenAIContentPart
}

type OpenAIContentPart struct {
	Type     string          `json:"type"`
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

// CallOpenAI is the base function for OpenAI compatible APIs.
// baseUrl: e.g., "https://api.openai.com/v1" or "http://localhost:11434/v1"
func CallOpenAI(baseUrl string, apiKey string, reqBody *OpenAIChatRequest) (*OpenAIResponse, error) {
	if apiKey == "" && (baseUrl == OPENAI_API_URL || baseUrl == OPENROUTER_API_URL) {
		switch baseUrl {
		case OPENAI_API_URL:
			apiKey = os.Getenv(constants.ENV_OPENAI_API_KEY)
			if apiKey == "" {
				return nil, fmt.Errorf("OpenAI api key or %s env not set", constants.ENV_OPENAI_API_KEY)
			}
		case OPENROUTER_API_URL:
			apiKey = os.Getenv(constants.ENV_OPENROUTER_API_KEY)
			if apiKey == "" {
				return nil, fmt.Errorf("OpenRouter api key or %s env not set", constants.ENV_OPENROUTER_API_KEY)
			}
		}
	}

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

	var apiResp OpenAIResponse
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

	return &apiResp, nil
}

// OpenAIChat performs a simple text-in, text-out conversation.
func OpenAIChat(baseUrl string, apiKey string, model string, promptText string) (string, error) {
	reqBody := &OpenAIChatRequest{
		Model:       model,
		Messages:    []OpenAIMessage{{Role: "user", Content: promptText}},
		Temperature: 0.7,
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
func OpenAIJsonResponse[T any](baseUrl string, apiKey string, model string, promptText string) (*T, error) {
	schema := jsonschema.Reflect(new(T))

	// OpenAI requires strict schema adherence
	reqBody := &OpenAIChatRequest{
		Model:       model,
		Messages:    []OpenAIMessage{{Role: "user", Content: promptText}},
		Temperature: 0.2, // Lower temp for structured data
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

	result := new(T)
	if err := json.Unmarshal([]byte(rawJsonString), &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal internal JSON: %w", err)
	}

	return result, nil
}

// OpenAIImageToText handles Vision capabilities.
func OpenAIImageToText(apiKey string, baseUrl string, model string, promptText string,
	imageBytes []byte, mimeType string) (string, error) {
	b64Data := base64.StdEncoding.EncodeToString(imageBytes)
	dataUrl := fmt.Sprintf("data:%s;base64,%s", mimeType, b64Data)

	// Construct multimodal message
	contentParts := []OpenAIContentPart{
		{Type: "text", Text: promptText},
		{Type: "image_url", ImageUrl: &OpenAIImageUrl{Url: dataUrl}},
	}

	reqBody := &OpenAIChatRequest{
		Model:       model,
		Messages:    []OpenAIMessage{{Role: "user", Content: contentParts}},
		Temperature: 0.4,
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
	imageBytes []byte, mimeType string) (*T, error) {
	schema := jsonschema.Reflect(new(T))

	b64Data := base64.StdEncoding.EncodeToString(imageBytes)
	dataUrl := fmt.Sprintf("data:%s;base64,%s", mimeType, b64Data)

	contentParts := []OpenAIContentPart{
		{Type: "text", Text: promptText},
		{Type: "image_url", ImageUrl: &OpenAIImageUrl{Url: dataUrl}},
	}

	reqBody := &OpenAIChatRequest{
		Model:       model,
		Messages:    []OpenAIMessage{{Role: "user", Content: contentParts}},
		Temperature: 0.4,
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

	result := new(T)
	if err := json.Unmarshal([]byte(rawJsonString), &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal internal JSON: %w", err)
	}

	return result, nil
}
