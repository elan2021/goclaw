package terminal

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"goclaw/internal/skill"
)

const maxOutputLen = 30_000

// Exec is a skill that executes shell commands.
type Exec struct{}

func (e *Exec) Definition() skill.ToolDefinition {
	return skill.ToolDefinition{
		Name:        "run_command",
		Description: "Execute a shell command and return its output. Use for system tasks like listing files, checking processes, running scripts. Commands have a 60-second timeout by default.",
		Parameters: map[string]skill.ParamSchema{
			"command": {
				Type:        "string",
				Description: "The shell command to execute",
			},
			"working_directory": {
				Type:        "string",
				Description: "Working directory for the command (optional, defaults to home dir)",
			},
		},
		Required: []string{"command"},
	}
}

func (e *Exec) Execute(ctx context.Context, args map[string]string) (skill.ToolResult, error) {
	command := args["command"]
	if command == "" {
		return skill.ToolResult{Error: "command argument is required"}, nil
	}

	// Apply a per-command timeout (max 60s).
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", command)
	}

	if dir := args["working_directory"]; dir != "" {
		cmd.Dir = dir
	}

	output, err := cmd.CombinedOutput()
	result := string(output)

	// Truncate very long output.
	if len(result) > maxOutputLen {
		result = result[:maxOutputLen] + "\n\n... [output truncated]"
	}

	if err != nil {
		if ctx.Err() != nil {
			return skill.ToolResult{
				Output: result,
				Error:  "command timed out after 60 seconds",
			}, nil
		}
		return skill.ToolResult{
			Output: result,
			Error:  fmt.Sprintf("command failed: %v", err),
		}, nil
	}

	if strings.TrimSpace(result) == "" {
		result = "(command completed successfully with no output)"
	}

	return skill.ToolResult{Output: result}, nil
}
