package main

// ============================================================================
// Agent 命令處理程式 (Agent Command Handler)
// ============================================================================
// 本檔案實作了 'app agent' 命令的處理邏輯。
// 這個命令有兩種運行模式：
//   1. 單次互動模式 (Single Message Mode): 使用 -m 參數傳入單次輸入
//   2. 互動式對話模式 (Interactive Mode): 啟動持續的對話終端
//
// 功能說明：
//   - 載入應用程式配置
//   - 建立 Agent 實例
//   - 處理命令列參數
//   - 執行單次或持續的對話
//   - 優雅地處理中斷信號 (Ctrl+C)
// ============================================================================

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/chiisen/mini_bot/pkg/agent"
	"github.com/chiisen/mini_bot/pkg/config"
	"github.com/chiisen/mini_bot/pkg/i18n"
	"github.com/chiisen/mini_bot/pkg/logger"
)

// ============================================================================
// RunAgent: Agent 命令的入口函數
// ============================================================================
// 處理 'app agent' 和 'app agent -m "..."' 命令的執行。
//
// 參數：
//   - args: 命令列參數列表 (不包含 'agent' 本身)
//
// 回傳：
//   - error: 如果執行過程中發生錯誤
//
// 執行流程：
//  1. 初始化日誌系統
//  2. 載入應用程式配置
//  3. 建立 Agent 實例
//  4. 建立上下文和取消函數
//  5. 解析命令列參數，判斷運行模式
//  6. 根據模式執行：
//     - 單次模式: 執行一次對話並返回
//     - 互動模式: 進入迴圈讀取使用者輸入
//
// ============================================================================
func RunAgent(args []string) error {
	// -------------------------------------------------------------------------
	// 步驟 1: 初始化日誌系統
	// -------------------------------------------------------------------------
	// 初始化 logger，false 表示不使用調試模式
	// 注意：未來可以加入 --debug 參數來啟用調試模式
	logger.Init(false)

	// -------------------------------------------------------------------------
	// 步驟 2: 載入應用程式配置
	// -------------------------------------------------------------------------
	// 從預設位置載入配置檔案
	// 配置檔案包含：
	//   - LLM 模型設定
	//   - API 金鑰和端點
	//   - Agent 行為參數 (溫度、最大 Token 等)
	//   - 工作區路徑
	//
	// 錯誤提示 "Have you run 'onboard'?" 引導使用者執行初始化
	cfg, err := config.Load("~/.minibot.go/config.json")
	if err != nil {
		return fmt.Errorf("failed to load config: %w. Have you run 'onboard'?", err)
	}

	// -------------------------------------------------------------------------
	// 步驟 3: 建立 Agent 實例
	// -------------------------------------------------------------------------
	// 根據配置建立 Agent 實例
	// 這會初始化所有必要的元件：
	//   - LLM 提供者
	//   - 工具註冊表
	//   - 對話會話管理器
	//   - 上下文建構器
	instance, err := agent.NewInstance(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize agent instance: %w", err)
	}

	// -------------------------------------------------------------------------
	// 步驟 4: 建立上下文
	// -------------------------------------------------------------------------
	// 建立可取消的上下文
	// 這用於優雅地處理中斷信號 (Ctrl+C)
	ctx, cancel := context.WithCancel(context.Background())
	// 確保函數返回時取消上下文，釋放資源
	defer cancel()

	// -------------------------------------------------------------------------
	// 步驟 5: 解析命令列參數
	// -------------------------------------------------------------------------
	// 檢查是否為單次互動模式
	// 單次模式使用 -m 參數指定輸入訊息
	isSingleMessage := false
	var singleMessageContent string

	// 遍歷參數，查找 -m 標誌
	for i, arg := range args {
		if arg == "-m" && i+1 < len(args) {
			isSingleMessage = true
			singleMessageContent = args[i+1] // -m 後面的內容作為輸入
			break
		}
	}

	// -------------------------------------------------------------------------
	// 步驟 6: 執行對話
	// -------------------------------------------------------------------------
	// 設定預設的會話鍵
	// 所有的 CLI 互動都使用相同的會話鍵 "cli_default"
	sessionKey := "cli_default"

	// 定義回調函數，用於處理 Agent 的回覆
	// 當 Agent 產生回覆時，這個函數會被調用
	printReply := func(msg string) {
		fmt.Printf("Agent: %s\n", msg)
	}

	// 根據模式執行
	if isSingleMessage {
		// -----------------------------------------------------------------
		// 單次互動模式
		// -----------------------------------------------------------------
		// 執行一次對話並返回結果
		// 適用於腳本化使用場景
		return instance.Run(ctx, sessionKey, singleMessageContent, printReply)
	}

	// -----------------------------------------------------------------
	// 互動式對話模式
	// -----------------------------------------------------------------
	// 進入持續的對話迴圈
	// 使用者可以輸入多條訊息，Agent 會即時回覆

	// 顯示歡迎訊息
	t := i18n.GetInstance()
	fmt.Println(t.T("agent.start"))

	// 建立通道接收系統中斷信號 (Ctrl+C)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	// 啟動 goroutine 處理中斷信號
	// 當收到中斷信號時，取消上下文並退出程式
	go func() {
		<-c // 阻塞直到收到信號
		fmt.Println("\nInterrupt received. Exiting...")
		cancel()   // 取消上下文
		os.Exit(0) // 正常退出程式
	}()

	// 建立 Scanner 從標準輸入讀取使用者輸入
	scanner := bufio.NewScanner(os.Stdin)

	// 主對話迴圈
	for {
		// 顯示提示符
		fmt.Print(t.T("agent.prompt"))

		// 讀取一行輸入
		if !scanner.Scan() {
			break // 如果無法讀取，退出迴圈
		}

		// 取得輸入內容並去除首尾空白
		input := strings.TrimSpace(scanner.Text())

		// 檢查退出命令
		// 使用者可以輸入 "exit" 或 "quit" 結束對話
		if input == "exit" || input == "quit" {
			break
		}

		// 跳過空行
		if input == "" {
			continue
		}

		// 執行對話
		// 呼叫 Agent 實例的 Run 方法處理輸入
		if err := instance.Run(ctx, sessionKey, input, printReply); err != nil {
			// 發生錯誤時顯示錯誤訊息，但繼續迴圈
			fmt.Printf(t.T("cli.error")+"\n", err)
		}
	}

	return nil
}
