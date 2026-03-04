package agent

// ============================================================================
// Agent 執行迴圈 (Agent Execution Loop)
// ============================================================================
// 這個檔案包含 Agent 的核心執行邏輯，負責：
//   1. 建構系統提示詞 (System Prompt)
//   2. 載入對話歷史記錄
//   3. 與 LLM (大型語言模型) 進行多輪對話
//   4. 執行 AI 請求的工具呼叫
//   5. 儲存更新後的對話歷史
//
// 設計思路：
//   - 採用 ReAct (Reasoning + Acting) 模式
//   - 支援多次工具呼叫迭代直到獲得最終答案
//   - 自動管理對話上下文和歷史記錄
// ============================================================================

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/chiisen/mini_bot/pkg/providers"
)

// ============================================================================
// Run: Agent 主執行函數
// ============================================================================
// 這是 Agent 的核心方法，執行完整的 LLM + 工具呼叫迴圈。
//
// 參數說明：
//   - ctx: Go 的上下文物件，用於控制請求的取消和超時
//   - sessionKey: 對話會話的唯一識別鍵，用於載入/儲存對話歷史
//   - userInput: 使用者輸入的訊息
//   - onReply: 回調函數，當 AI 回覆訊息時會被呼叫
//
// 回傳值：
//   - error: 如果執行過程中發生錯誤，回傳錯誤訊息
//
// 執行流程：
//  1. 建構系統提示詞 (包含身份、指南、可用工具等)
//  2. 從磁碟載入對話歷史
//  3. 組裝完整的訊息列表 (系統訊息 + 歷史訊息 + 使用者新輸入)
//  4. 進入工具呼叫迴圈 (最多 N 次迭代):
//     a. 傳送訊息給 LLM
//     b. 如果 LLM 回覆文字，呼叫 onReply 回調
//     c. 如果 LLM 請求工具呼叫，執行工具並將結果傳回給 LLM
//     d. 重複步驟 a 直到 LLM 不再請求工具或達到最大迭代次數
//  5. 儲存更新後的對話歷史到磁碟
//
// ============================================================================
func (a *AgentInstance) Run(
	ctx context.Context,
	sessionKey string,
	userInput string,
	onReply func(msg string),
) error {

	// -------------------------------------------------------------------------
	// 步驟 1: 建構系統提示詞 (Build System Prompt)
	// -------------------------------------------------------------------------
	// 從工具註冊表取得所有可用工具的定義
	toolDefs := a.Registry.Definitions()

	// 使用 ContextBuilder 根據工作區設定檔建構系統提示詞
	// 系統提示詞包含：身份定義、Agent 指南、性格特徵、使用者偏好、可用工具列表
	systemPrompt, err := a.CtxBuilder.Build(toolDefs)
	if err != nil {
		return fmt.Errorf("failed to build system context: %w", err)
	}

	// -------------------------------------------------------------------------
	// 步驟 2: 載入對話歷史 (Load Session History)
	// -------------------------------------------------------------------------
	// 根據 sessionKey 從磁碟載入之前的對話記錄
	// 如果是新的 sessionKey，會回傳空的訊息切片
	history, err := a.Sessions.Load(sessionKey)
	if err != nil {
		return fmt.Errorf("failed to load session %s context: %w", sessionKey, err)
	}

	// -------------------------------------------------------------------------
	// 步驟 3: 組裝完整的訊息列表 (Assemble Messages)
	// -------------------------------------------------------------------------
	// 我們將 SYSTEM 提示詞放在索引 0 的位置，確保每次都是最新的
	// 訊息列表的順序：系統訊息 -> 歷史訊息 -> 使用者新輸入
	messages := make([]providers.Message, 0, len(history)+2)

	// 加入系統提示詞 (包含 AI 的身份和工具定義)
	messages = append(messages, providers.Message{
		Role:    "system",
		Content: systemPrompt,
	})

	// 加入之前的對話歷史
	messages = append(messages, history...)

	// 加入新的使用者輸入
	// 注意：輸入會經過 SanitizeInput 函數進行安全處理
	messages = append(messages, providers.Message{
		Role:    "user",
		Content: SanitizeInput(userInput),
	})

	// 選擇性：壓縮上下文以節省 Token
	// 如果訊息數量超過設定的閾值，會自動壓縮上下文
	messages = a.Sessions.Compress(messages, a.Config.Agents.Defaults.MaxTokens)

	// -------------------------------------------------------------------------
	// 步驟 4: 工具呼叫迴圈 (Tool Execution Loop)
	// -------------------------------------------------------------------------
	// 設定最大迭代次數，防止無限迴圈
	// 每次迭代代表一次 LLM 呼叫 + 可能的工具執行
	maxIters := a.Config.Agents.Defaults.MaxToolIterations
	iterations := 0

	// 進入主要的對話迴圈
	// 迴圈會持續直到：
	//   - LLM 不再請求工具呼叫
	//   - 達到最大迭代次數
	//   - 上下文被取消 (ctx.Err())
	for iterations < maxIters {
		// 檢查上下文是否被取消
		if err := ctx.Err(); err != nil {
			return err
		}
		iterations++

		// 處理模型名稱
		// 有些 LLM 提供者使用 "provider/model" 的格式 (例如: openai/gpt-4)
		// 我們需要去除提供者前綴，只保留模型名稱
		modelName := a.Config.Agents.Defaults.Model
		if parts := strings.SplitN(modelName, "/", 2); len(parts) == 2 {
			modelName = parts[1]
		}

		// 發送請求給 LLM
		// 參數：
		//   - ctx: 上下文控制
		//   - messages: 對話歷史
		//   - toolDefs: 工具定義列表
		//   - modelName: 模型名稱
		//   - options: 模型選項 (溫度、最大 token 數等)
		response, err := a.Provider.Chat(ctx, messages, toolDefs, modelName, map[string]any{
			"temperature": a.Config.Agents.Defaults.Temperature,
			"max_tokens":  a.Config.Agents.Defaults.MaxTokens,
		})

		// 錯誤處理
		if err != nil {
			return fmt.Errorf("llm chat provider error: %w", err)
		}

		// 將 LLM 的原始回覆加入到對話歷史中
		// 這樣 LLM 就能「記住」自己說了什麼或呼叫了哪些工具
		// 注意：OpenAI API 要求精確地將 tool_calls 回顯到對話歷史中
		messages = append(messages, providers.Message{
			Role:      "assistant",
			Content:   response.Content,
			ToolCalls: response.ToolCalls,
		})

		// 如果 AI 回覆了文字內容，透過回調函數通知呼叫者
		if response.Content != "" {
			onReply(response.Content)

			// 如果沒有工具呼叫，則退出迴圈 (獲得最終答案)
			// 注意：有些 LLM 可能同時輸出文字和工具呼叫
			if len(response.ToolCalls) == 0 {
				break
			}
		}

		// -------------------------------------------------------------------------
		// 處理工具呼叫 (Handle Tool Calls)
		// -------------------------------------------------------------------------
		// 如果 LLM 請求執行工具，逐一處理每個工具呼叫
		if len(response.ToolCalls) > 0 {
			// 遍歷所有工具呼叫
			for _, call := range response.ToolCalls {
				// 解析工具參數
				// LLM 傳來的參數是 JSON 格式的字符串，需要反序列化為 map
				var args map[string]any
				if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
					// 如果解析失敗，將錯誤訊息傳回給 LLM
					messages = append(messages, providers.Message{
						Role:       "tool",
						Content:    fmt.Sprintf("Failed to parse tool arguments: %v", err),
						ToolCallID: call.ID,
					})
					continue // 繼續處理下一個工具呼叫
				}

				// 執行工具
				// 透過工具註冊表查找並執行對應的工具
				onReply(fmt.Sprintf("[Agent uses tool: %s...]", call.Function.Name))
				result := a.Registry.Execute(ctx, call.Function.Name, args)

				// 將工具執行結果傳回給 LLM
				// 這樣 LLM 可以根據結果產生最終回覆
				messages = append(messages, providers.Message{
					Role:       "tool",
					Content:    result.ForLLM,
					ToolCallID: call.ID,
				})
			}
			// 繼續迴圈，將工具輸出傳回給 LLM
			// LLM 可能會根據工具結果產生回覆或請求更多工具
		} else {
			// 沒有工具請求，且已經處理完文字回覆，退出迴圈
			break
		}
	}

	// 檢查是否因為達到最大迭代次數而退出
	if iterations >= maxIters {
		onReply("[Agent stopped: reached maximum tool iteration limit]")
	}

	// -------------------------------------------------------------------------
	// 步驟 5: 儲存對話歷史 (Save Updated Conversation History)
	// -------------------------------------------------------------------------
	// 將更新後的對話歷史儲存到磁碟，供未來的會話使用
	// 注意：我們在儲存前去除系統提示詞，避免重複複製

	// 建立新的切片，只包含非系統訊息
	var historyToSave []providers.Message
	for _, m := range messages {
		if m.Role != "system" {
			historyToSave = append(historyToSave, m)
		}
	}

	// 儲存到磁碟
	if err := a.Sessions.Save(sessionKey, historyToSave); err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}

	return nil
}
