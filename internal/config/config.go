package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config is the root configuration loaded from config.toml.
type Config struct {
	Server   ServerConfig   `toml:"server"`
	LLM      LLMConfig      `toml:"llm"`
	Channels ChannelsConfig `toml:"channels"`
	Jobs     JobsConfig     `toml:"jobs"`
	Agent    AgentConfig    `toml:"agent"`
}

type ServerConfig struct {
	Port int `toml:"port"`
}

type LLMConfig struct {
	DefaultProvider string          `toml:"default_provider"`
	OpenAI          OpenAIConfig    `toml:"openai"`
	Anthropic       AnthropicConfig `toml:"anthropic"`
	Ollama          OllamaConfig    `toml:"ollama"`
}

type OpenAIConfig struct {
	APIKey string `toml:"api_key"`
	Model  string `toml:"model"`
}

type AnthropicConfig struct {
	APIKey string `toml:"api_key"`
	Model  string `toml:"model"`
}

type OllamaConfig struct {
	BaseURL string `toml:"base_url"`
	Model   string `toml:"model"`
}

type ChannelsConfig struct {
	WhatsApp WhatsAppConfig `toml:"whatsapp"`
	Telegram TelegramConfig `toml:"telegram"`
}

type WhatsAppConfig struct {
	Enabled bool `toml:"enabled"`
}

type TelegramConfig struct {
	Enabled  bool   `toml:"enabled"`
	BotToken string `toml:"bot_token"`
}

type JobsConfig struct {
	CompressIntervalHours int `toml:"compress_interval_hours"`
}

type AgentConfig struct {
	MaxReActSteps  int `toml:"max_react_steps"`
	TimeoutSeconds int `toml:"timeout_seconds"`
}

// Load reads config.toml from the GoClaw data directory.
func Load() (*Config, error) {
	dataDir := DataDir()
	configPath := filepath.Join(dataDir, "config.toml")

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	cfg := &Config{}
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	applyDefaults(cfg)
	return cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 18789
	}
	if cfg.LLM.DefaultProvider == "" {
		cfg.LLM.DefaultProvider = "openai"
	}
	if cfg.LLM.OpenAI.Model == "" {
		cfg.LLM.OpenAI.Model = "gpt-4o"
	}
	if cfg.LLM.Anthropic.Model == "" {
		cfg.LLM.Anthropic.Model = "claude-sonnet-4-20250514"
	}
	if cfg.LLM.Ollama.BaseURL == "" {
		cfg.LLM.Ollama.BaseURL = "http://localhost:11434"
	}
	if cfg.LLM.Ollama.Model == "" {
		cfg.LLM.Ollama.Model = "llama3"
	}
	if cfg.Jobs.CompressIntervalHours == 0 {
		cfg.Jobs.CompressIntervalHours = 6
	}
	if cfg.Agent.MaxReActSteps == 0 {
		cfg.Agent.MaxReActSteps = 10
	}
	if cfg.Agent.TimeoutSeconds == 0 {
		cfg.Agent.TimeoutSeconds = 120
	}
}
