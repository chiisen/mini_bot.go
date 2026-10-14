package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chiisen/mini_bot/pkg/config"
)

// TestDefaultConfig_IsLoadableAndResolvable 確保 onboard 產生的範本與現行的
// config schema 一致。
//
// 先前範本仍是舊版的 model_list 格式，欄位也是 snake_case，導致照著 README 的
// 流程（onboard → agent）在第二步就失敗，而且錯誤訊息不會指向真正的原因。
func TestDefaultConfig_IsLoadableAndResolvable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(defaultConfig), 0600); err != nil {
		t.Fatalf("failed to write template: %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("config.Load failed on the onboard template: %v", err)
	}

	if len(cfg.Providers) == 0 {
		t.Fatal("範本沒有任何 providers，FindModel 必定失敗")
	}

	modelCfg, err := cfg.FindModel(cfg.Agents.Defaults.Model)
	if err != nil {
		t.Fatalf("FindModel(%q) failed: %v", cfg.Agents.Defaults.Model, err)
	}
	if modelCfg.APIBase == "" {
		t.Error("expected the template provider to define apiBase")
	}
	if modelCfg.RequestTimeout <= 0 {
		t.Error("expected the template to resolve a request timeout")
	}

	// 範本中的設定值必須真的被解析，而不是被靜默丟棄後套用預設值
	if cfg.Agents.Defaults.MaxTokens != 8192 {
		t.Errorf("expected maxTokens 8192 from the template, got %d", cfg.Agents.Defaults.MaxTokens)
	}
	if cfg.Agents.Defaults.Temperature != 0.7 {
		t.Errorf("expected temperature 0.7 from the template, got %v", cfg.Agents.Defaults.Temperature)
	}
	if cfg.Agents.Defaults.RequestTimeoutSeconds != 60 {
		t.Errorf("expected requestTimeoutSeconds 60 from the template, got %d", cfg.Agents.Defaults.RequestTimeoutSeconds)
	}
	if cfg.Agents.Defaults.MaxToolIterations != 20 {
		t.Errorf("expected maxToolIterations 20 from the template, got %d", cfg.Agents.Defaults.MaxToolIterations)
	}

	// Telegram 的欄位名必須是 botToken，寫成 token 會被靜默忽略
	if cfg.Channels.Telegram.Token != "YOUR_BOT_TOKEN_HERE" {
		t.Errorf("expected the template botToken placeholder, got %q", cfg.Channels.Telegram.Token)
	}
	if len(cfg.Channels.Telegram.AllowFrom) != 1 {
		t.Errorf("expected one allow_from entry, got %v", cfg.Channels.Telegram.AllowFrom)
	}
}

// TestValidateConfigFile 驗證既有設定檔的可用性檢查。
//
// onboard 不會覆蓋既有檔案，因此升級後留下的舊設定檔需要被檢查出來，否則使用者
// 只會看到 app agent 失敗。
func TestValidateConfigFile(t *testing.T) {
	t.Run("現行範本可通過", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.json")
		if err := os.WriteFile(path, []byte(defaultConfig), 0600); err != nil {
			t.Fatalf("failed to write: %v", err)
		}
		if err := validateConfigFile(path); err != nil {
			t.Errorf("範本應該可用，卻回報：%v", err)
		}
	})

	t.Run("舊版 schema 會被判定為不可用", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.json")
		legacy := `{
			"agents": {"defaults": {"model": "gpt4", "max_tokens": 8192}},
			"model_list": [{"model_name": "gpt4", "model": "openai/gpt-4", "api_key": "k"}]
		}`
		if err := os.WriteFile(path, []byte(legacy), 0600); err != nil {
			t.Fatalf("failed to write: %v", err)
		}
		if err := validateConfigFile(path); err == nil {
			t.Error("舊版 schema 應該被判定為不可用")
		}
	})

	t.Run("內容不是合法 JSON", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.json")
		if err := os.WriteFile(path, []byte("{ not json"), 0600); err != nil {
			t.Fatalf("failed to write: %v", err)
		}
		if err := validateConfigFile(path); err == nil {
			t.Error("無法解析的設定檔應該回報錯誤")
		}
	})
}

// TestIsPlaceholderToken 驗證範本佔位字串會被辨識，不會拿去呼叫 Telegram API。
func TestIsPlaceholderToken(t *testing.T) {
	cases := []struct {
		token string
		want  bool
	}{
		{"", true},
		{"YOUR_BOT_TOKEN_HERE", true},
		{"your-telegram-bot-token-here", true},
		{"123456789:AAF-abcdefghijklmnopqrstuvwxyz", false},
	}

	for _, tc := range cases {
		if got := isPlaceholderToken(tc.token); got != tc.want {
			t.Errorf("isPlaceholderToken(%q) = %v, want %v", tc.token, got, tc.want)
		}
	}
}
