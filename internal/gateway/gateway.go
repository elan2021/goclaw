package gateway

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"goclaw/internal/agent"
	"goclaw/internal/channel"
	"goclaw/internal/llm"
	"goclaw/internal/memory"
	"goclaw/internal/skill"
)

// Gateway is the central router that receives messages from all channels
// and dispatches them to per-user sessions.
type Gateway struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	incoming chan channel.IncomingMessage
	channels []channel.MessageChannel

	provider llm.Provider
	registry *skill.Registry
	history  *memory.History
	store    *memory.Store
	agentCfg agent.Config
}

// Session represents an independent goroutine for a user.
type Session struct {
	UserID string
	Cancel context.CancelFunc
	MsgCh  chan channel.IncomingMessage
	Runner *agent.Runner

	// Rate limiting.
	mu                sync.Mutex
	lastReset         time.Time
	messagesSinceLast int
}

// NewGateway creates the central message router.
func NewGateway(provider llm.Provider, registry *skill.Registry, history *memory.History, store *memory.Store, agentCfg agent.Config) *Gateway {
	return &Gateway{
		sessions: make(map[string]*Session),
		incoming: make(chan channel.IncomingMessage, 256),
		provider: provider,
		registry: registry,
		history:  history,
		store:    store,
		agentCfg: agentCfg,
	}
}

var wg sync.WaitGroup

// RegisterChannel adds a message channel (WhatsApp, Telegram).
func (g *Gateway) RegisterChannel(ch channel.MessageChannel) {
	g.channels = append(g.channels, ch)
}

// Run starts all channels and the main dispatch loop.
func (g *Gateway) Run(ctx context.Context) error {
	// Start all registered channels.
	for _, ch := range g.channels {
		ch := ch
		wg.Add(1)
		go func() {
			defer wg.Done()
			slog.Info("starting channel", "name", ch.Name())
			if err := ch.Start(ctx, g.incoming); err != nil {
				slog.Error("channel error", "name", ch.Name(), "error", err)
			}
		}()
	}

	slog.Info("gateway running", "channels", len(g.channels))

	// Main dispatch loop.
	for {
		select {
		case <-ctx.Done():
			g.shutdownAll()
			wg.Wait() // Wait for channels to stop.
			return ctx.Err()
		case msg := <-g.incoming:
			g.dispatch(ctx, msg)
		}
	}
}

// dispatch finds or creates a session and sends the message.
func (g *Gateway) dispatch(ctx context.Context, msg channel.IncomingMessage) {
	g.mu.Lock()
	sess, exists := g.sessions[msg.UserID]
	if !exists {
		sess = g.createSession(ctx, msg)
		g.sessions[msg.UserID] = sess
	}
	g.mu.Unlock()

	// Rate limit check.
	if g.isRateLimited(sess) {
		slog.Warn("user rate limited", "user_id", msg.UserID)
		g.SendProactive(ctx, channel.OutgoingMessage{
			Platform: msg.Platform,
			ChatID:   msg.ChatID,
			Text:     "⚠️ Você está enviando mensagens rápido demais. Por favor, aguarde um momento.",
		})
		return
	}

	// Non-blocking send.
	select {
	case sess.MsgCh <- msg:
	default:
		slog.Warn("session channel full, dropping message", "user_id", msg.UserID)
	}
}

func (g *Gateway) isRateLimited(s *Session) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	if now.Sub(s.lastReset) > time.Minute {
		s.lastReset = now
		s.messagesSinceLast = 0
	}

	s.messagesSinceLast++
	return s.messagesSinceLast > 10 // Max 10 msgs per minute.
}

// createSession starts an independent goroutine for a user.
func (g *Gateway) createSession(parentCtx context.Context, msg channel.IncomingMessage) *Session {
	ctx, cancel := context.WithCancel(parentCtx)

	sendFunc := g.buildSendFunc(msg.Platform)

	runner := agent.NewRunner(g.agentCfg, g.provider, g.registry, g.history, g.store, msg.UserID, sendFunc)

	sess := &Session{
		UserID: msg.UserID,
		Cancel: cancel,
		MsgCh:  make(chan channel.IncomingMessage, 32),
		Runner: runner,
	}

	go sess.Runner.Listen(ctx, sess.MsgCh)

	slog.Info("session created", "user_id", msg.UserID, "platform", msg.Platform)
	return sess
}

// buildSendFunc creates a send function that routes to the correct channel.
func (g *Gateway) buildSendFunc(platform string) func(ctx context.Context, msg channel.OutgoingMessage) error {
	return func(ctx context.Context, msg channel.OutgoingMessage) error {
		for _, ch := range g.channels {
			if ch.Name() == msg.Platform || ch.Name() == platform {
				return ch.Send(ctx, msg)
			}
		}
		slog.Error("no channel found for platform", "platform", msg.Platform)
		return nil
	}
}

// KillSession cancels a user's session goroutine (kill-switch).
func (g *Gateway) KillSession(userID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if sess, ok := g.sessions[userID]; ok {
		slog.Info("killing session", "user_id", userID)
		sess.Cancel()
		close(sess.MsgCh)
		delete(g.sessions, userID)
	}
}

// ActiveSessions returns the number of active sessions.
func (g *Gateway) ActiveSessions() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.sessions)
}

// SessionIDs returns the IDs of all active sessions.
func (g *Gateway) SessionIDs() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	ids := make([]string, 0, len(g.sessions))
	for id := range g.sessions {
		ids = append(ids, id)
	}
	return ids
}

// SendProactive sends a message to a user without an incoming trigger.
// Used by the scheduler for reminders and cronjobs.
func (g *Gateway) SendProactive(ctx context.Context, msg channel.OutgoingMessage) error {
	for _, ch := range g.channels {
		if ch.Name() == msg.Platform {
			return ch.Send(ctx, msg)
		}
	}
	// Fallback: try all channels.
	for _, ch := range g.channels {
		if err := ch.Send(ctx, msg); err == nil {
			return nil
		}
	}
	return nil
}

func (g *Gateway) shutdownAll() {
	g.mu.Lock()
	defer g.mu.Unlock()

	slog.Info("shutting down all sessions", "count", len(g.sessions))
	for id, sess := range g.sessions {
		sess.Cancel()
		delete(g.sessions, id)
	}

	for _, ch := range g.channels {
		if err := ch.Stop(); err != nil {
			slog.Error("channel stop error", "name", ch.Name(), "error", err)
		}
	}
}
