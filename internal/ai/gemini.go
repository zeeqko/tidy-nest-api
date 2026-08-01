// Package ai calls Google's Gemini API to recognize the category and name of
// an item from a user-uploaded photo, for the "AI Recognition" add-item flow.
package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// ErrNotConfigured means GEMINI_API_KEY is unset, so recognition is unavailable.
var ErrNotConfigured = errors.New("ai: GEMINI_API_KEY not configured")

const (
	geminiModel    = "gemini-3.5-flash-lite"
	endpoint       = "https://generativelanguage.googleapis.com/v1beta/models/" + geminiModel + ":generateContent"
	requestTimeout = 8 * time.Second
)

// Client calls Gemini to recognize item photos.
type Client struct {
	apiKey string
	http   *http.Client
}

// NewClientFromEnv builds a Client from GEMINI_API_KEY. A nil return (no
// error) means recognition is unavailable but the server should still start
// — callers must nil-check before use, mirroring storage.FromEnv's local-disk
// fallback pattern for "feature not configured" rather than treating it as fatal.
func NewClientFromEnv() *Client {
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		return nil
	}
	return &Client{apiKey: key, http: &http.Client{Timeout: requestTimeout}}
}

// Recognition is what Gemini reports about a photographed item.
type Recognition struct {
	Category string `json:"category"`
	ItemName string `json:"itemName"`
}

// Recognize asks Gemini to classify imageData (raw image bytes) into one of
// categoryNames plus a short item name. categoryNames constrains the
// response via the request's JSON schema so the result always maps to a real
// category the caller already has; it must be non-empty.
func (c *Client) Recognize(ctx context.Context, imageData []byte, contentType string, categoryNames []string) (Recognition, error) {
	if c == nil {
		return Recognition{}, ErrNotConfigured
	}
	if len(categoryNames) == 0 {
		return Recognition{}, errors.New("ai: no category names to classify into")
	}

	reqBody := geminiRequest{
		Contents: []geminiContent{{
			Parts: []geminiPart{
				{Text: recognitionPrompt},
				{InlineData: &geminiInlineData{MimeType: contentType, Data: base64.StdEncoding.EncodeToString(imageData)}},
			},
		}},
		GenerationConfig: geminiGenerationConfig{
			ResponseMimeType: "application/json",
			ResponseSchema: geminiSchema{
				Type: "OBJECT",
				Properties: map[string]geminiSchema{
					"category": {Type: "STRING", Enum: categoryNames},
					"itemName": {Type: "STRING"},
				},
				Required: []string{"category", "itemName"},
			},
		},
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return Recognition{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return Recognition{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	// Header, not a ?key= query param: net/http wraps pre-response failures
	// (timeouts, DNS errors, etc.) in a *url.Error whose Error() includes the
	// full request URL, which would otherwise leak the key into server logs.
	req.Header.Set("x-goog-api-key", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return Recognition{}, fmt.Errorf("ai: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Recognition{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return Recognition{}, fmt.Errorf("ai: gemini returned %d: %s", resp.StatusCode, truncate(body, 300))
	}

	var parsed geminiResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return Recognition{}, fmt.Errorf("ai: decode response: %w", err)
	}
	if len(parsed.Candidates) == 0 || len(parsed.Candidates[0].Content.Parts) == 0 {
		return Recognition{}, errors.New("ai: empty response from gemini")
	}

	var out Recognition
	if err := json.Unmarshal([]byte(parsed.Candidates[0].Content.Parts[0].Text), &out); err != nil {
		return Recognition{}, fmt.Errorf("ai: decode recognition: %w", err)
	}
	if out.Category == "" || out.ItemName == "" {
		return Recognition{}, errors.New("ai: incomplete recognition result")
	}
	return out, nil
}

const recognitionPrompt = `You are helping catalog a personal item for a home inventory app. Look at the photo and identify what the item is. Respond with the best-matching category from the provided list, and a short, specific, human-friendly item name (2-4 words, e.g. "Running Sneakers", "Wireless Mouse", "Wool Scarf") - not a generic restatement of the category.`

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "…"
}

// --- Gemini REST API request/response shapes (only the fields we use) ---

type geminiRequest struct {
	Contents         []geminiContent        `json:"contents"`
	GenerationConfig geminiGenerationConfig `json:"generationConfig"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text       string            `json:"text,omitempty"`
	InlineData *geminiInlineData `json:"inline_data,omitempty"`
}

type geminiInlineData struct {
	MimeType string `json:"mime_type"`
	Data     string `json:"data"`
}

type geminiGenerationConfig struct {
	ResponseMimeType string       `json:"responseMimeType"`
	ResponseSchema   geminiSchema `json:"responseSchema"`
}

type geminiSchema struct {
	Type       string                  `json:"type"`
	Properties map[string]geminiSchema `json:"properties,omitempty"`
	Required   []string                `json:"required,omitempty"`
	Enum       []string                `json:"enum,omitempty"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}
