package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestMemoryManager_Read_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewMemoryManager(filepath.Join(tmpDir, "nonexistent.md"))

	content, err := m.Read()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if content != "" {
		t.Errorf("expected empty content for non-existent file, got %s", content)
	}
}

func TestMemoryManager_Read_Exists(t *testing.T) {
	tmpDir := t.TempDir()
	memoryPath := filepath.Join(tmpDir, "MEMORY.md")
	os.WriteFile(memoryPath, []byte("Previous memories"), 0644)

	m := NewMemoryManager(memoryPath)
	content, err := m.Read()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if content != "Previous memories" {
		t.Errorf("expected 'Previous memories', got '%s'", content)
	}
}

func TestMemoryManager_Append_NewFile(t *testing.T) {
	tmpDir := t.TempDir()
	memoryPath := filepath.Join(tmpDir, "MEMORY.md")

	m := NewMemoryManager(memoryPath)
	err := m.Append("First memory")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify content
	data, err := os.ReadFile(memoryPath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	expected := "First memory\n"
	if string(data) != expected {
		t.Errorf("expected '%s', got '%s'", expected, string(data))
	}
}

func TestMemoryManager_Append_ExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	memoryPath := filepath.Join(tmpDir, "MEMORY.md")
	os.WriteFile(memoryPath, []byte("First memory\n"), 0644)

	m := NewMemoryManager(memoryPath)
	err := m.Append("Second memory")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify content
	data, err := os.ReadFile(memoryPath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	expected := "First memory\nSecond memory\n"
	if string(data) != expected {
		t.Errorf("expected '%s', got '%s'", expected, string(data))
	}
}

func TestMemoryManager_Append_Multiple(t *testing.T) {
	tmpDir := t.TempDir()
	memoryPath := filepath.Join(tmpDir, "MEMORY.md")

	m := NewMemoryManager(memoryPath)
	m.Append("Memory 1")
	m.Append("Memory 2")
	m.Append("Memory 3")

	data, err := os.ReadFile(memoryPath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	expected := "Memory 1\nMemory 2\nMemory 3\n"
	if string(data) != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, string(data))
	}
}

// TestMemoryManager_Append_StoresContentVerbatim 確保記憶內容原樣落盤。
// Append 曾誤用 SanitizeInput，導致寫入的記憶被包成 ```user-input code fence，
// 且 {} [] <script 等字元被轉義成 HTML entity，模型讀到的記憶因此是破損的。
func TestMemoryManager_Append_StoresContentVerbatim(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{"JSON 與方括號", `Use {"key": ["value1", "value2"]} as the payload`},
		{"Markdown code fence", "```go\nfmt.Println(\"hi\")\n```"},
		{"HTML 標籤", "<script>alert(1)</script> 不應被轉義"},
		{"中文與 emoji", "使用者偏好：回覆用繁體中文 🚀"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			memoryPath := filepath.Join(t.TempDir(), "MEMORY.md")
			m := NewMemoryManager(memoryPath)

			if err := m.Append(tc.content); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			data, err := os.ReadFile(memoryPath)
			if err != nil {
				t.Fatalf("failed to read file: %v", err)
			}

			expected := tc.content + "\n"
			if string(data) != expected {
				t.Errorf("expected '%s', got '%s'", expected, string(data))
			}
		})
	}
}

// ============================================================================
// MemoryWriteTool: 長期記憶的寫入端
// ============================================================================

// TestMemoryWriteTool_Execute 驗證工具把內容寫進記憶檔，且讀得回來。
func TestMemoryWriteTool_Execute(t *testing.T) {
	memoryPath := filepath.Join(t.TempDir(), "memory", "MEMORY.md")
	tool := &MemoryWriteTool{Memory: NewMemoryManager(memoryPath)}

	result := tool.Execute(context.Background(), map[string]any{"content": "使用者偏好繁體中文"})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.ForLLM)
	}

	data, err := os.ReadFile(memoryPath)
	if err != nil {
		t.Fatalf("記憶檔未被建立: %v", err)
	}
	if string(data) != "使用者偏好繁體中文\n" {
		t.Errorf("unexpected memory content: %q", string(data))
	}
}

// TestMemoryWriteTool_CreatesDirectory 驗證目錄不存在時仍可寫入
// （只有 onboard 會建立 workspace/memory）。
func TestMemoryWriteTool_CreatesDirectory(t *testing.T) {
	memoryPath := filepath.Join(t.TempDir(), "nonexistent", "memory", "MEMORY.md")
	tool := &MemoryWriteTool{Memory: NewMemoryManager(memoryPath)}

	if result := tool.Execute(context.Background(), map[string]any{"content": "note"}); result.IsError {
		t.Fatalf("unexpected error: %s", result.ForLLM)
	}
	if _, err := os.Stat(memoryPath); err != nil {
		t.Errorf("expected the memory file to exist: %v", err)
	}
}

// TestMemoryWriteTool_RejectsEmptyContent 驗證空內容會被拒絕。
func TestMemoryWriteTool_RejectsEmptyContent(t *testing.T) {
	tool := &MemoryWriteTool{Memory: NewMemoryManager(filepath.Join(t.TempDir(), "MEMORY.md"))}

	for _, args := range []map[string]any{{}, {"content": ""}, {"content": "   "}} {
		if result := tool.Execute(context.Background(), args); !result.IsError {
			t.Errorf("expected empty content to be rejected, args=%v", args)
		}
	}
}

// TestMemoryWriteTool_Schema 驗證工具名稱與參數 schema（LLM 靠這些資訊呼叫工具）。
func TestMemoryWriteTool_Schema(t *testing.T) {
	tool := &MemoryWriteTool{Memory: NewMemoryManager("unused")}

	if tool.Name() != "memory_write" {
		t.Errorf("expected name memory_write, got %s", tool.Name())
	}

	params := tool.Parameters()
	if params["type"] != "object" {
		t.Errorf("expected an object schema, got %v", params["type"])
	}

	required, ok := params["required"].([]string)
	if !ok || len(required) != 1 || required[0] != "content" {
		t.Errorf("expected content to be required, got %v", params["required"])
	}
}
