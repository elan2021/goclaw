package skill

import "context"

// ToolDefinition describes a tool for the LLM (JSON Schema compatible).
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]ParamSchema `json:"parameters"`
	Required    []string               `json:"required"`
}

// ParamSchema describes a tool parameter.
type ParamSchema struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Enum        []string `json:"enum,omitempty"`
}

// ToolCall represents the LLM's request to execute a tool.
type ToolCall struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Arguments map[string]string `json:"arguments"`
}

// ToolResult is the return value from tool execution.
type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Output     string `json:"output"`
	Error      string `json:"error,omitempty"`
}

// Skill is the interface every tool must implement.
type Skill interface {
	// Definition returns the tool spec for the LLM.
	Definition() ToolDefinition

	// Execute runs the tool with the given arguments.
	// The context MUST be respected for timeout and cancellation.
	Execute(ctx context.Context, args map[string]string) (ToolResult, error)
}
