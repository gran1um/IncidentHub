package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type OpenAIClient struct {
	chatURL   string
	modelsURL string
	apiKey    string
	client    *http.Client
}

func NewOpenAIClient(endpoint, apiKey string, timeout time.Duration) *OpenAIClient {
	endpoint = strings.TrimSpace(strings.TrimSuffix(endpoint, "/"))
	apiKey = strings.TrimSpace(apiKey)
	if endpoint == "" {
		return nil
	}
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	chatURL := endpoint + "/chat/completions"
	modelsURL := endpoint + "/models"
	if strings.HasSuffix(strings.ToLower(endpoint), "/chat/completions") {
		chatURL = endpoint
		modelsURL = strings.TrimSuffix(endpoint, "/chat/completions") + "/models"
	}
	return &OpenAIClient{
		chatURL:   chatURL,
		modelsURL: modelsURL,
		apiKey:    apiKey,
		client:    &http.Client{Timeout: timeout},
	}
}

func (c *OpenAIClient) Chat(ctx context.Context, model string, messages []ChatMessage) (string, error) {
	if c == nil || c.client == nil || c.chatURL == "" {
		return "", fmt.Errorf("openai client is not configured")
	}
	model = strings.TrimSpace(model)
	if model == "" {
		return "", fmt.Errorf("openai model is required")
	}

	payload := map[string]any{
		"model":    model,
		"messages": messages,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal openai request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.chatURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build openai request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("execute openai request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("openai request failed with status %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}

	var parsed struct {
		OutputText string `json:"output_text"`
		Choices    []struct {
			Text    string `json:"text"`
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", fmt.Errorf("decode openai response: %w", err)
	}

	for _, choice := range parsed.Choices {
		content := strings.TrimSpace(choice.Message.Content)
		if content != "" {
			return content, nil
		}
		if text := strings.TrimSpace(choice.Text); text != "" {
			return text, nil
		}
	}
	if text := strings.TrimSpace(parsed.OutputText); text != "" {
		return text, nil
	}
	return "", fmt.Errorf("empty openai response")
}

func (c *OpenAIClient) Ping(ctx context.Context) error {
	if c == nil || c.client == nil || c.modelsURL == "" {
		return fmt.Errorf("openai client is not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.modelsURL, http.NoBody)
	if err != nil {
		return fmt.Errorf("build openai ping request: %w", err)
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("execute openai ping request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("openai ping failed with status %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	return nil
}
