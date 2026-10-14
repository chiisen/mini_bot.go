package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/chiisen/mini_bot/pkg/agent"
	"github.com/chiisen/mini_bot/pkg/bus"
	"github.com/chiisen/mini_bot/pkg/channels"
	"github.com/chiisen/mini_bot/pkg/config"
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
	manager := channels.NewManager()

	if cfg.Channels.Telegram.Enabled {
		if isPlaceholderToken(cfg.Channels.Telegram.Token) {
			logger.Warn("Telegram is enabled but no valid bot token provided. Skipping Telegram.")
		} else {
			tgChan := channels.NewTelegramChannel(&cfg.Channels.Telegram, messageBus)
			manager.Register(tgChan)
			logger.Info("Registered Telegram Channel")
		}
	} else {
		logger.Info("Telegram is disabled, skipping.")
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
