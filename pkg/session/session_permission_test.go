package session

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chiisen/mini_bot/pkg/providers"
)

// TestManager_Save_FilePermission 驗證會話檔以 0600 寫入。
//
// 會話內容是完整對話記錄（可能含程式碼、金鑰與個資），應與檔案工具的 0600
// 一致，不讓同機其他使用者讀取。
func TestManager_Save_FilePermission(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	if err := m.Save("perm-session", []providers.Message{{Role: "user", Content: "Hello"}}); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	info, err := os.Stat(filepath.Join(tmpDir, "perm-session.json"))
	if err != nil {
		t.Fatalf("failed to stat session file: %v", err)
	}

	if got := info.Mode().Perm(); got != 0600 {
		t.Errorf("expected session file mode 0600, got %04o", got)
	}
}
