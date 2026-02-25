package llm

import "context"

// Message represents a chat message in the LLM conversation.
type Message struct {
	Role       string `json:"role"` // "system" | "user" | "assistant" | "tool"
	Content    string `json:"content"`
	Name       string `json:"name,omitempty"`         // For tool messages
	ToolCallID string `json:"tool_call_id,omitempty"` // For tool results
}

// ToolCallRequest represents a tool_call from the LLM response.
type ToolCallRequest struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // Raw JSON string
}

// Request is the input to a chat completion call.
type Request struct {
	Messages []Message
	Tools    []ToolSpec
}

// ToolSpec mirrors the tool definition format expected by LLM APIs.
type ToolSpec struct {
	Type     string       `json:"type"` // "function"
	Function FunctionSpec `json:"function"`
}

// FunctionSpec defines a callable function for the LLM.
type FunctionSpec struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

// Response is the output from a chat completion call.
type Response struct {
	Content   string            // Text content (may be empty if tool_calls)
	ToolCalls []ToolCallRequest // Tool calls requested by the LLM
}

// Provider is the interface for all LLM backends.
type Provider interface {
	// Name returns the provider identifier ("openai", "anthropic", "ollama").
	Name() string

	// ChatCompletion sends a chat completion request and returns the response.
	ChatCompletion(ctx context.Context, req Request) (*Response, error)
}

// Msg is a helper to create a Message.
func Msg(role, content string) Message {
	return Message{Role: role, Content: content}
}

// ToolMsg creates a tool result message.
func ToolMsg(toolCallID, content string) Message {
	return Message{
		Role:       "tool",
		Content:    content,
		ToolCallID: toolCallID,
	}
}
