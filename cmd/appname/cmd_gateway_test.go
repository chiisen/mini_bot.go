package main

import (
	"testing"

	"github.com/chiisen/mini_bot/pkg/bus"
	"github.com/chiisen/mini_bot/pkg/config"
)

// TestRegisterChannels 驗證頻道註冊的判斷，以及「沒有可用頻道」能被偵測。
//
// 先前沒有任何可用頻道時，StartAll 回傳的錯誤會讓 gateway 印出「正在關閉」後以
// 結束碼 0 退出，使用者看不出真正原因（見 issue #24）。
func TestRegisterChannels(t *testing.T) {
	cases := []struct {
		name string
		cfg  config.Config
		want int
	}{
		{
			name: "Telegram 未啟用",
			cfg:  config.Config{},
			want: 0,
		},
		{
			name: "啟用但 token 仍是佔位字串",
			cfg: config.Config{Channels: config.ChannelsConfig{
				Telegram: config.TelegramConfig{Enabled: true, Token: "YOUR_BOT_TOKEN_HERE"},
			}},
			want: 0,
		},
		{
			name: "啟用且 token 看起來有效",
			cfg: config.Config{Channels: config.ChannelsConfig{
				Telegram: config.TelegramConfig{Enabled: true, Token: "123456789:AAF-abcdefghijklmnop"},
			}},
			want: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, registered := registerChannels(&tc.cfg, bus.New(nil), t.TempDir()+"/telegram_offset")
			if registered != tc.want {
				t.Errorf("expected %d registered channel(s), got %d", tc.want, registered)
			}
		})
	}
}
