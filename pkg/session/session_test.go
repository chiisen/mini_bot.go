package session

import (
	"testing"

	"github.com/chiisen/mini_bot/pkg/providers"
)

func TestManager_Load_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	messages, err := m.Load("nonexistent-session")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(messages) != 0 {
		t.Errorf("expected empty messages, got %d", len(messages))
	}
}

func TestManager_SaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	messages := []providers.Message{
		{Role: "system", Content: "You are a helpful assistant"},
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
	}

	sessionKey := "test-session"
	if err := m.Save(sessionKey, messages); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	loaded, err := m.Load(sessionKey)
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	if len(loaded) != len(messages) {
		t.Errorf("expected %d messages, got %d", len(messages), len(loaded))
	}

	for i, msg := range loaded {
		if msg.Role != messages[i].Role {
			t.Errorf("message[%d] role: expected %s, got %s", i, messages[i].Role, msg.Role)
		}
		if msg.Content != messages[i].Content {
			t.Errorf("message[%d] content: expected %s, got %s", i, messages[i].Content, msg.Content)
		}
	}
}

func TestManager_Save_Overwrites(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	sessionKey := "test-session"

	// Save first session
	messages1 := []providers.Message{
		{Role: "user", Content: "First message"},
	}
	if err := m.Save(sessionKey, messages1); err != nil {
		t.Fatalf("failed to save first: %v", err)
	}

	// Save second session
	messages2 := []providers.Message{
		{Role: "user", Content: "Second message"},
		{Role: "assistant", Content: "Response"},
	}
	if err := m.Save(sessionKey, messages2); err != nil {
		t.Fatalf("failed to save second: %v", err)
	}

	// Load should have second session
	loaded, err := m.Load(sessionKey)
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	if len(loaded) != 2 {
		t.Errorf("expected 2 messages, got %d", len(loaded))
	}

	if loaded[0].Content != "Second message" {
		t.Errorf("expected first content Second message, got %s", loaded[0].Content)
	}
}

func TestManager_Compress_NoChange(t *testing.T) {
	m := NewManager("/tmp")

	// Test with fewer than 12 messages - should not compress
	messages := []providers.Message{
		{Role: "system", Content: "System prompt"},
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi"},
	}

	compressed := m.Compress(messages, 1000)
	if len(compressed) != len(messages) {
		t.Errorf("expected %d messages, got %d", len(messages), len(compressed))
	}
}

func TestManager_Compress_Aggressive(t *testing.T) {
	m := NewManager("/tmp")

	// Create more than 12 messages
	messages := make([]providers.Message, 20)
	messages[0] = providers.Message{Role: "system", Content: "System prompt"}
	for i := 1; i < 20; i++ {
		messages[i] = providers.Message{Role: "user", Content: "Message"}
	}

	compressed := m.Compress(messages, 1000)

	// Should keep system message and last 10 messages = 11 total
	if len(compressed) != 11 {
		t.Errorf("expected 11 messages after compression, got %d", len(compressed))
	}

	// First should be system
	if compressed[0].Role != "system" {
		t.Errorf("expected first message to be system, got %s", compressed[0].Role)
	}

	// Last should be the last original message
	if compressed[len(compressed)-1].Content != "Message" {
		t.Errorf("expected last message to be Message, got %s", compressed[len(compressed)-1].Content)
	}
}

func TestManager_Compress_NoSystemMessage(t *testing.T) {
	m := NewManager("/tmp")

	// No system message
	messages := make([]providers.Message, 15)
	for i := 0; i < 15; i++ {
		messages[i] = providers.Message{Role: "user", Content: "Message " + string(rune('0'+i))}
	}

	compressed := m.Compress(messages, 1000)

	// Should keep last 10
	if len(compressed) != 10 {
		t.Errorf("expected 10 messages after compression, got %d", len(compressed))
	}
}
