package channels

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/chiisen/mini_bot/pkg/config"
)

// TestTelegramChannel_OffsetPersistence 驗證長輪詢 offset 的讀寫。
//
// 不持久化時，行程若在兩次輪詢之間結束，Telegram 會重新送達那批更新，
// 使用者因此收到重複回覆（見 issue #25）。
func TestTelegramChannel_OffsetPersistence(t *testing.T) {
	offsetPath := filepath.Join(t.TempDir(), "nested", "telegram_offset")

	ch := &TelegramChannel{offsetPath: offsetPath}

	if got := ch.loadOffset(); got != 0 {
		t.Errorf("檔案不存在時應回 0，得到 %d", got)
	}

	ch.saveOffset(42)
	if got := ch.loadOffset(); got != 42 {
		t.Errorf("expected 42, got %d", got)
	}

	info, err := os.Stat(offsetPath)
	if err != nil {
		t.Fatalf("offset 檔案未被建立: %v", err)
	}
	// Windows 的 os.Stat 對可寫檔固定回報 0666，無法觀察 0600。
	if runtime.GOOS != "windows" {
		if perm := info.Mode().Perm(); perm != 0600 {
			t.Errorf("expected 0600, got %04o", perm)
		}
	}

	// 內容損壞時視為 0，而不是讓輪詢帶著錯誤的 offset 繼續
	if err := os.WriteFile(offsetPath, []byte("not-a-number"), 0600); err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	if got := ch.loadOffset(); got != 0 {
		t.Errorf("內容損壞時應回 0，得到 %d", got)
	}

	// 負值同樣視為 0
	if err := os.WriteFile(offsetPath, []byte("-5"), 0600); err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	if got := ch.loadOffset(); got != 0 {
		t.Errorf("負值應回 0，得到 %d", got)
	}
}

// TestTelegramChannel_OffsetDisabled 驗證未設定路徑時不持久化也不 panic。
func TestTelegramChannel_OffsetDisabled(t *testing.T) {
	ch := &TelegramChannel{}

	if got := ch.loadOffset(); got != 0 {
		t.Errorf("expected 0, got %d", got)
	}

	ch.saveOffset(10) // 不應 panic
}

// TestNewTelegramChannel 驗證建構子建立白名單與 offset 路徑。
func TestNewTelegramChannel(t *testing.T) {
	cfg := &config.TelegramConfig{Token: "123:abc", AllowFrom: []string{"1", "2"}}
	ch := NewTelegramChannel(cfg, nil, "/tmp/offset")

	if ch.Token != "123:abc" {
		t.Errorf("unexpected token: %s", ch.Token)
	}
	if !ch.AllowFrom["1"] || !ch.AllowFrom["2"] || ch.AllowFrom["3"] {
		t.Errorf("unexpected allow list: %v", ch.AllowFrom)
	}
	if ch.offsetPath != "/tmp/offset" {
		t.Errorf("unexpected offset path: %s", ch.offsetPath)
	}
}
