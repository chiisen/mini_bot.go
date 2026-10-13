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

	// 基準是展開 symlink 後的 workspace（macOS 的 tmpDir 位於 /var，會被展開成 /private/var）
	expected := filepath.Join(sandbox.Workspace, "test.txt")
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

	expected := filepath.Join(sandbox.Workspace, "subdir", "test.txt")
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
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

	expected := filepath.Join(sandbox.Workspace, "subdir/nested/file.txt")
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

// TestSandbox_CheckPath_SymlinkedWorkspace 驗證 workspace 路徑本身含 symlink 時，
// 既有檔案與尚未建立的新檔都必須被視為沙盒內的路徑。
// 這正是 macOS 的實際情況：/var 是 private/var 的 symlink，tmpDir 與 $TMPDIR 皆位於其下。
func TestSandbox_CheckPath_SymlinkedWorkspace(t *testing.T) {
	base := t.TempDir()
	realDir := filepath.Join(base, "real", "workspace")
	if err := os.MkdirAll(realDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	linkDir := filepath.Join(base, "link")
	if err := os.Symlink(filepath.Join(base, "real"), linkDir); err != nil {
		t.Skipf("無法建立 symlink，略過此測試: %v", err)
	}

	// 以含 symlink 的路徑建立沙盒
	sandbox, err := NewSandbox(filepath.Join(linkDir, "workspace"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := os.WriteFile(filepath.Join(realDir, "existing.txt"), []byte("hello"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	t.Run("既有檔案", func(t *testing.T) {
		if _, err := sandbox.CheckPath("existing.txt"); err != nil {
			t.Errorf("既有檔案應被允許，卻收到錯誤: %v", err)
		}
	})

	t.Run("尚未建立的新檔", func(t *testing.T) {
		result, err := sandbox.CheckPath("new.txt")
		if err != nil {
			t.Errorf("新檔應被允許，卻收到錯誤: %v", err)
		}
		if expected := filepath.Join(sandbox.Workspace, "new.txt"); result != expected {
			t.Errorf("expected %s, got %s", expected, result)
		}
	})
}

// TestSandbox_CheckPath_RejectsSymlinkEscape 驗證 workspace 內的 symlink 不能用來存取外部。
// 「新建檔案」的子案例同時是既有漏洞的迴歸測試：修正前，不存在的目標會跳過
// symlink 展開，因此可穿過連結寫到沙盒外。
func TestSandbox_CheckPath_RejectsSymlinkEscape(t *testing.T) {
	base := t.TempDir()
	workspaceDir := filepath.Join(base, "workspace")
	outsideDir := filepath.Join(base, "outside")

	for _, dir := range []string{workspaceDir, outsideDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create dir: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(outsideDir, "secret.txt"), []byte("secret"), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	// workspace/leak -> <base>/outside
	if err := os.Symlink(outsideDir, filepath.Join(workspaceDir, "leak")); err != nil {
		t.Skipf("無法建立 symlink，略過此測試: %v", err)
	}

	sandbox, err := NewSandbox(workspaceDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Run("既有檔案", func(t *testing.T) {
		if _, err := sandbox.CheckPath("leak/secret.txt"); err == nil {
			t.Error("透過 symlink 讀取既有檔案應被拒絕")
		}
	})

	t.Run("新建檔案", func(t *testing.T) {
		if _, err := sandbox.CheckPath("leak/new.txt"); err == nil {
			t.Error("透過 symlink 建立新檔應被拒絕")
		}
	})
}
