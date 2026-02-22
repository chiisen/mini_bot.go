package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/chiisen/mini_bot/pkg/config"
)

// RunStatus handles the 'app status' command.
func RunStatus(args []string) error {
	fmt.Println("🔍 MiniBot.go System Status")
	fmt.Println("-------------------------")

	// Print Runtime Resources
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Printf("🖥️  OS / Arch: %s / %s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("🧵 Goroutines: %d\n", runtime.NumGoroutine())
	// Alloc is bytes of allocated heap objects. Sys is total bytes of memory obtained from the OS.
	fmt.Printf("📊 Memory usage: Alloc = %v MiB, Sys = %v MiB\n", bToMb(m.Alloc), bToMb(m.Sys))
	fmt.Println("-------------------------")

	// Check configuration
	configPath := expandHome("~/.minibot.go/config.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Printf("⚠️  Config file: Missing at %s\n", configPath)
		fmt.Println("💡 Tip: Run 'app onboard' to initialize.")
		return nil
	}
	fmt.Printf("✅ Config file: Found at %s\n", configPath)

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Printf("❌ Config parse error: %v\n", err)
		return nil
	}

	fmt.Printf("✅ Default Model: %s\n", cfg.Agents.Defaults.Model)
	fmt.Printf("✅ Workspace Path: %s\n", cfg.Agents.Defaults.Workspace)

	if _, err := os.Stat(cfg.Agents.Defaults.Workspace); os.IsNotExist(err) {
		fmt.Printf("❌ Workspace directory missing: %s\n", cfg.Agents.Defaults.Workspace)
	} else {
		fmt.Printf("✅ Workspace Directory: Found\n")
	}

	// Telegram
	if cfg.Channels.Telegram.Enabled {
		if cfg.Channels.Telegram.Token == "" || cfg.Channels.Telegram.Token == "YOUR_BOT_TOKEN_HERE" {
			fmt.Printf("⚠️  Telegram: Enabled but Token is invalid/default\n")
		} else {
			fmt.Printf("✅ Telegram Channel: Ready\n")
		}
	} else {
		fmt.Printf("ℹ️  Telegram Channel: Disabled\n")
	}

	fmt.Println("-------------------------")
	fmt.Println("🚀 Run 'app agent' to start local interactive mode.")
	fmt.Println("🌐 Run 'app gateway' to start background services (Telegram).")

	return nil
}

// bToMb converts bytes to Megabytes
func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}
