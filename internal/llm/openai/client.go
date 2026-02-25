package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"goclaw/internal/llm"
)

const apiURL = "https://api.openai.com/v1/chat/completions"

// Client implements llm.Provider for OpenAI.
type Client struct {
	apiKey string
	model  string
	http   *http.Client
}

// New creates an OpenAI provider.
func New(apiKey, model string) *Client {
	return &Client{
		apiKey: apiKey,
		model:  model,
		http: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

func (c *Client) Name() string { return "openai" }

// ChatCompletion sends a request to OpenAI's chat completions endpoint.
func (c *Client) ChatCompletion(ctx context.Context, req llm.Request) (*llm.Response, error) {
	body := apiRequest{
		Model:    c.model,
		Messages: convertMessages(req.Messages),
	}

	if len(req.Tools) > 0 {
		body.Tools = req.Tools
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var apiResp apiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("openai returned no choices")
	}

	choice := apiResp.Choices[0]
	result := &llm.Response{
		Content: choice.Message.Content,
	}

	for _, tc := range choice.Message.ToolCalls {
		result.ToolCalls = append(result.ToolCalls, llm.ToolCallRequest{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		})
	}

	return result, nil
}

// --- Internal API types ---

type apiRequest struct {
	Model    string         `json:"model"`
	Messages []apiMessage   `json:"messages"`
	Tools    []llm.ToolSpec `json:"tools,omitempty"`
}

type apiMessage struct {
	Role       string        `json:"role"`
	Content    string        `json:"content,omitempty"`
	Name       string        `json:"name,omitempty"`
	ToolCallID string        `json:"tool_call_id,omitempty"`
	ToolCalls  []apiToolCall `json:"tool_calls,omitempty"`
}

type apiToolCall struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"`
	Function apiToolFunction `json:"function"`
}

type apiToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type apiResponse struct {
	Choices []apiChoice `json:"choices"`
}

type apiChoice struct {
	Message apiMessage `json:"message"`
}

func convertMessages(msgs []llm.Message) []apiMessage {
	out := make([]apiMessage, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, apiMessage{
			Role:       m.Role,
			Content:    m.Content,
			Name:       m.Name,
			ToolCallID: m.ToolCallID,
		})
	}
	return out
}
