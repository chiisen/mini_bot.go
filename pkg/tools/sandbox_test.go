package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSandbox_New(t *testing.T) {
	tmpDir := t.TempDir()

	sandbox, err := NewSandbox(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sandbox.Workspace == "" {
		t.Error("expected workspace to be set")
	}
}

func TestSandbox_CheckPath_Relative(t *testing.T) {
	tmpDir := t.TempDir()
	sandbox, _ := NewSandbox(tmpDir)

	// Test relative path
	result, err := sandbox.CheckPath("test.txt")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	expected := filepath.Join(tmpDir, "test.txt")
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

func TestSandbox_CheckPath_Absolute(t *testing.T) {
	tmpDir := t.TempDir()
	sandbox, _ := NewSandbox(tmpDir)

	// Test absolute path inside workspace
	testFile := filepath.Join(tmpDir, "subdir", "test.txt")
	os.MkdirAll(filepath.Dir(testFile), 0755)

	result, err := sandbox.CheckPath(testFile)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result != testFile {
		t.Errorf("expected %s, got %s", testFile, result)
	}
}

func TestSandbox_CheckPath_OutsideWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	sandbox, _ := NewSandbox(tmpDir)

	// Try to escape workspace
	_, err := sandbox.CheckPath("../../../etc/passwd")
	if err == nil {
		t.Error("expected error for path escaping workspace")
	}
}

func TestSandbox_CheckPath_Nested(t *testing.T) {
	tmpDir := t.TempDir()
	sandbox, _ := NewSandbox(tmpDir)

	// Test nested path
	result, err := sandbox.CheckPath("subdir/nested/file.txt")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	expected := filepath.Join(tmpDir, "subdir/nested/file.txt")
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

func TestSandbox_CheckPath_ParentTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "subdir"), 0755)
	sandbox, _ := NewSandbox(tmpDir)

	// Try path traversal attack
	_, err := sandbox.CheckPath("subdir/../../../test.txt")
	if err == nil {
		t.Error("expected error for path traversal")
	}
}
