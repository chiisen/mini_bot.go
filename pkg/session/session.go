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
//   - 檔案權限: 0600 (只有所有者可讀寫)
//
// ============================================================================
func (m *Manager) Save(sessionKey string, messages []providers.Message) error {
	// 確保儲存目錄存在。只有走過 `app onboard` 的預設 workspace 才會有 sessions
	// 目錄；以環境變數或 config 指定其他 workspace 時（例如
	// MINIBOT_AGENTS_DEFAULTS_WORKSPACE），若不在這裡建立，寫入就會失敗。
	if err := os.MkdirAll(m.StorageDir, 0755); err != nil {
		return fmt.Errorf("failed to create session directory %s: %w", m.StorageDir, err)
	}

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
	// 權限 0600: rw------- (只有所有者可讀寫；會話內容是完整對話記錄，
	// 可能含程式碼、金鑰與個資，不應讓同機其他使用者讀取)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write session %s: %w", sessionKey, err)
	}

	return nil
}

// ============================================================================
// Compress: 上下文壓縮
// ============================================================================
// 依「token 預算」保留最近的對話，取代先前的固定則數（12 則 / 保留 10 則）。
//
// 固定則數的問題在於它與內容長度無關：10 則含大段程式碼或工具輸出的訊息可能遠超
// 模型上限，而 100 則短訊息則可能遠低於上限卻照樣被丟掉。改成預算之後，短對話不會
// 因為「則數多」被裁切，長對話則會被裁到塞得下。
//
// 壓縮策略：
//  1. 系統提示詞（第一則，若有）一律保留
//  2. 從最新的一則往回累積，直到超出 maxTokens 預算
//  3. 至少保留最後一則訊息——即使它本身就超出預算，也不能讓模型沒有東西可回
//  4. 裁切點往前對齊工具呼叫群組的起點，避免產生孤兒 tool 訊息
//
// 參數：
//   - messages: 原始訊息列表
//   - maxTokens: 這次請求的 token 預算；<= 0 時採用 DefaultContextBudgetTokens
//
// 回傳：
//   - []providers.Message: 壓縮後的訊息列表（未超預算時原樣返回）
//
// 備註：被丟棄的中段目前直接捨棄，沒有摘要。若日後需要保留事實，可另做摘要機制。
// ============================================================================
func (m *Manager) Compress(messages []providers.Message, maxTokens int) []providers.Message {
	if len(messages) <= 1 {
		return messages
	}

	budget := maxTokens
	if budget <= 0 {
		budget = DefaultContextBudgetTokens
	}

	// 系統提示詞一律保留
	var system *providers.Message
	rest := messages
	if messages[0].Role == "system" {
		system = &messages[0]
		rest = messages[1:]
	}

	used := 0
	if system != nil {
		used = estimateMessageTokens(*system)
	}

	// 從最新往回累積，直到超出預算。條件中的 start < len(rest) 確保最後一則一定
	// 會被保留（第一次迭代時 start 等於 len(rest)）。
	start := len(rest)
	for i := len(rest) - 1; i >= 0; i-- {
		cost := estimateMessageTokens(rest[i])
		if used+cost > budget && start < len(rest) {
			break
		}
		used += cost
		start = i
	}

	// 裁切點必須落在工具呼叫群組的起點，不能切在群組中間：`tool` 訊息只能緊接在
	// 發出該呼叫的 assistant 訊息之後，否則 OpenAI 相容 API 會以 400 拒絕整個請求
	// （messages with role 'tool' must be a response to a preceding message with
	// 'tool_calls'）。往前推的代價是可能超出預算，換來的是合法可用的序列。
	for start > 0 && rest[start].Role == "tool" {
		start--
	}

	// 全部都在預算內（或已無可裁切）時原樣返回
	if start == 0 {
		return messages
	}

	compressed := make([]providers.Message, 0, len(rest)-start+1)
	if system != nil {
		compressed = append(compressed, *system)
	}
	compressed = append(compressed, rest[start:]...)

	return compressed
}
