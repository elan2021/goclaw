package anthropic

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

const apiURL = "https://api.anthropic.com/v1/messages"

// Client implements llm.Provider for Anthropic Claude.
type Client struct {
	apiKey string
	model  string
	http   *http.Client
}

// New creates an Anthropic provider.
func New(apiKey, model string) *Client {
	return &Client{
		apiKey: apiKey,
		model:  model,
		http: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

func (c *Client) Name() string { return "anthropic" }

// ChatCompletion sends a request to Anthropic's messages endpoint.
func (c *Client) ChatCompletion(ctx context.Context, req llm.Request) (*llm.Response, error) {
	// Separate system message from conversation.
	var systemPrompt string
	var messages []apiMessage

	for _, m := range req.Messages {
		if m.Role == "system" {
			systemPrompt = m.Content
			continue
		}

		role := m.Role
		if role == "tool" {
			// Anthropic uses "user" role with tool_result content blocks.
			messages = append(messages, apiMessage{
				Role: "user",
				Content: []contentBlock{{
					Type:      "tool_result",
					ToolUseID: m.ToolCallID,
					Content:   m.Content,
				}},
			})
			continue
		}

		messages = append(messages, apiMessage{
			Role:    role,
			Content: []contentBlock{{Type: "text", Text: m.Content}},
		})
	}

	body := apiRequest{
		Model:     c.model,
		MaxTokens: 4096,
		System:    systemPrompt,
		Messages:  messages,
	}

	if len(req.Tools) > 0 {
		body.Tools = convertTools(req.Tools)
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
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

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
		return nil, fmt.Errorf("anthropic API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var apiResp apiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	result := &llm.Response{}
	for _, block := range apiResp.Content {
		switch block.Type {
		case "text":
			result.Content += block.Text
		case "tool_use":
			argsJSON, _ := json.Marshal(block.Input)
			result.ToolCalls = append(result.ToolCalls, llm.ToolCallRequest{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: string(argsJSON),
			})
		}
	}

	return result, nil
}

// --- Internal API types ---

type apiRequest struct {
	Model     string       `json:"model"`
	MaxTokens int          `json:"max_tokens"`
	System    string       `json:"system,omitempty"`
	Messages  []apiMessage `json:"messages"`
	Tools     []apiTool    `json:"tools,omitempty"`
}

type apiMessage struct {
	Role    string         `json:"role"`
	Content []contentBlock `json:"content"`
}

type contentBlock struct {
	Type      string      `json:"type"`
	Text      string      `json:"text,omitempty"`
	ID        string      `json:"id,omitempty"`
	Name      string      `json:"name,omitempty"`
	Input     interface{} `json:"input,omitempty"`
	ToolUseID string      `json:"tool_use_id,omitempty"`
	Content   string      `json:"content,omitempty"`
}

type apiResponse struct {
	Content []contentBlock `json:"content"`
}

type apiTool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"input_schema"`
}

func convertTools(specs []llm.ToolSpec) []apiTool {
	tools := make([]apiTool, 0, len(specs))
	for _, s := range specs {
		tools = append(tools, apiTool{
			Name:        s.Function.Name,
			Description: s.Function.Description,
			InputSchema: s.Function.Parameters,
		})
	}
	return tools
}
