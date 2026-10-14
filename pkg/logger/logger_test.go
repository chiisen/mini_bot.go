package logger

import (
	"fmt"
	"strings"
	"testing"
)

// TestSanitizeMessage 驗證日誌訊息中的敏感字串會被遮蔽。
//
// SECURITY_UPGRADE_TODO.md 把「日誌敏感資訊過濾」列為已完成，因此這裡把實際行為
// 固定下來：金鑰不得以明文出現在日誌行裡。
func TestSanitizeMessage(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		mustNotBe string
	}{
		{
			name:      "OpenAI 風格金鑰",
			input:     "using sk-abcdefghijklmnopqrstuvwxyz01 for the request",
			mustNotBe: "sk-abcdefghijklmnopqrstuvwxyz01",
		},
		{
			name:      "key=value 形式",
			input:     "config api_key=supersecretvalue loaded",
			mustNotBe: "supersecretvalue",
		},
		{
			name:      "Telegram bot token",
			input:     "bot_token=123456789:AAF-abcdefghijklmnopqrstuv",
			mustNotBe: "AAF-abcdefghijklmnopqrstuv",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeMessage(tc.input)
			if strings.Contains(got, tc.mustNotBe) {
				t.Errorf("敏感字串未被遮蔽：%s", got)
			}
			if !strings.Contains(got, "REDACTED") {
				t.Errorf("預期留下遮蔽標記，得到：%s", got)
			}
		})
	}
}

// TestSanitizeMessage_KeepsNormalText 驗證一般文字不會被動到。
func TestSanitizeMessage_KeepsNormalText(t *testing.T) {
	const msg = "session saved for chat 12345"

	if got := sanitizeMessage(msg); got != msg {
		t.Errorf("一般訊息不應被改寫：%s", got)
	}
}

// TestSanitizeArgs 驗證以敏感名稱作為鍵的參數值會被遮蔽，其他參數不受影響。
func TestSanitizeArgs(t *testing.T) {
	got := sanitizeArgs("error", "boom", "api_key", "sk-1234567890abcdefghij")
	joined := fmt.Sprint(got...)

	if strings.Contains(joined, "sk-1234567890abcdefghij") {
		t.Errorf("api_key 的值未被遮蔽：%v", got)
	}
	if !strings.Contains(joined, "boom") {
		t.Errorf("非敏感的參數不應被遮蔽：%v", got)
	}
}

// TestSanitizeArgs_PreservesPairs 驗證鍵與值仍成對（日誌解析不會錯位）。
func TestSanitizeArgs_PreservesPairs(t *testing.T) {
	got := sanitizeArgs("token", "secret-value", "count", 3)

	if len(got) != 4 {
		t.Fatalf("expected 4 elements, got %d: %v", len(got), got)
	}
	if got[0] != "token" || got[1] != "[REDACTED]" {
		t.Errorf("unexpected pair: %v", got[:2])
	}
	if got[2] != "count" || got[3] != 3 {
		t.Errorf("unexpected pair: %v", got[2:])
	}
}
