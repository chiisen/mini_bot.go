package agent

// ============================================================================
// 上下文建構器 (Context Builder) 與輸入安全處理
// ============================================================================
// 本檔案包含兩個主要功能：
//   1. Builder: 根據工作區的設定檔建構系統提示詞
//   2. SanitizeInput: 對使用者輸入進行安全過濾，防止注入攻擊
//
// 系統提示詞的組成部分：
//   - IDENTITY.md     : AI 的身份定義
//   - AGENT.md        : Agent 的行為指南
//   - SOUL.md         : AI 的人格特徵
//   - USER.md         : 使用者偏好設定
//   - MEMORY.md       : 長期記憶 (可選)
//   - AVAILABLE TOOLS : 可用工具列表
// ============================================================================

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/chiisen/mini_bot/pkg/i18n"
	"github.com/chiisen/mini_bot/pkg/providers"
)

// ============================================================================
// 常數定義
// ============================================================================

// MaxInputLength 定義使用者輸入的最大長度
// 超過這個長度的輸入會被截斷
const MaxInputLength = 10000

// injectionPatterns 是用於檢測提示詞注入攻擊的關鍵字列表
// 這些模式常被用於嘗試绕过 AI 的安全限制
var injectionPatterns = []string{
	"ignore previous instructions",    // 忽略之前的指令
	"ignore all previous instruction", // 忽略所有之前的指令
	"disregard previous",              // 忽略之前的
	"forget all instructions",         // 忘記所有指令
	"you are now",                     // 你現在是
	"you are a",                       // 你是個
	"act as",                          // 扮演
	"pretend to be",                   // 假裝是
	"roleplay as",                     // 角色扮演
	"new instructions:",               // 新指令:
	"system:",                         // 系統:
	"assistant:",                      // 助手:
	"human:",                          // 人類:
}

// ============================================================================
// SanitizeInput: 輸入安全處理函數
// ============================================================================
// 這個函數對使用者輸入進行多層安全處理：
//  1. 長度截斷 - 防止過長輸入導致的問題
//  2. 注入攻擊檢測 - 識別並標記常見的提示詞注入模式
//  3. 腳本標籤轉義 - 防止 XSS 攻擊
//  4. 模板變量轉義 - 防止模板注入
//  5. 方括號轉義 - 防止 Markdown 注入
//  6. 格式包裝 - 用 Markdown 程式碼塊包裝輸入
//
// 參數：
//   - input: 原始使用者輸入字串
//
// 回傳：
//   - 經過安全處理後的字串
//
// ============================================================================
func SanitizeInput(input string) string {
	// ---------------------------------------------------------------------
	// 步驟 1: 長度截斷
	// ---------------------------------------------------------------------
	// 如果輸入超過最大長度，則截斷多餘部分
	// 這可以防止過長輸入導致的資源耗盡
	if len(input) > MaxInputLength {
		input = input[:MaxInputLength]
	}

	// ---------------------------------------------------------------------
	// 步驟 2: 提示詞注入檢測
	// ---------------------------------------------------------------------
	// 將輸入轉為小寫以便進行模式匹配
	// 檢查是否包含常見的提示詞注入關鍵字
	lower := strings.ToLower(input)
	for _, pattern := range injectionPatterns {
		if strings.Contains(lower, pattern) {
			// 將檢測到的注入模式替換為警告標記
			// 例如: "ignore previous" -> "[FILTERED IGNORE PREVIOUS]"
			marker := fmt.Sprintf("[FILTERED %s]", strings.ToUpper(pattern))
			input = strings.ReplaceAll(input, pattern, marker)
		}
	}

	// ---------------------------------------------------------------------
	// 步驟 3: 腳本標籤轉義 (防止 XSS)
	// ---------------------------------------------------------------------
	// 將 HTML 腳本標籤轉義，防止跨站腳本攻擊
	input = strings.ReplaceAll(input, "<script", "&lt;script")
	input = strings.ReplaceAll(input, "</script>", "&lt;/script>")

	// ---------------------------------------------------------------------
	// 步驟 4: 模板變量轉義 (防止模板注入)
	// ---------------------------------------------------------------------
	// 轉義 Handlebars/Jinja 等模板引擎的變量語法
	input = strings.ReplaceAll(input, "{{", "&lbrace;&lbrace;")
	input = strings.ReplaceAll(input, "}}", "&rbrace;&rbrace;")
	input = strings.ReplaceAll(input, "{", "&#123;")
	input = strings.ReplaceAll(input, "}", "&#125;")

	// ---------------------------------------------------------------------
	// 步驟 5: 方括號轉義 (防止 Markdown 注入)
	// ---------------------------------------------------------------------
	// 方括號在 Markdown 中有特殊意義 (連結、圖片等)
	// 轉義它們以確保輸入被當作純文本處理
	input = strings.ReplaceAll(input, "[", "&#91;")
	input = strings.ReplaceAll(input, "]", "&#93;")

	// ---------------------------------------------------------------------
	// 步驟 6: 格式包裝
	// ---------------------------------------------------------------------
	// 用 Markdown 程式碼塊包裝處理後的輸入
	// 這有助於 AI 區分使用者輸入和系統指令
	return "```user-input\n" + input + "\n```"
}

// ============================================================================
// Builder: 系統提示詞建構器
// ============================================================================
// Builder 負責根據工作區中的 Markdown 檔案建構完整的系統提示詞。
// 系統提示詞是傳給 LLM 的初始指令，定義了 AI 的身份、能力邊界和行為規則。
type Builder struct {
	WorkspacePath string // 工作區的根目錄路徑
}

// NewContextBuilder 建立一個新的 ContextBuilder 實例
//
// 參數：
//   - workspacePath: 工作區的根目錄路徑
//
// 回傳：
//   - *Builder: 新的建構器實例
func NewContextBuilder(workspacePath string) *Builder {
	return &Builder{
		WorkspacePath: workspacePath,
	}
}

// ============================================================================
// Build: 建構系統提示詞
// ============================================================================
// 這個方法從工作區讀取多個 Markdown 檔案，並將它們組合成一個完整的系統提示詞。
// 檔案按照預定義的順序載入，每個檔案都會被賦予一個區段標題。
//
// 載入順序和對應的區段標題：
//  1. IDENTITY.md       -> [IDENTITY]          : AI 身份定義
//  2. AGENT.md          -> [AGENT GUIDELINES]  : 行為指南
//  3. SOUL.md           -> [PERSONALITY]       : 人格特徵
//  4. USER.md           -> [USER PREFERENCES]  : 使用者偏好
//  5. memory/MEMORY.md  -> [MEMORY]            : 長期記憶 (可選)
//
// 最後，如果提供了工具定義，還會附加工具使用指南。
//
// 參數：
//   - tools: 可用的工具定義列表
//
// 回傳：
//   - string: 完整的系統提示詞
//   - error:  如果建構過程中發生錯誤
//
// ============================================================================
func (b *Builder) Build(tools []providers.ToolDefinition) (string, error) {
	var parts []string // 用於存儲所有部分的切片

	t := i18n.GetInstance()

	// ---------------------------------------------------------------------
	// 步驟 1: 定義要載入的檔案列表
	// ---------------------------------------------------------------------
	// 每個條目包含：
	//   - FileName: 相對於工作區根目錄的檔案路徑
	//   - Header:   載入後要添加的區段標題
	filesToLoad := []struct {
		FileName string
		Header   string
	}{
		{"IDENTITY.md", t.T("sections.identity")},
		{"AGENT.md", t.T("sections.agent_guidelines")},
		{"SOUL.md", t.T("sections.personality")},
		{"USER.md", t.T("sections.user_preferences")},
		{filepath.Join("memory", "MEMORY.md"), t.T("sections.memory")},
	}

	// ---------------------------------------------------------------------
	// 步驟 2: 依序載入每個檔案
	// ---------------------------------------------------------------------
	// 嘗試讀取每個檔案，如果檔案存在且不為空，則添加到提示詞中
	for _, req := range filesToLoad {
		// 構建完整的檔案路徑
		path := filepath.Join(b.WorkspacePath, req.FileName)

		// 嘗試讀取檔案
		// 注意：這裡使用了 err == nil 的慣用法
		// 如果檔案不存在，會靜默忽略而不是返回錯誤
		if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
			// 檔案存在且有內容，添加到 parts 中
			parts = append(parts, req.Header)   // 添加區段標題
			parts = append(parts, string(data)) // 添加檔案內容
		}
	}

	// ---------------------------------------------------------------------
	// 步驟 3: 附加工具使用指南
	// ---------------------------------------------------------------------
	// 如果有可用的工具，附加工具列表和使用說明到提示詞中
	if len(tools) > 0 {
		var toolDesc strings.Builder // 使用 strings.Builder 優化字串拼接
		toolDesc.WriteString(t.T("sections.available_tools") + "\n")
		toolDesc.WriteString("You have access to the following tools:\n")

		// 遍歷所有工具，生成工具列表
		for _, t := range tools {
			toolDesc.WriteString(fmt.Sprintf("- %s: %s\n", t.Function.Name, t.Function.Description))
		}

		// 添加工具使用說明
		toolDesc.WriteString("\nWhen you need to perform an action, output a tool call request. Do not try to hallucinate commands execution in plain text, use the provided tools.\n")

		// 將工具說明添加到 parts 中
		parts = append(parts, toolDesc.String())
	}

	// ---------------------------------------------------------------------
	// 步驟 4: 合併所有部分
	// ---------------------------------------------------------------------
	// 使用兩個換行符連接所有部分，形成完整的系統提示詞
	return strings.Join(parts, "\n\n"), nil
}
