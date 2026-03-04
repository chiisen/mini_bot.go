package tools

// ============================================================================
// 工具註冊表 (Tool Registry)
// ============================================================================
// 本檔案定義了工具註冊表，用於管理和執行各種工具。
//
// 設計原理：
//   - 工具註冊表是一個工具的容器，儲存所有可用的工具
//   - 每個工具都必須實作 Tool 介面
//   - 透過名稱可以查詢和執行對應的工具
//   - 提供了錯誤處理機制，捕獲工具執行中的 Panic
//
// 工具 (Tool) 必須實作的方法：
//   - Name():        工具的唯一名稱
//   - Description(): 工具的描述，供 LLM 理解工具用途
//   - Parameters():  工具的參數定義 (JSON Schema 格式)
//   - Execute():     工具的執行邏輯
// ============================================================================

import (
	"context"
	"fmt"

	"github.com/chiisen/mini_bot/pkg/providers"
)

// ============================================================================
// ToolRegistry: 工具註冊表結構體
// ============================================================================
// 負責儲存和管理所有已註冊的工具。
type ToolRegistry struct {
	// tools 是一個 map，以工具名稱為鍵儲存工具實例
	// 使用 map 可以實現 O(1) 時間複雜度的工具查詢
	tools map[string]Tool
}

// NewRegistry 建立新的工具註冊表
//
// 回傳：
//   - *ToolRegistry: 初始化好的工具註冊表
func NewRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]Tool),
	}
}

// ============================================================================
// Register: 註冊工具
// ============================================================================
// 將一個工具註冊到註冊表中。
// 如果已經存在同名工具，新的工具會覆蓋舊的。
//
// 參數：
//   - tool: 實作了 Tool 介面的工具實例
//
// 備註：
//   - 工具名稱由 tool.Name() 方法決定
//   - 建議使用簡潔、描述性的名稱 (如 read_file, write_file)
//
// ============================================================================
func (r *ToolRegistry) Register(tool Tool) {
	r.tools[tool.Name()] = tool
}

// ============================================================================
// Execute: 執行工具
// ============================================================================
// 根據工具名稱查詢並執行對應的工具。
// 這是工具系統的核心方法，負責：
//  1. 工具查詢
//  2. 錯誤處理 (Tool Not Found)
//  3. Panic 捕獲 (防止一個工具的錯誤影響整個系統)
//  4. 結果封裝
//
// 參數：
//   - ctx:  上下文物件，用於控制執行時間和取消
//   - name: 工具名稱
//   - args: 工具參數 (map 格式)
//
// 回傳：
//   - *ToolResult: 工具執行結果
//
// 錯誤處理：
//   - 如果工具不存在，返回錯誤訊息
//   - 如果工具執行過程中發生 Panic，捕獲並返回錯誤訊息
//   - 工具內部的錯誤會封裝在 ToolResult.ForLLM 中
//
// ============================================================================
func (r *ToolRegistry) Execute(ctx context.Context, name string, args map[string]any) *ToolResult {
	// 根據名稱查詢工具
	t, ok := r.tools[name]

	// 工具不存在處理
	if !ok {
		return &ToolResult{
			ForLLM:  fmt.Sprintf("Tool '%s' not found.", name), // 錯誤訊息會傳給 LLM
			IsError: true,                                      // 標記為錯誤
		}
	}

	// -------------------------------------------------------------------------
	// Panic 捕獲機制
	// -------------------------------------------------------------------------
	// 使用 defer 和 recover 捕獲執行過程中的 Panic
	// 這確保即使某個工具崩潰，也不會影響整個 Agent 的運行
	var res *ToolResult
	func() {
		defer func() {
			// 檢查是否發生了 Panic
			if rec := recover(); rec != nil {
				// 將 Panic 轉換為錯誤結果
				res = &ToolResult{
					ForLLM:  fmt.Sprintf("Tool panic: %v", rec),
					IsError: true,
				}
			}
		}()
		// 正常執行工具
		res = t.Execute(ctx, args)
	}()

	// 返回執行結果
	return res
}

// ============================================================================
// Definitions: 獲取工具定義列表
// ============================================================================
// 返回所有已註冊工具的定義，用於生成給 LLM 的工具列表。
// 這些定義包含：
//   - 工具名稱
//   - 工具描述
//   - 參數 Schema
//
// 這個方法用於：
//   - 在 Agent 啟動時生成系統提示詞
//   - 讓 LLM 知道有哪些工具可用
//
// 回傳：
//   - []providers.ToolDefinition: 工具定義列表
//
// ============================================================================
func (r *ToolRegistry) Definitions() []providers.ToolDefinition {
	// 預分配切片容量，提高效能
	var defs []providers.ToolDefinition

	// 遍歷所有工具，收集它們的定義
	for _, t := range r.tools {
		// 將 Tool 轉換為 providers.ToolDefinition 格式
		defs = append(defs, providers.ToolDefinition{
			Type: "function", // OpenAI 格式的工具類型
			Function: providers.ToolFunctionDefinition{
				Name:        t.Name(),        // 工具名稱
				Description: t.Description(), // 工具描述
				Parameters:  t.Parameters(),  // 參數定義 (JSON Schema)
			},
		})
	}

	return defs
}
