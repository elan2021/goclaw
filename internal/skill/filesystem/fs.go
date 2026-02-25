package filesystem

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"goclaw/internal/skill"
)

// ReadFile is a skill that reads file contents.
type ReadFile struct{}

func (r *ReadFile) Definition() skill.ToolDefinition {
	return skill.ToolDefinition{
		Name:        "read_file",
		Description: "Read the contents of a file at the given path. Returns the full text content of the file.",
		Parameters: map[string]skill.ParamSchema{
			"path": {
				Type:        "string",
				Description: "Absolute or relative path to the file to read",
			},
		},
		Required: []string{"path"},
	}
}

func (r *ReadFile) Execute(ctx context.Context, args map[string]string) (skill.ToolResult, error) {
	path := args["path"]
	if path == "" {
		return skill.ToolResult{Error: "path argument is required"}, nil
	}

	// Resolve to absolute path.
	absPath, err := filepath.Abs(path)
	if err != nil {
		return skill.ToolResult{Error: fmt.Sprintf("invalid path: %v", err)}, nil
	}

	select {
	case <-ctx.Done():
		return skill.ToolResult{}, ctx.Err()
	default:
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return skill.ToolResult{Error: fmt.Sprintf("failed to read file: %v", err)}, nil
	}

	// Truncate very large files.
	content := string(data)
	const maxLen = 50_000
	if len(content) > maxLen {
		content = content[:maxLen] + "\n\n... [truncated, file too large]"
	}

	return skill.ToolResult{Output: content}, nil
}

// WriteFile is a skill that writes content to a file.
type WriteFile struct{}

func (w *WriteFile) Definition() skill.ToolDefinition {
	return skill.ToolDefinition{
		Name:        "write_file",
		Description: "Write content to a file at the given path. Creates parent directories if needed. Overwrites existing content.",
		Parameters: map[string]skill.ParamSchema{
			"path": {
				Type:        "string",
				Description: "Absolute or relative path to the file to write",
			},
			"content": {
				Type:        "string",
				Description: "The content to write to the file",
			},
		},
		Required: []string{"path", "content"},
	}
}

func (w *WriteFile) Execute(ctx context.Context, args map[string]string) (skill.ToolResult, error) {
	path := args["path"]
	content := args["content"]
	if path == "" {
		return skill.ToolResult{Error: "path argument is required"}, nil
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return skill.ToolResult{Error: fmt.Sprintf("invalid path: %v", err)}, nil
	}

	select {
	case <-ctx.Done():
		return skill.ToolResult{}, ctx.Err()
	default:
	}

	// Ensure parent directory exists.
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return skill.ToolResult{Error: fmt.Sprintf("failed to create directory: %v", err)}, nil
	}

	if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
		return skill.ToolResult{Error: fmt.Sprintf("failed to write file: %v", err)}, nil
	}

	return skill.ToolResult{Output: fmt.Sprintf("File written successfully: %s (%d bytes)", absPath, len(content))}, nil
}

// ListDir is a skill that lists directory contents.
type ListDir struct{}

func (l *ListDir) Definition() skill.ToolDefinition {
	return skill.ToolDefinition{
		Name:        "list_directory",
		Description: "List the contents of a directory. Returns file and subdirectory names with their types and sizes.",
		Parameters: map[string]skill.ParamSchema{
			"path": {
				Type:        "string",
				Description: "Absolute or relative path to the directory to list",
			},
		},
		Required: []string{"path"},
	}
}

func (l *ListDir) Execute(ctx context.Context, args map[string]string) (skill.ToolResult, error) {
	path := args["path"]
	if path == "" {
		path = "."
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return skill.ToolResult{Error: fmt.Sprintf("invalid path: %v", err)}, nil
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		return skill.ToolResult{Error: fmt.Sprintf("failed to read directory: %v", err)}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Directory: %s\n\n", absPath))

	for _, entry := range entries {
		info, _ := entry.Info()
		if entry.IsDir() {
			sb.WriteString(fmt.Sprintf("  [DIR]  %s/\n", entry.Name()))
		} else if info != nil {
			sb.WriteString(fmt.Sprintf("  [FILE] %s (%d bytes)\n", entry.Name(), info.Size()))
		} else {
			sb.WriteString(fmt.Sprintf("  [FILE] %s\n", entry.Name()))
		}
	}

	return skill.ToolResult{Output: sb.String()}, nil
}
