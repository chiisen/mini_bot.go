package main

import (
	"testing"

	"github.com/chiisen/mini_bot/pkg/config"
)

// TestDescribeLanguage 驗證語言來源的判斷。
//
// config.Load 一定會把 Language 填上預設值，因此先前「(default)」分支永遠到不了，
// 而且設定了不支援的語言時會顯示得像是生效了（i18n.SetLang 是靜默忽略）。
func TestDescribeLanguage(t *testing.T) {
	cases := []struct {
		name          string
		effective     string
		envLang       string
		cfgLang       string
		wantSource    string
		wantSupported bool
	}{
		{
			name:          "環境變數且被支援",
			effective:     "zh-tw",
			envLang:       "zh-tw",
			cfgLang:       "en",
			wantSource:    "env",
			wantSupported: true,
		},
		{
			name:          "環境變數不被支援（SetLang 會忽略）",
			effective:     "en",
			envLang:       "fr",
			cfgLang:       "en",
			wantSource:    "env",
			wantSupported: false,
		},
		{
			name:          "設定檔指定非預設語言且被支援",
			effective:     "zh-tw",
			cfgLang:       "zh-tw",
			wantSource:    "config",
			wantSupported: true,
		},
		{
			name:          "設定檔指定不被支援的語言",
			effective:     "en",
			cfgLang:       "fr",
			wantSource:    "config",
			wantSupported: false,
		},
		{
			name:          "未設定時使用預設值",
			effective:     "en",
			cfgLang:       config.DefaultLanguage,
			wantSource:    "default",
			wantSupported: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := describeLanguage(tc.effective, tc.envLang, tc.cfgLang)

			if got.Source != tc.wantSource {
				t.Errorf("expected source %q, got %q", tc.wantSource, got.Source)
			}
			if got.Supported != tc.wantSupported {
				t.Errorf("expected supported=%v, got %v", tc.wantSupported, got.Supported)
			}
			if got.Effective != tc.effective {
				t.Errorf("expected effective %q, got %q", tc.effective, got.Effective)
			}
		})
	}
}

// TestTelegramStatus 驗證 Telegram 狀態與 gateway 的判斷一致。
//
// 先前 status 自行比對固定的佔位字串，與 gateway 的 isPlaceholderToken 不一致，
// 因此 config.example.json 風格的小寫佔位字串會顯示 Ready，而 gateway 其實會跳過。
func TestTelegramStatus(t *testing.T) {
	cases := []struct {
		name string
		cfg  config.TelegramConfig
		want string
	}{
		{"未啟用", config.TelegramConfig{}, "ℹ️  Telegram Channel: Disabled"},
		{
			"啟用但使用範本佔位字串",
			config.TelegramConfig{Enabled: true, Token: "YOUR_BOT_TOKEN_HERE"},
			"⚠️  Telegram: Enabled but Token is invalid/default",
		},
		{
			"啟用但使用 config.example.json 的小寫佔位字串",
			config.TelegramConfig{Enabled: true, Token: "your-telegram-bot-token-here"},
			"⚠️  Telegram: Enabled but Token is invalid/default",
		},
		{
			"啟用且 token 看起來有效",
			config.TelegramConfig{Enabled: true, Token: "123456789:AAF-abcdefghijklmnop"},
			"✅ Telegram Channel: Ready",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := telegramStatus(tc.cfg); got != tc.want {
				t.Errorf("expected %q, got %q", tc.want, got)
			}
		})
	}
}
