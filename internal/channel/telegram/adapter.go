package telegram

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"goclaw/internal/channel"

	tele "gopkg.in/telebot.v3"
)

// Adapter implements channel.MessageChannel for Telegram.
type Adapter struct {
	bot   *tele.Bot
	token string
}

// New creates a Telegram adapter.
func New(token string) *Adapter {
	return &Adapter{token: token}
}

func (a *Adapter) Name() string { return "telegram" }

// Start initializes the Telegram bot and begins listening.
func (a *Adapter) Start(ctx context.Context, incoming chan<- channel.IncomingMessage) error {
	pref := tele.Settings{
		Token:  a.token,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	bot, err := tele.NewBot(pref)
	if err != nil {
		return err
	}
	a.bot = bot

	bot.Handle(tele.OnText, func(c tele.Context) error {
		msg := channel.IncomingMessage{
			Platform:  "telegram",
			UserID:    strconv.FormatInt(c.Sender().ID, 10),
			ChatID:    strconv.FormatInt(c.Chat().ID, 10),
			Text:      c.Text(),
			Timestamp: time.Now().Unix(),
			Metadata: map[string]string{
				"first_name": c.Sender().FirstName,
				"last_name":  c.Sender().LastName,
				"username":   c.Sender().Username,
			},
		}

		select {
		case incoming <- msg:
		default:
			slog.Warn("incoming channel full, dropping telegram message")
		}
		return nil
	})

	slog.Info("telegram bot starting", "username", bot.Me.Username)

	// Run bot in a goroutine and stop on context cancellation.
	go func() {
		<-ctx.Done()
		slog.Info("telegram bot stopping")
		bot.Stop()
	}()

	bot.Start()
	return nil
}

// Send sends a message back to Telegram.
func (a *Adapter) Send(ctx context.Context, msg channel.OutgoingMessage) error {
	if a.bot == nil {
		return nil
	}

	chatID, err := strconv.ParseInt(msg.ChatID, 10, 64)
	if err != nil {
		return err
	}

	chat := &tele.Chat{ID: chatID}
	_, err = a.bot.Send(chat, msg.Text)
	return err
}

// Stop gracefully shuts down the Telegram bot.
func (a *Adapter) Stop() error {
	if a.bot != nil {
		a.bot.Stop()
	}
	return nil
}
