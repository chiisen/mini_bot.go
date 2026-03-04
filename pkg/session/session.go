package session

// ============================================================================
// 對話會話管理 (Session Management)
// ============================================================================
// 本檔案提供對話會話的持久化管理功能。
// 會話管理器負責：
//   - 載入歷史對話記錄
//   - 保存新的對話記錄
//   - 上下文壓縮 (當對話過長時)
//
// 設計原理：
//   - 對話歷史以 JSON 格式保存在磁碟上
//   - 每個會話由一個唯一的 sessionKey 識別
//   - 檔案命名格式: {sessionKey}.json
//   - 支援上下文壓縮以節省 Token 數量
// ============================================================================

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/chiisen/mini_bot/pkg/providers"
)

// ============================================================================
// Manager: 對話會話管理器
// ============================================================================
// 負責管理對話歷史的持久化儲存。
type Manager struct {
	StorageDir string // 儲存目錄路徑，所有會話檔案都保存在此目錄下
}

// NewManager 建立新的會話管理器
//
// 參數：
//   - storageDir: 儲存目錄的路徑
//
// 回傳：
//   - *Manager: 新的會話管理器實例
func NewManager(storageDir string) *Manager {
	return &Manager{
		StorageDir: storageDir,
	}
}

// ============================================================================
// Load: 載入對話歷史
// ============================================================================
// 根據會話鍵從磁碟載入對話歷史記錄。
// 如果會話不存在（即檔案不存在），會返回空的訊息切片而不報錯。
// 這是因為新會話沒有歷史記錄是正常情況。
//
// 參數：
//   - sessionKey: 對話會話的唯一識別鍵
//
// 回傳：
//   - []providers.Message: 對話歷史訊息列表
//   - error: 如果讀取過程中發生錯誤
//
// 檔案格式：
//   - 檔案位置: {StorageDir}/{sessionKey}.json
//   - 檔案格式: JSON 陣列，每個元素是一個 Message 物件
//
// ============================================================================
func (m *Manager) Load(sessionKey string) ([]providers.Message, error) {
	// 構建完整的檔案路徑
	path := filepath.Join(m.StorageDir, sessionKey+".json")

	// 檢查檔案是否存在
	// 如果檔案不存在，返回空的訊息切片（這是新會話的正常情況）
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return []providers.Message{}, nil
	}

	// 讀取檔案內容
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read session %s: %w", sessionKey, err)
	}

	// 解析 JSON 資料
	var messages []providers.Message
	if err := json.Unmarshal(data, &messages); err != nil {
		return nil, fmt.Errorf("failed to parse session %s: %w", sessionKey, err)
	}

	return messages, nil
}

// ============================================================================
// Save: 保存對話歷史
// ============================================================================
// 將對話歷史寫入磁碟進行持久化儲存。
//
// 參數：
//   - sessionKey: 對話會話的唯一識別鍵
//   - messages: 要保存的訊息列表
//
// 回傳：
//   - error: 如果寫入過程中發生錯誤
//
// 檔案格式：
//   - 檔案位置: {StorageDir}/{sessionKey}.json
//   - 檔案格式: JSON，使用 json.MarshalIndent 進行美化格式化
//   - 檔案權限: 0644 (所有者可讀寫， others 可讀)
//
// ============================================================================
func (m *Manager) Save(sessionKey string, messages []providers.Message) error {
	// 構建完整的檔案路徑
	path := filepath.Join(m.StorageDir, sessionKey+".json")

	// 將訊息序列化為 JSON
	// 使用 MarshalIndent 進行美化格式化，便於人類閱讀和調試
	// 參數: data, prefix (前綴), indent (縮排)
	data, err := json.MarshalIndent(messages, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize session %s: %w", sessionKey, err)
	}

	// 寫入檔案
	// 權限 0644: rw-r--r-- (所有者可讀寫，其他使用者可讀)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write session %s: %w", sessionKey, err)
	}

	return nil
}

// ============================================================================
// Compress: 上下文壓縮
// ============================================================================
// 當對話歷史過長時，進行上下文壓縮以節省 Token 數量。
// 這對於處理長對話非常重要，因為大多數 LLM 都有 Token 數量限制。
//
// 壓縮策略 (MVP 版本)：
//  1. 如果訊息數量 <= 12，不進行壓縮
//  2. 總是保留第一條系統訊息 (index 0)
//  3. 保留最後 10 條訊息
//  4. 丟棄中間的訊息
//
// 這種策略的假設：
//   - 系統提示詞是最重要的，需要保留
//   - 最近的對話最相關
//   - 早期的對話歷史相對不重要
//
// 參數：
//   - messages: 原始訊息列表
//   - maxTokens: 最大 Token 數量 (目前未使用，預留給未來更精確的壓縮)
//
// 回傳：
//   - []providers.Message: 壓縮後的訊息列表
//
// ============================================================================
func (m *Manager) Compress(messages []providers.Message, maxTokens int) []providers.Message {
	// 簡單的基於規則的壓縮邏輯 (MVP)
	// 如果訊息數量很少，不需要壓縮
	if len(messages) <= 12 {
		return messages
	}

	var compressed []providers.Message

	// 總是保留系統提示詞 (假設它在第一個位置)
	if len(messages) > 0 && messages[0].Role == "system" {
		compressed = append(compressed, messages[0])
		// 從剩餘訊息中移除已處理的系統訊息
		messages = messages[1:]
	}

	// 保留最後 10 條訊息
	// 這些通常是最重要的上下文
	tailSize := 10
	if len(messages) < tailSize {
		tailSize = len(messages)
	}

	// 追加最後 tailSize 條訊息
	compressed = append(compressed, messages[len(messages)-tailSize:]...)

	return compressed
}
