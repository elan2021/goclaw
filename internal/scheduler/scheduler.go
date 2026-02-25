package scheduler

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"sync"
	"time"
)

// ScheduledTask represents a scheduled task (reminder or cronjob).
type ScheduledTask struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	ChatID      string    `json:"chat_id"`
	Platform    string    `json:"platform"`
	Message     string    `json:"message"`
	RunAt       time.Time `json:"run_at"`
	CronExpr    string    `json:"cron_expr,omitempty"`
	IsAgentTask bool      `json:"is_agent_task"`
	CreatedAt   time.Time `json:"created_at"`
}

// Dispatcher is the callback the scheduler calls when a task fires.
type Dispatcher func(ctx context.Context, task ScheduledTask)

// Scheduler manages scheduled tasks with disk persistence.
type Scheduler struct {
	mu       sync.RWMutex
	tasks    []ScheduledTask
	filePath string
	dispatch Dispatcher
}

// New creates a scheduler that persists to the given file.
func New(filePath string, dispatch Dispatcher) (*Scheduler, error) {
	s := &Scheduler{
		filePath: filePath,
		dispatch: dispatch,
	}
	if err := s.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return s, nil
}

// Add adds a new scheduled task.
func (s *Scheduler) Add(task ScheduledTask) error {
	s.mu.Lock()
	s.tasks = append(s.tasks, task)
	s.mu.Unlock()
	slog.Info("task scheduled", "id", task.ID, "run_at", task.RunAt, "cron", task.CronExpr)
	return s.persist()
}

// Remove removes a scheduled task by ID.
func (s *Scheduler) Remove(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, t := range s.tasks {
		if t.ID == id {
			s.tasks = append(s.tasks[:i], s.tasks[i+1:]...)
			s.persist()
			return true
		}
	}
	return false
}

// List returns all scheduled tasks.
func (s *Scheduler) List() []ScheduledTask {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ScheduledTask, len(s.tasks))
	copy(out, s.tasks)
	return out
}

// Run starts the scheduler loop. Checks every 30 seconds for tasks to dispatch.
func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	slog.Info("scheduler running", "tasks_loaded", len(s.tasks))

	for {
		select {
		case <-ctx.Done():
			slog.Info("scheduler stopped")
			return
		case now := <-ticker.C:
			s.checkAndDispatch(ctx, now)
		}
	}
}

func (s *Scheduler) checkAndDispatch(ctx context.Context, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	remaining := make([]ScheduledTask, 0, len(s.tasks))
	dispatched := 0

	for _, task := range s.tasks {
		if task.CronExpr == "" && !now.Before(task.RunAt) {
			// One-time reminder: dispatch and remove.
			go s.dispatch(ctx, task)
			dispatched++
			continue
		}

		if task.CronExpr != "" && cronMatches(task.CronExpr, now) {
			// Cronjob: dispatch but keep in list.
			go s.dispatch(ctx, task)
			dispatched++
		}

		remaining = append(remaining, task)
	}

	if dispatched > 0 {
		s.tasks = remaining
		s.persist()
		slog.Info("scheduler dispatched tasks", "count", dispatched)
	}
}

func (s *Scheduler) persist() error {
	s.mu.RLock()
	data, err := json.MarshalIndent(s.tasks, "", "  ")
	s.mu.RUnlock()
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, data, 0644)
}

func (s *Scheduler) load() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &s.tasks)
}

// cronMatches checks if a simple cron expression matches the given time.
// Supports format: "minute hour day month weekday" (like standard cron).
// Uses * for wildcard. Example: "0 9 * * *" = every day at 9:00.
func cronMatches(expr string, t time.Time) bool {
	var minute, hour, day, month, weekday string
	n, _ := parseFields(expr)
	if n < 5 {
		return false
	}

	parts := splitFields(expr)
	minute = parts[0]
	hour = parts[1]
	day = parts[2]
	month = parts[3]
	weekday = parts[4]

	return fieldMatches(minute, t.Minute()) &&
		fieldMatches(hour, t.Hour()) &&
		fieldMatches(day, t.Day()) &&
		fieldMatches(month, int(t.Month())) &&
		fieldMatches(weekday, int(t.Weekday()))
}

func fieldMatches(field string, value int) bool {
	if field == "*" {
		return true
	}
	var expected int
	if _, err := parseIntFromStr(field, &expected); err == nil {
		return expected == value
	}
	return false
}

func splitFields(expr string) []string {
	fields := make([]string, 0, 5)
	current := ""
	for _, c := range expr {
		if c == ' ' || c == '\t' {
			if current != "" {
				fields = append(fields, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		fields = append(fields, current)
	}
	return fields
}

func parseFields(expr string) (int, error) {
	return len(splitFields(expr)), nil
}

func parseIntFromStr(s string, result *int) (int, error) {
	val := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, os.ErrInvalid
		}
		val = val*10 + int(c-'0')
	}
	*result = val
	return val, nil
}
