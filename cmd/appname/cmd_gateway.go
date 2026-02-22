package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
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
		if cfg.Channels.Telegram.Token == "" || cfg.Channels.Telegram.Token == "YOUR_BOT_TOKEN_HERE" {
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
