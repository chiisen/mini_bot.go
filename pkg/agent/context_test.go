package agent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chiisen/mini_bot/pkg/providers"
)

func TestBuilder_Build_NoFiles(t *testing.T) {
	tmpDir := t.TempDir()
	builder := NewContextBuilder(tmpDir)

	result, err := builder.Build([]providers.ToolDefinition{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != "" {
		t.Errorf("expected empty result, got %s", result)
	}
}

func TestBuilder_Build_WithIdentityFile(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "IDENTITY.md"), []byte("I am a helpful assistant"), 0644)

	builder := NewContextBuilder(tmpDir)
	result, err := builder.Build([]providers.ToolDefinition{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !contains(result, "[IDENTITY]") {
		t.Error("expected [IDENTITY] section in result")
	}
	if !contains(result, "I am a helpful assistant") {
		t.Error("expected identity content in result")
	}
}

func TestBuilder_Build_LoadOrder(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "IDENTITY.md"), []byte("Identity content"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "AGENT.md"), []byte("Agent guidelines"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "SOUL.md"), []byte("Personality traits"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "USER.md"), []byte("User preferences"), 0644)

	builder := NewContextBuilder(tmpDir)
	result, err := builder.Build([]providers.ToolDefinition{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check order: IDENTITY -> AGENT -> SOUL -> USER
	identityPos := indexOf(result, "[IDENTITY]")
	agentPos := indexOf(result, "[AGENT GUIDELINES]")
	soulPos := indexOf(result, "[PERSONALITY]")
	userPos := indexOf(result, "[USER PREFERENCES]")

	if identityPos < 0 || agentPos < 0 || soulPos < 0 || userPos < 0 {
		t.Fatal("missing expected sections")
	}

	if !(identityPos < agentPos && agentPos < soulPos && soulPos < userPos) {
		t.Error("sections should be in correct order")
	}
}

func TestBuilder_Build_WithTools(t *testing.T) {
	tmpDir := t.TempDir()
	builder := NewContextBuilder(tmpDir)

	tools := []providers.ToolDefinition{
		{
			Type: "function",
			Function: providers.ToolFunctionDefinition{
				Name:        "read_file",
				Description: "Read a file",
			},
		},
		{
			Type: "function",
			Function: providers.ToolFunctionDefinition{
				Name:        "exec",
				Description: "Execute a command",
			},
		},
	}

	result, err := builder.Build(tools)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !contains(result, "[AVAILABLE TOOLS]") {
		t.Error("expected [AVAILABLE TOOLS] section")
	}
	if !contains(result, "read_file") {
		t.Error("expected read_file tool description")
	}
	if !contains(result, "exec") {
		t.Error("expected exec tool description")
	}
}

func TestBuilder_Build_WithMemory(t *testing.T) {
	tmpDir := t.TempDir()
	memoryDir := filepath.Join(tmpDir, "memory")
	os.MkdirAll(memoryDir, 0755)
	os.WriteFile(filepath.Join(memoryDir, "MEMORY.md"), []byte("Past memories here"), 0644)

	builder := NewContextBuilder(tmpDir)
	result, err := builder.Build([]providers.ToolDefinition{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !contains(result, "[MEMORY]") {
		t.Error("expected [MEMORY] section")
	}
	if !contains(result, "Past memories here") {
		t.Error("expected memory content")
	}
}

func TestBuilder_Build_MissingFiles(t *testing.T) {
	tmpDir := t.TempDir()
	// Create only some files
	os.WriteFile(filepath.Join(tmpDir, "IDENTITY.md"), []byte("I exist"), 0644)

	builder := NewContextBuilder(tmpDir)
	result, err := builder.Build([]providers.ToolDefinition{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have IDENTITY but not other sections
	if !contains(result, "[IDENTITY]") {
		t.Error("expected [IDENTITY] section")
	}
	if contains(result, "[MEMORY]") {
		t.Error("should not have [MEMORY] section for missing file")
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && indexOf(s, substr) >= 0
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
