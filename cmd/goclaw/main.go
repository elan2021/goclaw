package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"goclaw/internal/agent"
	"goclaw/internal/channel/telegram"
	"goclaw/internal/channel/whatsapp"
	"goclaw/internal/config"
	"goclaw/internal/gateway"
	"goclaw/internal/jobs"
	"goclaw/internal/llm"
	"goclaw/internal/llm/anthropic"
	"goclaw/internal/llm/ollama"
	"goclaw/internal/llm/openai"
	"goclaw/internal/memory"
	"goclaw/internal/scheduler"
	"goclaw/internal/skill"
	"goclaw/internal/skill/browser"
	"goclaw/internal/skill/cronjob"
	"goclaw/internal/skill/filesystem"
	memoryskill "goclaw/internal/skill/memory"
	"goclaw/internal/skill/reminder"
	"goclaw/internal/skill/terminal"
	"goclaw/internal/web"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	slog.Info("goclaw starting")

	// Ensure data directory exists.
	if err := config.EnsureDataDir(); err != nil {
		slog.Error("failed to create data directory", "error", err)
		os.Exit(1)
	}

	// Load configuration.
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Initialize LLM provider.
	provider := initProvider(cfg)
	slog.Info("llm provider initialized", "provider", provider.Name())

	// Initialize scheduler.
	sched := initScheduler(cfg)

	// Initialize skill registry.
	registry := skill.NewRegistry()
	registry.Register(&filesystem.ReadFile{})
	registry.Register(&filesystem.WriteFile{})
	registry.Register(&filesystem.ListDir{})
	registry.Register(&terminal.Exec{})
	registry.Register(&browser.Navigate{})
	registry.Register(&memoryskill.Recall{})
	registry.Register(&memoryskill.SaveNote{})
	registry.Register(&reminder.SetReminder{Scheduler: sched})
	registry.Register(&cronjob.SetCronjob{Scheduler: sched})
	registry.Register(&cronjob.ListCronjobs{Scheduler: sched})

	slog.Info("skills registered", "count", registry.Count())

	// Initialize memory.
	history := memory.NewHistory(config.HistoryDir())
	store := memory.NewStore(config.MemoryDir())

	// Initialize background jobs.
	compressor := memory.NewCompressor(provider, config.HistoryDir())
	jobsRunner := jobs.NewRunner(compressor, time.Duration(cfg.Jobs.CompressIntervalHours)*time.Hour)

	// Initialize agent config.
	agentCfg := agent.Config{
		MaxReActSteps:  cfg.Agent.MaxReActSteps,
		TimeoutSeconds: cfg.Agent.TimeoutSeconds,
	}

	// Initialize gateway.
	gw := gateway.NewGateway(provider, registry, history, store, agentCfg)

	// Register channels.
	registerChannels(gw, cfg)

	// Context with signal handling.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Initialize web server.
	webSrv := web.New(cfg.Server.Port, gw, sched)

	slog.Info("goclaw ready",
		"port", cfg.Server.Port,
		"provider", provider.Name(),
		"skills", registry.Count(),
	)

	// Start all components.
	go sched.Run(ctx)
	go jobsRunner.Start(ctx)
	go func() {
		if err := webSrv.Start(ctx); err != nil {
			slog.Error("web server error", "error", err)
		}
	}()

	// Run gateway (blocking).
	if err := gw.Run(ctx); err != nil && err != context.Canceled {
		slog.Error("gateway error", "error", err)
		os.Exit(1)
	}

	slog.Info("goclaw shutdown complete")
}

func initProvider(cfg *config.Config) llm.Provider {
	switch cfg.LLM.DefaultProvider {
	case "openai":
		if cfg.LLM.OpenAI.APIKey == "" {
			slog.Error("OpenAI API key not configured. Edit ~/.goclaw/config.toml")
			os.Exit(1)
		}
		return openai.New(cfg.LLM.OpenAI.APIKey, cfg.LLM.OpenAI.Model)

	case "anthropic":
		if cfg.LLM.Anthropic.APIKey == "" {
			slog.Error("Anthropic API key not configured. Edit ~/.goclaw/config.toml")
			os.Exit(1)
		}
		return anthropic.New(cfg.LLM.Anthropic.APIKey, cfg.LLM.Anthropic.Model)

	case "ollama":
		return ollama.New(cfg.LLM.Ollama.BaseURL, cfg.LLM.Ollama.Model)

	default:
		slog.Error("unsupported LLM provider", "provider", cfg.LLM.DefaultProvider)
		os.Exit(1)
		return nil
	}
}

func initScheduler(cfg *config.Config) *scheduler.Scheduler {
	// The dispatcher will be connected to the gateway after both are created.
	// For now, create with a placeholder dispatcher.
	sched, err := scheduler.New(config.SchedulesPath(), func(ctx context.Context, task scheduler.ScheduledTask) {
		// This dispatcher sends proactive messages via the gateway.
		// It will be wired properly when the web panel is added.
		slog.Info("scheduler dispatch", "id", task.ID, "message", task.Message)
	})
	if err != nil {
		slog.Error("failed to init scheduler", "error", err)
		os.Exit(1)
	}
	return sched
}

func registerChannels(gw *gateway.Gateway, cfg *config.Config) {
	registered := 0

	if cfg.Channels.Telegram.Enabled {
		if cfg.Channels.Telegram.BotToken == "" {
			slog.Error("Telegram bot token not configured")
			os.Exit(1)
		}
		gw.RegisterChannel(telegram.New(cfg.Channels.Telegram.BotToken))
		slog.Info("channel registered", "name", "telegram")
		registered++
	}

	if cfg.Channels.WhatsApp.Enabled {
		gw.RegisterChannel(whatsapp.New())
		slog.Info("channel registered", "name", "whatsapp")
		registered++
	}

	if registered == 0 {
		slog.Error("no channels enabled. Edit ~/.goclaw/config.toml")
		os.Exit(1)
	}
}
