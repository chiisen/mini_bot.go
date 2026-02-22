package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/chiisen/mini_bot/pkg/providers"
)

type Manager struct {
	StorageDir string
}

func NewManager(storageDir string) *Manager {
	return &Manager{
		StorageDir: storageDir,
	}
}

// Load reads session message history from JSON.
// If the session does not exist, it returns an empty slice without an error.
func (m *Manager) Load(sessionKey string) ([]providers.Message, error) {
	path := filepath.Join(m.StorageDir, sessionKey+".json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return []providers.Message{}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read session %s: %w", sessionKey, err)
	}

	var messages []providers.Message
	if err := json.Unmarshal(data, &messages); err != nil {
		return nil, fmt.Errorf("failed to parse session %s: %w", sessionKey, err)
	}

	return messages, nil
}

// Save writes session message history to JSON.
func (m *Manager) Save(sessionKey string, messages []providers.Message) error {
	path := filepath.Join(m.StorageDir, sessionKey+".json")
	data, err := json.MarshalIndent(messages, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize session %s: %w", sessionKey, err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write session %s: %w", sessionKey, err)
	}

	return nil
}

// Compress applies context window compression if messages exceed maxTokens.
// For now, it aggressively keeps the first system message and the last 10 messages.
func (m *Manager) Compress(messages []providers.Message, maxTokens int) []providers.Message {
	// Simple rule-based compression logic for MVP
	if len(messages) <= 12 {
		return messages
	}

	var compressed []providers.Message
	// Always keep the system prompt (assume it's the first message)
	if len(messages) > 0 && messages[0].Role == "system" {
		compressed = append(compressed, messages[0])
		messages = messages[1:]
	}

	// Keep last 10
	tailSize := 10
	if len(messages) < tailSize {
		tailSize = len(messages)
	}
	
	compressed = append(compressed, messages[len(messages)-tailSize:]...)
	return compressed
}
