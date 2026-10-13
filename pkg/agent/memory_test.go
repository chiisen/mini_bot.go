package agent

import (
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
