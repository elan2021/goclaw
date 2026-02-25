package cronjob

import (
	"context"
	"fmt"
	"strings"
	"time"

	"goclaw/internal/scheduler"
	"goclaw/internal/skill"
)

// SetCronjob is a skill that lets the LLM schedule recurring tasks.
type SetCronjob struct {
	Scheduler *scheduler.Scheduler
}

func (s *SetCronjob) Definition() skill.ToolDefinition {
	return skill.ToolDefinition{
		Name:        "set_cronjob",
		Description: "Schedule a recurring task using a cron expression. The task will run at the specified interval. Use standard cron format: 'minute hour day month weekday'. Examples: '0 9 * * *' (every day at 9am), '0 */2 * * *' (every 2 hours), '30 8 * * 1' (Monday 8:30am).",
		Parameters: map[string]skill.ParamSchema{
			"message": {
				Type:        "string",
				Description: "The message to send or task description for the agent to execute",
			},
			"cron": {
				Type:        "string",
				Description: "Cron expression: 'minute hour day month weekday' (e.g., '0 9 * * *')",
			},
			"is_agent_task": {
				Type:        "string",
				Description: "If 'true', the agent will execute the message as a prompt. If 'false' (default), just sends the message as a reminder.",
			},
			"user_id": {
				Type:        "string",
				Description: "The user ID",
			},
			"chat_id": {
				Type:        "string",
				Description: "The chat ID",
			},
			"platform": {
				Type:        "string",
				Description: "The platform ('telegram' or 'whatsapp')",
			},
		},
		Required: []string{"message", "cron", "user_id", "chat_id", "platform"},
	}
}

func (s *SetCronjob) Execute(ctx context.Context, args map[string]string) (skill.ToolResult, error) {
	message := args["message"]
	cronExpr := args["cron"]
	userID := args["user_id"]
	chatID := args["chat_id"]
	platform := args["platform"]
	isAgent := strings.ToLower(args["is_agent_task"]) == "true"

	if message == "" || cronExpr == "" {
		return skill.ToolResult{Error: "message and cron arguments are required"}, nil
	}

	// Validate cron expression has 5 fields.
	fields := strings.Fields(cronExpr)
	if len(fields) != 5 {
		return skill.ToolResult{Error: "cron expression must have 5 fields: minute hour day month weekday"}, nil
	}

	task := scheduler.ScheduledTask{
		ID:          fmt.Sprintf("cron_%d", time.Now().UnixNano()),
		UserID:      userID,
		ChatID:      chatID,
		Platform:    platform,
		Message:     message,
		CronExpr:    cronExpr,
		IsAgentTask: isAgent,
		CreatedAt:   time.Now(),
	}

	if err := s.Scheduler.Add(task); err != nil {
		return skill.ToolResult{Error: fmt.Sprintf("failed to schedule: %v", err)}, nil
	}

	desc := describeCron(cronExpr)
	return skill.ToolResult{
		Output: fmt.Sprintf("✅ Cronjob criado (%s): %q [ID: %s]", desc, message, task.ID),
	}, nil
}

func describeCron(expr string) string {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return expr
	}

	min, hour := fields[0], fields[1]

	if min == "0" && hour != "*" && fields[2] == "*" && fields[3] == "*" && fields[4] == "*" {
		return fmt.Sprintf("todo dia às %s:00", hour)
	}
	if min != "*" && hour != "*" && fields[2] == "*" && fields[3] == "*" && fields[4] == "*" {
		return fmt.Sprintf("todo dia às %s:%s", hour, min)
	}
	return expr
}

// ListCronjobs is a skill to list all scheduled tasks.
type ListCronjobs struct {
	Scheduler *scheduler.Scheduler
}

func (l *ListCronjobs) Definition() skill.ToolDefinition {
	return skill.ToolDefinition{
		Name:        "list_schedules",
		Description: "List all scheduled reminders and cronjobs.",
		Parameters:  map[string]skill.ParamSchema{},
		Required:    []string{},
	}
}

func (l *ListCronjobs) Execute(ctx context.Context, args map[string]string) (skill.ToolResult, error) {
	tasks := l.Scheduler.List()
	if len(tasks) == 0 {
		return skill.ToolResult{Output: "Nenhuma tarefa agendada."}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📋 %d tarefa(s) agendada(s):\n\n", len(tasks)))

	for _, t := range tasks {
		if t.CronExpr != "" {
			sb.WriteString(fmt.Sprintf("🔁 [%s] Cron: %s → %q\n", t.ID, t.CronExpr, t.Message))
		} else {
			sb.WriteString(fmt.Sprintf("⏰ [%s] Em %s → %q\n", t.ID, t.RunAt.Format("02/01 15:04"), t.Message))
		}
	}

	return skill.ToolResult{Output: sb.String()}, nil
}
