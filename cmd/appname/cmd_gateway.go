package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/chiisen/mini_bot/pkg/agent"
	"github.com/chiisen/mini_bot/pkg/bus"
	"github.com/chiisen/mini_bot/pkg/channels"
	"github.com/chiisen/mini_bot/pkg/config"
	"github.com/chiisen/mini_bot/pkg/i18n"
	"github.com/chiisen/mini_bot/pkg/logger"
)

// RunGateway handles the 'app gateway' command.
func RunGateway(args []string) error {
	logger.Init(false) // Use true if --debug is passed
	logger.Info("Starting Gateway mode...")

	// 1. Load config
	cfg, err := config.Load("~/.minibot.go/config.json")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// 2. Build Agent
	instance, err := agent.NewInstance(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize agent instance: %w", err)
	}

	// 3. Create Bus and start background listeners
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	messageBus := bus.New(instance)
	messageBus.Start(ctx)

	// 4. Register enabled channels
	offsetPath := filepath.Join(expandHome(config.DefaultConfigDir), "telegram_offset")
	manager, registered := registerChannels(cfg, messageBus, offsetPath)

	// 沒有任何可用頻道時直接回報。否則 StartAll 的錯誤會讓程式印出「正在關閉」後
	// 以結束碼 0 退出，使用者看不出真正的原因（見 issue #24）。
	if registered == 0 {
		return errors.New(i18n.GetInstance().T("cli.gateway_no_channels"))
	}

	// 5. Start Manager in background
	go func() {
		if err := manager.StartAll(ctx); err != nil {
			logger.Error("Channel manager error", "error", err)
			cancel() // force exit if channels crash
		}
	}()

	// 6. Wait for Termination Signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	logger.Info("Gateway is running. Press Ctrl+C to stop.")

	select {
	case <-c:
		logger.Info("Interrupt received, shutting down gracefully...")
		cancel()
	case <-ctx.Done():
		logger.Info("Context cancelled, shutting down...")
	}

	return nil
}

// registerChannels 依設定註冊可用的頻道，並回傳註冊數量。
//
// Telegram 只有在 enabled 為 true 且 token 不是範本佔位字串時才會註冊；呼叫端
// 據此判斷 gateway 是否有事情可做（見 issue #24）。
func registerChannels(cfg *config.Config, messageBus *bus.MessageBus, offsetPath string) (*channels.Manager, int) {
	manager := channels.NewManager()
	registered := 0

	if !cfg.Channels.Telegram.Enabled {
		logger.Info("Telegram is disabled, skipping.")
		return manager, registered
	}

	if isPlaceholderToken(cfg.Channels.Telegram.Token) {
		logger.Warn("Telegram is enabled but no valid bot token provided. Skipping Telegram.")
		return manager, registered
	}

	manager.Register(channels.NewTelegramChannel(&cfg.Channels.Telegram, messageBus, offsetPath))
	registered++
	logger.Info("Registered Telegram Channel")

	return manager, registered
}

// isPlaceholderToken 判斷 token 是否仍是範本中的佔位字串。
//
// 比起比對固定的常數字串，這裡一併涵蓋不同大小寫的寫法，避免使用者的
// config.json 還是範本內容時，拿佔位字串去呼叫 Telegram API 而陷入
// 無盡的失敗重試。
func isPlaceholderToken(token string) bool {
	if token == "" {
		return true
	}

	lower := strings.ToLower(token)
	return strings.Contains(lower, "your_bot_token") ||
		strings.Contains(lower, "your-telegram-bot-token")
}
