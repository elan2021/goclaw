package reminder

import (
	"context"
	"fmt"
	"time"

	"goclaw/internal/scheduler"
	"goclaw/internal/skill"
)

// SetReminder is a skill that lets the LLM schedule one-time reminders.
type SetReminder struct {
	Scheduler *scheduler.Scheduler
}

func (s *SetReminder) Definition() skill.ToolDefinition {
	return skill.ToolDefinition{
		Name:        "set_reminder",
		Description: "Schedule a one-time reminder. The user will receive a message at the specified time. Use ISO 8601 format for the time (e.g., '2026-02-25T15:00:00-03:00'). You can also use relative times like '+30m' (30 minutes), '+2h' (2 hours), '+1d' (1 day).",
		Parameters: map[string]skill.ParamSchema{
			"message": {
				Type:        "string",
				Description: "The reminder message to send",
			},
			"at": {
				Type:        "string",
				Description: "When to send the reminder. ISO 8601 datetime or relative: '+30m', '+2h', '+1d'",
			},
			"user_id": {
				Type:        "string",
				Description: "The user ID to send the reminder to (use the current user's ID)",
			},
			"chat_id": {
				Type:        "string",
				Description: "The chat ID to send the reminder to",
			},
			"platform": {
				Type:        "string",
				Description: "The platform to send the reminder on ('telegram' or 'whatsapp')",
			},
		},
		Required: []string{"message", "at", "user_id", "chat_id", "platform"},
	}
}

func (s *SetReminder) Execute(ctx context.Context, args map[string]string) (skill.ToolResult, error) {
	message := args["message"]
	at := args["at"]
	userID := args["user_id"]
	chatID := args["chat_id"]
	platform := args["platform"]

	if message == "" || at == "" {
		return skill.ToolResult{Error: "message and at arguments are required"}, nil
	}

	runAt, err := parseTime(at)
	if err != nil {
		return skill.ToolResult{Error: fmt.Sprintf("invalid time format: %v", err)}, nil
	}

	if runAt.Before(time.Now()) {
		return skill.ToolResult{Error: "cannot schedule a reminder in the past"}, nil
	}

	task := scheduler.ScheduledTask{
		ID:          fmt.Sprintf("rem_%d", time.Now().UnixNano()),
		UserID:      userID,
		ChatID:      chatID,
		Platform:    platform,
		Message:     fmt.Sprintf("⏰ Lembrete: %s", message),
		RunAt:       runAt,
		IsAgentTask: false,
		CreatedAt:   time.Now(),
	}

	if err := s.Scheduler.Add(task); err != nil {
		return skill.ToolResult{Error: fmt.Sprintf("failed to schedule: %v", err)}, nil
	}

	return skill.ToolResult{
		Output: fmt.Sprintf("✅ Lembrete agendado para %s: %q", runAt.Format("02/01/2006 15:04"), message),
	}, nil
}

func parseTime(s string) (time.Time, error) {
	// Relative time: +30m, +2h, +1d.
	if len(s) > 1 && s[0] == '+' {
		unit := s[len(s)-1]
		numStr := s[1 : len(s)-1]

		var val int
		for _, c := range numStr {
			if c < '0' || c > '9' {
				break
			}
			val = val*10 + int(c-'0')
		}

		if val == 0 {
			return time.Time{}, fmt.Errorf("invalid relative time: %s", s)
		}

		switch unit {
		case 'm':
			return time.Now().Add(time.Duration(val) * time.Minute), nil
		case 'h':
			return time.Now().Add(time.Duration(val) * time.Hour), nil
		case 'd':
			return time.Now().Add(time.Duration(val) * 24 * time.Hour), nil
		default:
			return time.Time{}, fmt.Errorf("unknown time unit: %c (use m, h, or d)", unit)
		}
	}

	// ISO 8601.
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		// Try without timezone.
		t, err = time.ParseInLocation("2006-01-02T15:04:05", s, time.Local)
	}
	return t, err
}
