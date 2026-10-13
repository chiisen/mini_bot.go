package i18n

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// TestI18n_ConcurrentLoadAndTranslate 驗證多個 goroutine 同時載入與取用翻譯時
// 不會發生 data race。
//
// 修正前 ensureInitialized 以無鎖的 bool 判斷是否已初始化，而 LoadFromDir 在未持鎖
// 的情況下寫入 translations map；gateway 模式下多個 goroutine 並行呼叫 T() 時，
// 可能出現 fatal error: concurrent map writes 而直接終止程式。
func TestI18n_ConcurrentLoadAndTranslate(t *testing.T) {
	langDir := t.TempDir()
	writeLangFile(t, filepath.Join(langDir, "en.json"), `{"sections":{"identity":"[IDENTITY]"}}`)
	writeLangFile(t, filepath.Join(langDir, "zh-tw.json"), `{"sections":{"identity":"[身分]"}}`)

	inst := &I18n{
		translations:   make(map[string]map[string]any),
		currentLang:    "en",
		availableLangs: []string{"en", "zh-tw"},
	}

	start := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_ = inst.LoadFromDir(langDir)
			_ = inst.T("sections.identity")
			inst.SetLang("zh-tw")
			inst.SetLang("en")
		}()
	}

	close(start)
	wg.Wait()

	inst.SetLang("en")
	if got := inst.T("sections.identity"); got != "[IDENTITY]" {
		t.Errorf("expected [IDENTITY], got %s", got)
	}
}

func writeLangFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
}
