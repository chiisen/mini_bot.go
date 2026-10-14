package i18n

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

var (
	instance *I18n
	once     sync.Once
	mu       sync.RWMutex
	execDir  string
)

type I18n struct {
	translations   map[string]map[string]any
	currentLang    string
	availableLangs []string
	initOnce       sync.Once
}

func init() {
	once.Do(func() {
		instance = &I18n{
			translations:   make(map[string]map[string]any),
			currentLang:    "en",
			availableLangs: []string{"en", "zh-tw"},
		}
	})
	execDir, _ = os.Getwd()
}

func GetInstance() *I18n {
	return instance
}

// langDirCandidates 回傳載入語言檔的候選目錄，依優先序：
//
//  1. 執行檔所在目錄——與 pkg/config 的尋找邏輯一致，讓「執行檔旁放著 lang/」
//     的部署方式不受當前工作目錄影響
//  2. 當前工作目錄
//
// 先前只檢查工作目錄，因此換個工作目錄執行（例如 `docker run -w /tmp ...`）就會
// 找不到語言檔，所有 i18n 字串退化成原始 key。
func langDirCandidates() []string {
	var dirs []string

	if exe, err := os.Executable(); err == nil {
		dirs = append(dirs, filepath.Join(filepath.Dir(exe), "lang"))
	}
	if execDir != "" {
		dirs = append(dirs, filepath.Join(execDir, "lang"))
	}

	return dirs
}

// hasTranslations 回報是否已載入任何語言檔。
func (i *I18n) hasTranslations() bool {
	mu.RLock()
	defer mu.RUnlock()
	return len(i.translations) > 0
}

// ensureInitialized 只在第一次取用翻譯時載入語言檔。
//
// 必須是執行緒安全的：gateway 模式下會有多個 goroutine 同時呼叫 T()，
// 而載入動作會寫入 translations map，未同步的並行寫入會直接以
// fatal error: concurrent map writes 終止程式。
func (i *I18n) ensureInitialized() {
	i.initOnce.Do(func() {
		for _, dir := range langDirCandidates() {
			_ = i.LoadFromDir(dir)
			if i.hasTranslations() {
				break
			}
		}

		if lang := os.Getenv("MINIBOT_LANGUAGE"); lang != "" {
			i.SetLang(lang)
		}
	})
}

func (i *I18n) LoadFromDir(dir string) error {
	loaded := make(map[string]map[string]any)

	for _, lang := range i.availableLangs {
		path := filepath.Join(dir, lang+".json")
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var trans map[string]any
		if err := json.Unmarshal(data, &trans); err != nil {
			return fmt.Errorf("failed to parse %s: %w", path, err)
		}
		loaded[lang] = trans
	}

	// 先把檔案讀完再進鎖內寫入 map，避免在鎖內做 I/O
	mu.Lock()
	defer mu.Unlock()
	for lang, trans := range loaded {
		i.translations[lang] = trans
	}
	return nil
}

func (i *I18n) SetLang(lang string) {
	mu.Lock()
	defer mu.Unlock()
	for _, l := range i.availableLangs {
		if l == lang {
			i.currentLang = lang
			return
		}
	}
}

func (i *I18n) GetLang() string {
	mu.RLock()
	defer mu.RUnlock()
	return i.currentLang
}

func (i *I18n) T(key string, args ...any) string {
	i.ensureInitialized()

	mu.RLock()
	defer mu.RUnlock()

	trans := i.translations[i.currentLang]
	if trans == nil {
		trans = i.translations["en"]
	}

	keys := parseKey(key)
	val := getNested(trans, keys)

	if str, ok := val.(string); ok {
		if len(args) > 0 {
			return fmt.Sprintf(str, args...)
		}
		return str
	}
	return key
}

func getNested(m map[string]any, keys []string) any {
	current := m
	for _, k := range keys[:len(keys)-1] {
		if next, ok := current[k].(map[string]any); ok {
			current = next
		} else {
			return nil
		}
	}
	return current[keys[len(keys)-1]]
}

func parseKey(key string) []string {
	var keys []string
	var current []byte
	for _, c := range key {
		if c == '.' {
			if len(current) > 0 {
				keys = append(keys, string(current))
				current = nil
			}
		} else {
			current = append(current, byte(c))
		}
	}
	if len(current) > 0 {
		keys = append(keys, string(current))
	}
	return keys
}

func (i *I18n) AvailableLangs() []string {
	mu.RLock()
	defer mu.RUnlock()
	langs := make([]string, len(i.availableLangs))
	copy(langs, i.availableLangs)
	return langs
}

func SetDefaultLang(lang string) {
	GetInstance().SetLang(lang)
}
