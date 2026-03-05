package main

// ============================================================================
// 程式入口點 (Main Entry Point)
// ============================================================================
// 本程式是一個 CLI (命令列介面) 工具，透過不同的子命令來執行各種功能。
// 主要支援以下幾種命令：
//   - onboard   : 初始化配置和工作區
//   - agent     : 啟動單次對話或互動模式
//   - gateway   : 啟動 Telegram 閘道器
//   - version   : 顯示版本資訊
//   - status    : 顯示系統狀態
//   - help      : 顯示說明資訊
//
// 使用方式：
//   ./appname <command> [arguments]
//
// ============================================================================

import (
	"fmt"
	"os"

	"github.com/chiisen/mini_bot/pkg/i18n"
)

// main 是程式的入口函數，負責解析命令列參數並分發到對應的處理函數。
//
// 命令列參數結構：
//   - os.Args[0] : 程式名稱
//   - os.Args[1] : 子命令 (例如: agent, onboard 等)
//   - os.Args[2:] : 子命令的額外參數
func main() {
	// 檢查是否有提供子命令
	// 如果沒有提供任何參數，顯示說明資訊並退出
	if len(os.Args) < 2 {
		printHelp()
		os.Exit(1)
	}

	// 提取子命令 (第一個參數)
	command := os.Args[1]

	// 提取子命令的額外參數 (從第二個參數開始)
	args := os.Args[2:]

	// 使用 switch 語句根據子命令分發到不同的處理函數
	switch command {
	case "onboard":
		// onboard 命令：初始化配置和工作區
		// 會建立必要的目錄結構和設定檔
		if err := RunOnboard(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "agent":
		// agent 命令：啟動 AI Agent
		// 可用於單次互動 (-m 參數) 或互動式對話模式
		if err := RunAgent(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "gateway":
		// gateway 命令：啟動 Telegram 閘道器
		// 允許透過 Telegram 與 Agent 互動
		if err := RunGateway(args); err != nil {
			t := i18n.GetInstance()
			fmt.Fprintf(os.Stderr, t.T("cli.error")+"\n", err)
			os.Exit(1)
		}
	case "version":
		// version 命令：顯示程式版本
		t := i18n.GetInstance()
		fmt.Printf("%s %s\n", t.T("app.name"), t.T("app.version"))
	case "status":
		// status 命令：顯示系統狀態
		// 可顯示目前的工作區、配置狀態等資訊
		if err := RunStatus(args); err != nil {
			t := i18n.GetInstance()
			fmt.Fprintf(os.Stderr, t.T("cli.error")+"\n", err)
			os.Exit(1)
		}
	case "help", "-h", "--help":
		// help, -h, --help 命令：顯示說明資訊
		printHelp()
	default:
		// 未知命令：顯示錯誤訊息和說明
		t := i18n.GetInstance()
		fmt.Printf(t.T("cli.unknown_command")+"\n", command)
		printHelp()
		os.Exit(1)
	}
}

// printHelp 函數顯示程式的使用說明
//
// 說明內容包括：
//   - 程式的基本使用語法
//   - 所有可用的子命令及其簡短描述
func printHelp() {
	t := i18n.GetInstance()
	fmt.Print(t.T("cli.usage") + `

` + t.T("cli.commands.onboard") + `
    ` + t.T("cli.commands.agent") + `
    ` + t.T("cli.commands.gateway") + `
    ` + t.T("cli.commands.version") + `
    ` + t.T("cli.commands.status") + `
`)
}
