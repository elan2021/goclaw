package whatsapp

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"goclaw/internal/channel"
	"goclaw/internal/config"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"

	_ "github.com/mattn/go-sqlite3"
	"google.golang.org/protobuf/proto"
)

// waLogger implements waLog.Logger.
type waLogger struct {
	module string
}

func (l *waLogger) Errorf(msg string, args ...interface{}) {
	slog.Error(fmt.Sprintf(msg, args...), "module", l.module)
}
func (l *waLogger) Warnf(msg string, args ...interface{}) {
	slog.Warn(fmt.Sprintf(msg, args...), "module", l.module)
}
func (l *waLogger) Infof(msg string, args ...interface{}) {
	slog.Info(fmt.Sprintf(msg, args...), "module", l.module)
}
func (l *waLogger) Debugf(msg string, args ...interface{}) {
	slog.Debug(fmt.Sprintf(msg, args...), "module", l.module)
}
func (l *waLogger) Sub(module string) waLog.Logger { return &waLogger{module: l.module + ":" + module} }

// Compile-time check.
var _ waLog.Logger = (*waLogger)(nil)

// Adapter implements channel.MessageChannel.
type Adapter struct {
	client   *whatsmeow.Client
	incoming chan<- channel.IncomingMessage
}

func New() *Adapter             { return &Adapter{} }
func (a *Adapter) Name() string { return "whatsapp" }

func (a *Adapter) Start(ctx context.Context, incoming chan<- channel.IncomingMessage) error {
	a.incoming = incoming
	dbPath := filepath.Join(config.DataDir(), "whatsapp.db")

	// Create logger.
	logger := &waLogger{module: "WhatsApp"}

	// Use context in New.
	container, err := sqlstore.New(ctx, "sqlite3", fmt.Sprintf("file:%s?_foreign_keys=on", dbPath), logger)
	if err != nil {
		return fmt.Errorf("whatsapp db init: %w", err)
	}

	// Use context in GetFirstDevice.
	deviceStore, err := container.GetFirstDevice(ctx)
	if err != nil {
		return fmt.Errorf("whatsapp device store: %w", err)
	}

	client := whatsmeow.NewClient(deviceStore, logger)
	a.client = client
	client.AddEventHandler(a.eventHandler)

	if client.Store.ID == nil {
		qrChan, _ := client.GetQRChannel(ctx)
		err = client.Connect()
		if err != nil {
			return err
		}

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case evt, ok := <-qrChan:
				if !ok {
					return nil
				}
				if evt.Event == "code" {
					fmt.Fprintf(os.Stderr, "\n🔗 WhatsApp QR Code:\n%s\n\n", evt.Code)
				}
			}
		}
	} else {
		err = client.Connect()
		if err != nil {
			return err
		}
	}

	slog.Info("whatsapp connected")
	<-ctx.Done()
	return nil
}

func (a *Adapter) eventHandler(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		if v.Message == nil {
			return
		}
		var text string
		if v.Message.Conversation != nil {
			text = v.Message.GetConversation()
		} else if v.Message.ExtendedTextMessage != nil {
			text = v.Message.ExtendedTextMessage.GetText()
		}
		if text == "" {
			return
		}

		msg := channel.IncomingMessage{
			Platform:  "whatsapp",
			UserID:    v.Info.Sender.User,
			ChatID:    v.Info.Chat.String(),
			Text:      text,
			Timestamp: v.Info.Timestamp.Unix(),
		}
		select {
		case a.incoming <- msg:
		default:
		}
	}
}

func (a *Adapter) Send(ctx context.Context, msg channel.OutgoingMessage) error {
	if a.client == nil {
		return fmt.Errorf("whatsapp client not connected")
	}
	jid, err := types.ParseJID(msg.ChatID)
	if err != nil {
		return err
	}
	_, err = a.client.SendMessage(ctx, jid, &waE2E.Message{
		Conversation: proto.String(msg.Text),
	})
	return err
}

func (a *Adapter) Stop() error {
	if a.client != nil {
		a.client.Disconnect()
	}
	return nil
}
