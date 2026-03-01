package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type OllamaClient struct {
	endpoint string
	client   *http.Client
}

func NewOllamaClient(endpoint string, timeout time.Duration) *OllamaClient {
	endpoint = strings.TrimSpace(strings.TrimSuffix(endpoint, "/"))
	if endpoint == "" {
		return nil
	}
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	return &OllamaClient{
		endpoint: endpoint,
		client:   &http.Client{Timeout: timeout},
	}
}

func (c *OllamaClient) Chat(ctx context.Context, model string, messages []ChatMessage) (string, error) {
	if c == nil || c.client == nil || c.endpoint == "" {
		return "", fmt.Errorf("ollama client is not configured")
	}
	model = strings.TrimSpace(model)
	if model == "" {
		return "", fmt.Errorf("ollama model is required")
	}
	payload := map[string]any{
		"model":    model,
		"messages": messages,
		"stream":   false,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal ollama request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build ollama request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("execute ollama request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("ollama request failed with status %s", resp.Status)
	}

	var parsed struct {
		Model   string `json:"model"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		Response string `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", fmt.Errorf("decode ollama response: %w", err)
	}

	content := strings.TrimSpace(parsed.Message.Content)
	if content == "" {
		content = strings.TrimSpace(parsed.Response)
	}
	if content == "" {
		return "", fmt.Errorf("empty ollama response")
	}
	return content, nil
}

func (c *OllamaClient) Ping(ctx context.Context) error {
	if c == nil || c.client == nil || c.endpoint == "" {
		return fmt.Errorf("ollama client is not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint+"/api/tags", http.NoBody)
	if err != nil {
		return fmt.Errorf("build ollama ping request: %w", err)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("execute ollama ping request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ollama ping failed with status %s", resp.Status)
	}
	return nil
}
