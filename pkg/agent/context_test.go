package agent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chiisen/mini_bot/pkg/i18n"
	"github.com/chiisen/mini_bot/pkg/providers"
)

// loadTestTranslations 載入專案的 lang 目錄並固定為英文，讓測試環境與正式執行時一致。
//
// 段落標題由 i18n 提供：若語言檔未載入，T() 會直接回傳 key 本身（例如
// "sections.identity"），系統提示詞就會出現這種字串。因此這裡除了載入，
// 也明確驗證載入確實生效，避免測試退化成「兩邊都用原始 key 相比」而誤判通過。
func loadTestTranslations(t *testing.T) {
	t.Helper()

	inst := i18n.GetInstance()
	if err := inst.LoadFromDir(filepath.Join("..", "..", "lang")); err != nil {
		t.Fatalf("載入 lang 目錄失敗: %v", err)
	}
	inst.SetLang("en")

	if got := inst.T("sections.identity"); got == "sections.identity" {
		t.Fatalf("語言檔未載入，T() 回傳原始 key: %s", got)
	}
}

// tr 取得翻譯字串；測試以 i18n 的輸出為預期值，不硬編碼特定語言的文字。
func tr(key string) string {
	return i18n.GetInstance().T(key)
}

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
	loadTestTranslations(t)

	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "IDENTITY.md"), []byte("I am a helpful assistant"), 0644)

	builder := NewContextBuilder(tmpDir)
	result, err := builder.Build([]providers.ToolDefinition{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !contains(result, tr("sections.identity")) {
		t.Errorf("expected %s section in result", tr("sections.identity"))
	}
	if !contains(result, "I am a helpful assistant") {
		t.Error("expected identity content in result")
	}
}

func TestBuilder_Build_LoadOrder(t *testing.T) {
	loadTestTranslations(t)

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
	identityPos := indexOf(result, tr("sections.identity"))
	agentPos := indexOf(result, tr("sections.agent_guidelines"))
	soulPos := indexOf(result, tr("sections.personality"))
	userPos := indexOf(result, tr("sections.user_preferences"))

	if identityPos < 0 || agentPos < 0 || soulPos < 0 || userPos < 0 {
		t.Fatal("missing expected sections")
	}

	if !(identityPos < agentPos && agentPos < soulPos && soulPos < userPos) {
		t.Error("sections should be in correct order")
	}
}

func TestBuilder_Build_WithTools(t *testing.T) {
	loadTestTranslations(t)

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

	if !contains(result, tr("sections.available_tools")) {
		t.Errorf("expected %s section", tr("sections.available_tools"))
	}
	if !contains(result, "read_file") {
		t.Error("expected read_file tool description")
	}
	if !contains(result, "exec") {
		t.Error("expected exec tool description")
	}
}

func TestBuilder_Build_WithMemory(t *testing.T) {
	loadTestTranslations(t)

	tmpDir := t.TempDir()
	memoryDir := filepath.Join(tmpDir, "memory")
	os.MkdirAll(memoryDir, 0755)
	os.WriteFile(filepath.Join(memoryDir, "MEMORY.md"), []byte("Past memories here"), 0644)

	builder := NewContextBuilder(tmpDir)
	result, err := builder.Build([]providers.ToolDefinition{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !contains(result, tr("sections.memory")) {
		t.Errorf("expected %s section", tr("sections.memory"))
	}
	if !contains(result, "Past memories here") {
		t.Error("expected memory content")
	}
}

func TestBuilder_Build_MissingFiles(t *testing.T) {
	loadTestTranslations(t)

	tmpDir := t.TempDir()
	// Create only some files
	os.WriteFile(filepath.Join(tmpDir, "IDENTITY.md"), []byte("I exist"), 0644)

	builder := NewContextBuilder(tmpDir)
	result, err := builder.Build([]providers.ToolDefinition{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have IDENTITY but not other sections
	if !contains(result, tr("sections.identity")) {
		t.Errorf("expected %s section", tr("sections.identity"))
	}
	if contains(result, tr("sections.memory")) {
		t.Errorf("should not have %s section for missing file", tr("sections.memory"))
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
