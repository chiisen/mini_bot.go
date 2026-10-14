package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/chiisen/mini_bot/pkg/config"
	"github.com/chiisen/mini_bot/pkg/i18n"
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

	// Language
	report := describeLanguage(i18n.GetInstance().GetLang(), os.Getenv("MINIBOT_LANGUAGE"), cfg.Language)
	if report.Supported {
		fmt.Printf("🌐 Language: %s (from %s)\n", report.Effective, report.Source)
	} else {
		fmt.Printf("⚠️  Language: %q (from %s) 不被支援，實際使用 %s\n",
			report.Requested, report.Source, report.Effective)
	}

	fmt.Printf("✅ Workspace Path: %s\n", cfg.Agents.Defaults.Workspace)

	if _, err := os.Stat(cfg.Agents.Defaults.Workspace); os.IsNotExist(err) {
		fmt.Printf("❌ Workspace directory missing: %s\n", cfg.Agents.Defaults.Workspace)
	} else {
		fmt.Printf("✅ Workspace Directory: Found\n")
	}

	// Telegram
	fmt.Println(telegramStatus(cfg.Channels.Telegram))

	fmt.Println("-------------------------")
	fmt.Println("🚀 Run 'app agent' to start local interactive mode.")
	fmt.Println("🌐 Run 'app gateway' to start background services (Telegram).")

	return nil
}

// languageReport 是要顯示的語言資訊。
type languageReport struct {
	Effective string // 實際生效的語言
	Source    string // "env" | "config" | "default"
	Requested string // 設定值；與 Effective 不同時代表不被支援
	Supported bool
}

// describeLanguage 整理語言設定要顯示的內容。
//
// config.Load 會把 Language 填上預設值，因此單看 cfg.Language 無法分辨「使用者
// 設定的」與「預設值」；這裡用是否等於預設值來判斷來源。另外 i18n.SetLang 對不支援
// 的語言是靜默忽略，先前會直接顯示設定值，看起來像是生效了——這裡改為標示出來。
func describeLanguage(effective, envLang, cfgLang string) languageReport {
	switch {
	case envLang != "":
		return languageReport{
			Effective: effective,
			Source:    "env",
			Requested: envLang,
			Supported: envLang == effective,
		}
	case cfgLang != "" && cfgLang != config.DefaultLanguage:
		return languageReport{
			Effective: effective,
			Source:    "config",
			Requested: cfgLang,
			Supported: cfgLang == effective,
		}
	default:
		return languageReport{Effective: effective, Source: "default", Supported: true}
	}
}

// telegramStatus 回傳 Telegram 頻道的狀態描述。
//
// 佔位字串的判斷與 gateway 共用 isPlaceholderToken，否則 status 可能顯示 Ready、
// 而 gateway 卻跳過該頻道（見 issue #23 的同類問題）。
func telegramStatus(cfg config.TelegramConfig) string {
	if !cfg.Enabled {
		return "ℹ️  Telegram Channel: Disabled"
	}
	if isPlaceholderToken(cfg.Token) {
		return "⚠️  Telegram: Enabled but Token is invalid/default"
	}
	return "✅ Telegram Channel: Ready"
}

// bToMb converts bytes to Megabytes
func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}
