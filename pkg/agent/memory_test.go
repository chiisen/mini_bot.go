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
