package channel

import "context"

// IncomingMessage represents a message received from any platform.
type IncomingMessage struct {
	Platform  string // "whatsapp" | "telegram"
	UserID    string // Unique user identifier on the platform
	ChatID    string // Chat/group identifier
	Text      string
	Timestamp int64
	Metadata  map[string]string // Extra data (name, media, etc.)
}

// OutgoingMessage represents a response to send back to the user.
type OutgoingMessage struct {
	ChatID   string
	Text     string
	ReplyTo  string // Original message ID (optional)
	Platform string // Target platform for proactive messages
}

// MessageChannel is the interface every platform adapter must implement.
type MessageChannel interface {
	// Name returns the platform identifier ("whatsapp", "telegram").
	Name() string

	// Start initializes the connection and begins listening for messages.
	// Messages are sent to the `incoming` channel until ctx is cancelled.
	Start(ctx context.Context, incoming chan<- IncomingMessage) error

	// Send sends a message back to the platform.
	Send(ctx context.Context, msg OutgoingMessage) error

	// Stop gracefully shuts down the connection.
	Stop() error
}
