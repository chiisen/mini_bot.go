package agent

import (
	"context"
	"testing"

	"github.com/chiisen/mini_bot/pkg/providers"
)

type mockProvider struct {
	responses []providers.LLMResponse
	callCount int
}

func (m *mockProvider) Chat(
	ctx context.Context,
	messages []providers.Message,
	tools []providers.ToolDefinition,
	model string,
	options map[string]any,
) (*providers.LLMResponse, error) {
	if m.callCount >= len(m.responses) {
		return &providers.LLMResponse{Content: "No more responses"}, nil
	}
	resp := m.responses[m.callCount]
	m.callCount++
	return &resp, nil
}

func (m *mockProvider) GetDefaultModel() string {
	return "mock-model"
}

func TestRun_NoTools(t *testing.T) {
	// This test would require setting up a full AgentInstance with mocks
	// For now, we'll test the loop logic indirectly through integration tests
	t.Skip("Requires full AgentInstance setup with mocks")
}

func TestNewContextBuilder(t *testing.T) {
	tmpDir := t.TempDir()
	builder := NewContextBuilder(tmpDir)

	if builder.WorkspacePath != tmpDir {
		t.Errorf("expected workspace path %s, got %s", tmpDir, builder.WorkspacePath)
	}
}

func TestNewMemoryManager(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewMemoryManager(tmpDir)

	if m.MemoryPath != tmpDir {
		t.Errorf("expected memory path %s, got %s", tmpDir, m.MemoryPath)
	}
}
