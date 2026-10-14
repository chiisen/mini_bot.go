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

// TestLangDirCandidates 驗證語言檔的尋找順序。
//
// 第一個候選必須是執行檔所在目錄（與 pkg/config 的邏輯一致）：先前只檢查工作
// 目錄，導致換個工作目錄執行時所有 i18n 字串退化成原始 key。
func TestLangDirCandidates(t *testing.T) {
	dirs := langDirCandidates()

	if len(dirs) == 0 {
		t.Fatal("expected at least one candidate directory")
	}

	exe, err := os.Executable()
	if err != nil {
		t.Skipf("無法取得執行檔路徑，略過順序檢查: %v", err)
	}

	want := filepath.Join(filepath.Dir(exe), "lang")
	if dirs[0] != want {
		t.Errorf("第一個候選應為執行檔旁的 lang（%s），得到 %s", want, dirs[0])
	}
}

// TestHasTranslations 驗證載入前後的回報，以及載入失敗不會清空既有翻譯。
func TestHasTranslations(t *testing.T) {
	langDir := t.TempDir()
	inst := &I18n{
		translations:   make(map[string]map[string]any),
		currentLang:    "en",
		availableLangs: []string{"en", "zh-tw"},
	}

	if inst.hasTranslations() {
		t.Error("尚未載入時應回報 false")
	}

	writeLangFile(t, filepath.Join(langDir, "en.json"), `{"app":{"name":"MiniBot"}}`)

	if err := inst.LoadFromDir(langDir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !inst.hasTranslations() {
		t.Error("載入後應回報 true")
	}
	if got := inst.T("app.name"); got != "MiniBot" {
		t.Errorf("expected MiniBot, got %s", got)
	}

	if err := inst.LoadFromDir(filepath.Join(langDir, "missing")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := inst.T("app.name"); got != "MiniBot" {
		t.Errorf("載入不存在的目錄不應清空既有翻譯，得到 %s", got)
	}
}
