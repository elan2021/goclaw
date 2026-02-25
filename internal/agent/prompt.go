package agent

import (
	"os"

	"goclaw/internal/config"
	"goclaw/internal/memory"
)

// buildSystemPrompt reads SOUL.md and AGENTS.md, searches memory notes, and constructs the system prompt.
func buildSystemPrompt(store *memory.Store, userID, query string) string {
	soulPath := config.SoulPath()
	soul := readFileOrDefault(soulPath, "You are a helpful AI assistant.")
	agents := readFileOrDefault(config.AgentsPath(), "")

	prompt := soul
	if agents != "" {
		prompt += "\n\n---\n\n" + agents
	}

	// Check for Onboarding: If Soul is default, instruct the agent to onboard.
	isDefaultSoul := soul == "# Soul\n\nYou are a helpful AI assistant. Be concise and direct.\n"
	if isDefaultSoul {
		prompt += "\n\n[ONBOARDING MODE ACTIVE]\n"
		prompt += "This is the user's first interaction. You MUST follow these steps:\n"
		prompt += "1. Introduce yourself as GoClaw.\n"
		prompt += "2. Ask the user's name and how they want to be called.\n"
		prompt += "3. Ask about their main goals for using GoClaw.\n"
		prompt += "4. Explain that you will save these preferences to your 'Soul' so you can remember them.\n"
		prompt += "5. Once learned, use your tools to update SOUL.md with this new identity.\n"
	}

	// Memory RAG-lite: search and inject relevant notes.
	if store != nil && query != "" {
		notes, _ := store.SearchNotes(query)
		if notes != "" {
			prompt += "\n\n---\n\nRELEVANT MEMORY NOTES (Long-term):\n" + notes
		}
	}

	prompt += "\n\n---\n\nIMPORTANT RULES:\n"
	prompt += "- You have access to tools. Use them when needed to accomplish tasks.\n"
	prompt += "- Always respond in the user's language (default to Portuguese if unclear).\n"
	prompt += "- Be concise and direct.\n"
	prompt += "- When using tools, explain what you're doing briefly.\n"
	prompt += "- If a tool fails, explain the error and try an alternative approach.\n"
	prompt += "- You can update your own SOUL.md and AGENTS.md using filesystem tools.\n"

	return prompt
}

func readFileOrDefault(path, fallback string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return fallback
	}
	return string(data)
}
