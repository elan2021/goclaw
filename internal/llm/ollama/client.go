package ollama

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

// Client implements llm.Provider for Ollama (local LLM).
type Client struct {
	baseURL string
	model   string
	http    *http.Client
}

// New creates an Ollama provider.
func New(baseURL, model string) *Client {
	return &Client{
		baseURL: baseURL,
		model:   model,
		http: &http.Client{
			Timeout: 300 * time.Second, // Local models can be slow.
		},
	}
}

func (c *Client) Name() string { return "ollama" }

// ChatCompletion sends a request to Ollama's chat endpoint.
func (c *Client) ChatCompletion(ctx context.Context, req llm.Request) (*llm.Response, error) {
	messages := make([]ollamaMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		messages = append(messages, ollamaMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	body := ollamaRequest{
		Model:    c.model,
		Messages: messages,
		Stream:   false, // Non-streaming for simplicity.
	}

	if len(req.Tools) > 0 {
		body.Tools = convertTools(req.Tools)
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/chat", c.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

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
		return nil, fmt.Errorf("ollama API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var apiResp ollamaResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	result := &llm.Response{
		Content: apiResp.Message.Content,
	}

	for _, tc := range apiResp.Message.ToolCalls {
		argsJSON, _ := json.Marshal(tc.Function.Arguments)
		result.ToolCalls = append(result.ToolCalls, llm.ToolCallRequest{
			ID:        fmt.Sprintf("call_%d", len(result.ToolCalls)),
			Name:      tc.Function.Name,
			Arguments: string(argsJSON),
		})
	}

	return result, nil
}

// --- Internal API types ---

type ollamaRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Tools    []ollamaTool    `json:"tools,omitempty"`
}

type ollamaMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
}

type ollamaToolCall struct {
	Function ollamaFunctionCall `json:"function"`
}

type ollamaFunctionCall struct {
	Name      string      `json:"name"`
	Arguments interface{} `json:"arguments"`
}

type ollamaResponse struct {
	Message ollamaMessage `json:"message"`
	Done    bool          `json:"done"`
}

type ollamaTool struct {
	Type     string            `json:"type"`
	Function ollamaFunctionDef `json:"function"`
}

type ollamaFunctionDef struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

func convertTools(specs []llm.ToolSpec) []ollamaTool {
	tools := make([]ollamaTool, 0, len(specs))
	for _, s := range specs {
		tools = append(tools, ollamaTool{
			Type: "function",
			Function: ollamaFunctionDef{
				Name:        s.Function.Name,
				Description: s.Function.Description,
				Parameters:  s.Function.Parameters,
			},
		})
	}
	return tools
}
