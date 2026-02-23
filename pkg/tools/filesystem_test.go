package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestReadFileTool(t *testing.T) {
	tmpDir := t.TempDir()
	sandbox, _ := NewSandbox(tmpDir)

	// Create test file
	testContent := "Hello World\nLine 2"
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte(testContent), 0644)

	tool := ReadFileTool{Sandbox: sandbox}
	result := tool.Execute(context.Background(), map[string]any{"path": "test.txt"})

	if result.IsError {
		t.Errorf("unexpected error: %s", result.ForLLM)
	}

	// Should contain line numbers
	if len(result.ForLLM) == 0 {
		t.Error("expected content, got empty")
	}
}

func TestReadFileTool_Errors(t *testing.T) {
	tmpDir := t.TempDir()
	sandbox, _ := NewSandbox(tmpDir)

	tool := ReadFileTool{Sandbox: sandbox}

	// Test non-existent file
	result := tool.Execute(context.Background(), map[string]any{"path": "nonexistent.txt"})
	if !result.IsError {
		t.Error("expected error for non-existent file")
	}

	// Test path outside workspace
	result = tool.Execute(context.Background(), map[string]any{"path": "../etc/passwd"})
	if !result.IsError {
		t.Error("expected error for path outside workspace")
	}
}

func TestWriteFileTool(t *testing.T) {
	tmpDir := t.TempDir()
	sandbox, _ := NewSandbox(tmpDir)

	tool := WriteFileTool{Sandbox: sandbox}
	result := tool.Execute(context.Background(), map[string]any{
		"path":    "newfile.txt",
		"content": "Test content",
	})

	if result.IsError {
		t.Errorf("unexpected error: %s", result.ForLLM)
	}

	// Verify file was created
	content, err := os.ReadFile(filepath.Join(tmpDir, "newfile.txt"))
	if err != nil {
		t.Fatalf("failed to read created file: %v", err)
	}
	if string(content) != "Test content" {
		t.Errorf("expected 'Test content', got '%s'", string(content))
	}
}

func TestAppendFileTool(t *testing.T) {
	tmpDir := t.TempDir()
	sandbox, _ := NewSandbox(tmpDir)

	// Create initial file
	testFile := filepath.Join(tmpDir, "append.txt")
	os.WriteFile(testFile, []byte("Line 1\n"), 0644)

	tool := AppendFileTool{Sandbox: sandbox}
	result := tool.Execute(context.Background(), map[string]any{
		"path":    "append.txt",
		"content": "Line 2\n",
	})

	if result.IsError {
		t.Errorf("unexpected error: %s", result.ForLLM)
	}

	// Verify content
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	expected := "Line 1\nLine 2\n"
	if string(content) != expected {
		t.Errorf("expected '%s', got '%s'", expected, string(content))
	}
}

func TestEditFileTool(t *testing.T) {
	tmpDir := t.TempDir()
	sandbox, _ := NewSandbox(tmpDir)

	// Create initial file
	testContent := "Line 1\nLine 2\nLine 3\n"
	testFile := filepath.Join(tmpDir, "edit.txt")
	os.WriteFile(testFile, []byte(testContent), 0644)

	tool := EditFileTool{Sandbox: sandbox}
	result := tool.Execute(context.Background(), map[string]any{
		"path":        "edit.txt",
		"start_line":  2.0,
		"end_line":    2.0,
		"new_content": "Modified Line 2",
	})

	if result.IsError {
		t.Errorf("unexpected error: %s", result.ForLLM)
	}

	// Verify content
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	expected := "Line 1\nModified Line 2\nLine 3\n"
	if string(content) != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, string(content))
	}
}

func TestEditFileTool_OutOfBounds(t *testing.T) {
	tmpDir := t.TempDir()
	sandbox, _ := NewSandbox(tmpDir)

	testFile := filepath.Join(tmpDir, "edit.txt")
	os.WriteFile(testFile, []byte("Line 1\nLine 2\n"), 0644)

	tool := EditFileTool{Sandbox: sandbox}

	// Test start_line out of bounds
	result := tool.Execute(context.Background(), map[string]any{
		"path":        "edit.txt",
		"start_line":  10.0,
		"end_line":    15.0,
		"new_content": "New",
	})
	if !result.IsError {
		t.Error("expected error for out of bounds")
	}

	// Test end_line < start_line
	result = tool.Execute(context.Background(), map[string]any{
		"path":        "edit.txt",
		"start_line":  3.0,
		"end_line":    1.0,
		"new_content": "New",
	})
	if !result.IsError {
		t.Error("expected error for end < start")
	}
}

func TestListDirTool(t *testing.T) {
	tmpDir := t.TempDir()
	sandbox, _ := NewSandbox(tmpDir)

	// Create test directory structure
	os.MkdirAll(filepath.Join(tmpDir, "subdir"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("content"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "file2.log"), []byte("content"), 0644)

	tool := ListDirTool{Sandbox: sandbox}
	result := tool.Execute(context.Background(), map[string]any{"path": "."})

	if result.IsError {
		t.Errorf("unexpected error: %s", result.ForLLM)
	}

	// Should contain both files
	if result.ForLLM == "" {
		t.Error("expected directory listing")
	}
}

func TestListDirTool_Errors(t *testing.T) {
	tmpDir := t.TempDir()
	sandbox, _ := NewSandbox(tmpDir)

	tool := ListDirTool{Sandbox: sandbox}

	// Test non-existent directory
	result := tool.Execute(context.Background(), map[string]any{"path": "nonexistent"})
	if !result.IsError {
		t.Error("expected error for non-existent directory")
	}
}
