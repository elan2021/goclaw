package config

import (
	"os"
	"path/filepath"
)

// DataDir returns the GoClaw data directory (~/.goclaw/).
// It creates the directory tree if it doesn't exist.
func DataDir() string {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".goclaw")
	return dir
}

// EnsureDataDir creates the full directory tree under ~/.goclaw/.
func EnsureDataDir() error {
	base := DataDir()
	dirs := []string{
		base,
		filepath.Join(base, "memory", "notes"),
		filepath.Join(base, "memory", "summaries"),
		filepath.Join(base, "history"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return err
		}
	}

	// Create default SOUL.md if not present.
	soulPath := filepath.Join(base, "SOUL.md")
	if _, err := os.Stat(soulPath); os.IsNotExist(err) {
		defaultSoul := "# Soul\n\nYou are a helpful AI assistant. Be concise and direct.\n"
		_ = os.WriteFile(soulPath, []byte(defaultSoul), 0644)
	}

	// Create default AGENTS.md if not present.
	agentsPath := filepath.Join(base, "AGENTS.md")
	if _, err := os.Stat(agentsPath); os.IsNotExist(err) {
		defaultAgents := "# Agent Instructions\n\nFollow user instructions carefully.\n"
		_ = os.WriteFile(agentsPath, []byte(defaultAgents), 0644)
	}

	// Create default config.toml if not present.
	configPath := filepath.Join(base, "config.toml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		defaultConfig := `[server]
port = 18789

[llm]
default_provider = "openai"

[llm.openai]
api_key = ""
model = "gpt-4o"

[llm.anthropic]
api_key = ""
model = "claude-sonnet-4-20250514"

[llm.ollama]
base_url = "http://localhost:11434"
model = "llama3"

[channels.whatsapp]
enabled = false

[channels.telegram]
enabled = false
bot_token = ""

[jobs]
compress_interval_hours = 6

[agent]
max_react_steps = 10
timeout_seconds = 120
`
		_ = os.WriteFile(configPath, []byte(defaultConfig), 0644)
	}

	return nil
}

// SoulPath returns the path to SOUL.md.
func SoulPath() string {
	return filepath.Join(DataDir(), "SOUL.md")
}

// AgentsPath returns the path to AGENTS.md.
func AgentsPath() string {
	return filepath.Join(DataDir(), "AGENTS.md")
}

// HistoryDir returns the path to the history directory.
func HistoryDir() string {
	return filepath.Join(DataDir(), "history")
}

// MemoryDir returns the path to the memory NOTES directory.
func MemoryDir() string {
	return filepath.Join(DataDir(), "memory", "notes")
}

// SchedulesPath returns the path to schedules.json.
func SchedulesPath() string {
	return filepath.Join(DataDir(), "schedules.json")
}
