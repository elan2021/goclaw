package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"goclaw/internal/channel"
	"goclaw/internal/llm"
	"goclaw/internal/memory"
	"goclaw/internal/skill"
)

// Config holds agent-level settings.
type Config struct {
	MaxReActSteps  int
	TimeoutSeconds int
}

// Runner is the brain of the agent. One Runner per user session.
type Runner struct {
	provider llm.Provider
	registry *skill.Registry
	history  *memory.History
	store    *memory.Store
	sendFunc func(ctx context.Context, msg channel.OutgoingMessage) error
	userID   string
	cfg      Config
}

// NewRunner creates a new agent runner for a user session.
func NewRunner(cfg Config, provider llm.Provider, registry *skill.Registry, history *memory.History, store *memory.Store, userID string, sendFunc func(ctx context.Context, msg channel.OutgoingMessage) error) *Runner {
	if cfg.MaxReActSteps == 0 {
		cfg.MaxReActSteps = 10
	}
	if cfg.TimeoutSeconds == 0 {
		cfg.TimeoutSeconds = 120
	}
	return &Runner{
		provider: provider,
		registry: registry,
		history:  history,
		store:    store,
		sendFunc: sendFunc,
		userID:   userID,
		cfg:      cfg,
	}
}

// Listen is the main goroutine loop for a user session.
func (r *Runner) Listen(ctx context.Context, msgCh <-chan channel.IncomingMessage) {
	slog.Info("agent session started", "user_id", r.userID)
	defer slog.Info("agent session ended", "user_id", r.userID)

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-msgCh:
			if !ok {
				return
			}
			r.handleMessage(ctx, msg)
		}
	}
}

// handleMessage runs the ReAct loop for a single user message.
func (r *Runner) handleMessage(ctx context.Context, msg channel.IncomingMessage) {
	// Safety timeout for the entire interaction.
	ctx, cancel := context.WithTimeout(ctx, time.Duration(r.cfg.TimeoutSeconds)*time.Second)
	defer cancel()

	slog.Info("processing message", "user_id", r.userID, "text_len", len(msg.Text))

	// Build conversation: system prompt + history + new message.
	systemPrompt := buildSystemPrompt(r.store, r.userID, msg.Text)
	messages := []llm.Message{
		llm.Msg("system", systemPrompt),
	}

	// Append recent history.
	historyMsgs := r.history.Recent(r.userID, 20)
	messages = append(messages, historyMsgs...)

	// Append the new user message.
	messages = append(messages, llm.Msg("user", msg.Text))
	r.history.Append(r.userID, "user", msg.Text)

	// Build tool specs from registry.
	toolSpecs := buildToolSpecs(r.registry.Definitions())

	// ReAct loop.
	for step := 0; step < r.cfg.MaxReActSteps; step++ {
		select {
		case <-ctx.Done():
			r.reply(ctx, msg, "⏱️ Tempo esgotado. Operação cancelada.")
			return
		default:
		}

		resp, err := r.provider.ChatCompletion(ctx, llm.Request{
			Messages: messages,
			Tools:    toolSpecs,
		})
		if err != nil {
			slog.Error("llm call failed", "error", err, "step", step)
			r.reply(ctx, msg, fmt.Sprintf("❌ Erro ao contactar o LLM: %v", err))
			return
		}

		// Case 1: LLM returned text → reply and finish.
		if resp.Content != "" && len(resp.ToolCalls) == 0 {
			r.history.Append(r.userID, "assistant", resp.Content)
			r.reply(ctx, msg, resp.Content)
			return
		}

		// Case 2: LLM requested tool calls.
		if len(resp.ToolCalls) > 0 {
			// Add assistant message with tool calls marker.
			messages = append(messages, llm.Message{
				Role:    "assistant",
				Content: resp.Content,
			})

			for _, tc := range resp.ToolCalls {
				slog.Info("executing tool", "name", tc.Name, "step", step)

				toolResult := r.executeTool(ctx, tc)

				messages = append(messages, llm.ToolMsg(tc.ID, toolResult))
			}
			continue
		}

		// Case 3: Empty response — shouldn't happen, but handle gracefully.
		slog.Warn("empty response from LLM", "step", step)
		r.reply(ctx, msg, "🤔 Não recebi uma resposta do modelo. Tente novamente.")
		return
	}

	// If we exit the loop, the agent hit max steps.
	r.history.Append(r.userID, "assistant", "⚠️ Limite de passos atingido.")
	r.reply(ctx, msg, "⚠️ Limite de passos atingido. Operação interrompida para segurança.")
}

// executeTool runs a single tool and returns the observation text.
func (r *Runner) executeTool(ctx context.Context, tc llm.ToolCallRequest) string {
	// Parse arguments from JSON string.
	var args map[string]string
	if err := json.Unmarshal([]byte(tc.Arguments), &args); err != nil {
		return fmt.Sprintf("Error parsing tool arguments: %v", err)
	}

	tool, ok := r.registry.Get(tc.Name)
	if !ok {
		return fmt.Sprintf("Tool '%s' not found. Available tools: %v", tc.Name, toolNames(r.registry))
	}

	result, err := tool.Execute(ctx, args)
	if err != nil {
		return fmt.Sprintf("Tool '%s' failed with error: %v", tc.Name, err)
	}

	if result.Error != "" {
		return fmt.Sprintf("Tool '%s' error: %s", tc.Name, result.Error)
	}

	return result.Output
}

func (r *Runner) reply(ctx context.Context, msg channel.IncomingMessage, text string) {
	outMsg := channel.OutgoingMessage{
		ChatID:   msg.ChatID,
		Text:     text,
		Platform: msg.Platform,
	}
	if err := r.sendFunc(ctx, outMsg); err != nil {
		slog.Error("failed to send reply", "error", err, "user_id", r.userID)
	}
}

func buildToolSpecs(defs []skill.ToolDefinition) []llm.ToolSpec {
	specs := make([]llm.ToolSpec, 0, len(defs))
	for _, d := range defs {
		params := map[string]interface{}{
			"type":       "object",
			"properties": d.Parameters,
			"required":   d.Required,
		}
		specs = append(specs, llm.ToolSpec{
			Type: "function",
			Function: llm.FunctionSpec{
				Name:        d.Name,
				Description: d.Description,
				Parameters:  params,
			},
		})
	}
	return specs
}

func toolNames(reg *skill.Registry) []string {
	names := make([]string, 0)
	for _, d := range reg.Definitions() {
		names = append(names, d.Name)
	}
	return names
}
