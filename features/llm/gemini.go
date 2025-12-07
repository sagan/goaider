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
	"time"

	"github.com/invopop/jsonschema"
	"github.com/sagan/goaider/constants"
)

const (
	// Gemini API base url
	GEMINI_API_URL = "https://generativelanguage.googleapis.com/v1beta/models/"

	// Base backoff for Gemini: set to 6s to respect the default 10 RPM quota
	GeminiApiBaseBackoff = 6 * time.Second
	GeminiApiMaxBackoff  = 60 * time.Second
)

// 2. API Request Structures (Nested structure for Gemini API)
type GeminiRequest struct {
	Contents         []Content         `json:"contents"`
	GenerationConfig *GenerationConfig `json:"generationConfig"`
}

type Content struct {
	Role  string `json:"role"`
	Parts []Part `json:"parts"`
}

type InlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

type Part struct {
	Text       string      `json:"text,omitempty"`
	InlineData *InlineData `json:"inlineData,omitempty"`
}

type GenerationConfig struct {
	ResponseMimeType   string             `json:"responseMimeType"`
	ResponseJsonSchema *jsonschema.Schema `json:"responseJsonSchema"`
	Temperature        float64            `json:"temperature"` // Higher = more creative
}

type GeminiResponse struct {
	PromptFeedback PromptFeedback `json:"promptFeedback"`
	Candidates     []Candidate    `json:"candidates"`
}

type Candidate struct {
	Content       Content        `json:"content"`
	FinishReason  string         `json:"finishReason"`
	Index         int            `json:"index"`
	SafetyRatings []SafetyRating `json:"safetyRatings"`
}

type SafetyRating struct {
	Category    string `json:"category"`
	Probability string `json:"probability"`
}

type PromptFeedback struct {
	BlockReason   string         `json:"blockReason,omitempty"`
	SafetyRatings []SafetyRating `json:"safetyRatings,omitempty"`
}

func Gemini(apiKey string, model string, reqBody *GeminiRequest) (*GeminiResponse, error) {
	if apiKey == "" {
		apiKey = os.Getenv(constants.ENV_GEMINI_API_KEY)
		if apiKey == "" {
			return nil, fmt.Errorf("Gemini api key or %s env not set", constants.ENV_GEMINI_API_KEY)
		}
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s%s:generateContent?key=%s", GEMINI_API_URL, model, apiKey)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, &ApiError{Status: resp.StatusCode, Body: string(bodyBytes)}
	}

	var apiResp *GeminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, &ApiError{Message: "failed to decode response", Err: err, Retryable: true}
	}

	if apiResp.PromptFeedback.BlockReason != "" {
		return nil, &ApiError{Message: fmt.Sprintf("blocked: %s", apiResp.PromptFeedback.BlockReason)}
	}

	if len(apiResp.Candidates) == 0 || len(apiResp.Candidates[0].Content.Parts) == 0 {
		return nil, &ApiError{Message: "no contents returned by Gemini", Retryable: true}
	}
	return apiResp, nil
}

// GeminiJsonResponse calls the Gemini API with a prompt and expects a JSON response
// that conforms to the schema of type T.
// It returns a pointer to the unmarshalled JSON object of type T.
// T must be a struct type.
func GeminiJsonResponse[T any](apiKey string, model string, promptText string) (*T, error) {
	schema := jsonschema.Reflect(new(T))
	reqBody := &GeminiRequest{
		Contents: []Content{{Parts: []Part{{Text: promptText}}}},
		GenerationConfig: &GenerationConfig{
			ResponseMimeType:   "application/json",
			ResponseJsonSchema: schema,
			Temperature:        1.0, // Higher: more creativity
		},
	}

	apiResp, err := Gemini(apiKey, model, reqBody)
	if err != nil {
		return nil, err
	}

	// The actual JSON data is a string *inside* the Text field
	rawJsonString := apiResp.Candidates[0].Content.Parts[0].Text

	result := new(T)
	if err := json.Unmarshal([]byte(rawJsonString), &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal internal JSON: %w", err)
	}

	return result, nil
}

// Simplest one-shot chat
func GeminiChat(apiKey string, model string, promptText string) (string, error) {
	reqBody := &GeminiRequest{
		Contents: []Content{{Parts: []Part{{Text: promptText}}}},
	}
	apiResp, err := Gemini(apiKey, model, reqBody)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(apiResp.Candidates[0].Content.Parts[0].Text), nil
}

// GeminiImageToText sends an image and a text prompt to Gemini and returns the text response.
// supported mimeTypes: "image/png", "image/jpeg", "image/webp", "image/heic", "image/heif"
func GeminiImageToText(apiKey string, model string, promptText string,
	imageBytes []byte, mimeType string) (string, error) {
	// 1. Encode image to Base64
	b64Data := base64.StdEncoding.EncodeToString(imageBytes)

	// 2. Construct Request
	reqBody := &GeminiRequest{
		Contents: []Content{
			{
				Role: "user",
				Parts: []Part{
					{Text: promptText},
					{
						InlineData: &InlineData{
							MimeType: mimeType,
							Data:     b64Data,
						},
					},
				},
			},
		},
		GenerationConfig: &GenerationConfig{
			Temperature: 0.4, // Lower temperature for more descriptive/accurate results
		},
	}

	// 3. Call API
	apiResp, err := Gemini(apiKey, model, reqBody)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(apiResp.Candidates[0].Content.Parts[0].Text), nil
}

// GeminiImageToJson sends an image and a text prompt to Gemini and enforces a JSON response
// that conforms to the schema of type T.
func GeminiImageToJson[T any](apiKey string, model string, promptText string, imageBytes []byte, mimeType string) (*T, error) {
	// 1. Generate Schema
	schema := jsonschema.Reflect(new(T))

	// 2. Encode Image
	b64Data := base64.StdEncoding.EncodeToString(imageBytes)

	// 3. Construct Request with Image AND Schema
	reqBody := &GeminiRequest{
		Contents: []Content{
			{
				Role: "user",
				Parts: []Part{
					{Text: promptText},
					{
						InlineData: &InlineData{
							MimeType: mimeType,
							Data:     b64Data,
						},
					},
				},
			},
		},
		GenerationConfig: &GenerationConfig{
			ResponseMimeType:   "application/json",
			ResponseJsonSchema: schema,
			Temperature:        0.4,
		},
	}

	// 4. Call API
	apiResp, err := Gemini(apiKey, model, reqBody)
	if err != nil {
		return nil, err
	}

	// 5. Parse JSON
	rawJsonString := apiResp.Candidates[0].Content.Parts[0].Text
	result := new(T)
	if err := json.Unmarshal([]byte(rawJsonString), &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal internal JSON: %w", err)
	}

	return result, nil
}
