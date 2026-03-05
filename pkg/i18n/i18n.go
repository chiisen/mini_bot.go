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
	initialized    bool
}

func init() {
	once.Do(func() {
		instance = &I18n{
			translations:   make(map[string]map[string]any),
			currentLang:    "en",
			availableLangs: []string{"en", "zh-tw"},
			initialized:    false,
		}
	})
	execDir, _ = os.Getwd()
}

func GetInstance() *I18n {
	return instance
}

func (i *I18n) ensureInitialized() {
	if i.initialized {
		return
	}
	langDir := filepath.Join(execDir, "lang")
	if _, err := os.Stat(langDir); os.IsNotExist(err) {
		langDir = "./lang"
	}
	_ = i.LoadFromDir(langDir)

	if lang := os.Getenv("MINIBOT_LANGUAGE"); lang != "" {
		i.SetLang(lang)
	}

	i.initialized = true
}

func (i *I18n) LoadFromDir(dir string) error {
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
