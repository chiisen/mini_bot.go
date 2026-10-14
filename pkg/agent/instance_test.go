package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chiisen/mini_bot/pkg/config"
)

// ============================================================================
// 接線驗證
// ============================================================================

// TestNewInstance_RegistersMemoryWriteTool 驗證長期記憶的寫入端真的有接上。
//
// 加入之前，MEMORY.md 只會被 ContextBuilder 讀進系統提示詞，專案裡沒有任何地方
// 寫入它——長期記憶是一條空管線。
func TestNewInstance_RegistersMemoryWriteTool(t *testing.T) {
	loadTestTranslations(t)

	workspace := t.TempDir()
	cfg := &config.Config{}
	cfg.Agents.Defaults.Workspace = workspace
	cfg.Agents.Defaults.Model = "openai/gpt-4o-mini"
	cfg.Agents.Defaults.MaxTokens = 1024
	cfg.Agents.Defaults.MaxToolIterations = 3
	cfg.Providers = map[string]config.ModelConfig{
		"openai": {APIKey: "test-key", APIBase: "https://api.openai.com/v1"},
	}

	instance, err := NewInstance(cfg)
	if err != nil {
		t.Fatalf("NewInstance failed: %v", err)
	}

	var names []string
	for _, def := range instance.Registry.Definitions() {
		names = append(names, def.Function.Name)
	}
	if !containsString(names, "memory_write") {
		t.Fatalf("expected memory_write to be registered, got %v", names)
	}

	// 透過註冊表真的執行一次，確認寫入落在 workspace/memory/MEMORY.md
	result := instance.Registry.Execute(context.Background(), "memory_write", map[string]any{"content": "偏好繁體中文"})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.ForLLM)
	}

	path := filepath.Join(workspace, "memory", "MEMORY.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected the memory file at %s: %v", path, err)
	}
	if !strings.Contains(string(data), "偏好繁體中文") {
		t.Errorf("unexpected memory content: %q", string(data))
	}
}

func containsString(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}
