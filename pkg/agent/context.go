package agent

// ============================================================================
// 上下文建構器 (Context Builder) 與輸入安全處理
// ============================================================================
// 本檔案包含兩個主要功能：
//   1. Builder: 根據工作區的設定檔建構系統提示詞
//   2. SanitizeInput: 準備送進對話的使用者輸入
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
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/chiisen/mini_bot/pkg/i18n"
	"github.com/chiisen/mini_bot/pkg/providers"
)

// ============================================================================
// 常數定義
// ============================================================================

// MaxInputLength 定義使用者輸入的最大長度，以 rune（字元）為單位，
// 避免截斷時把中文或 emoji 切成無效的 UTF-8。
const MaxInputLength = 10000

// inputFence 是包住使用者輸入的界線。
const inputFence = "```"

// injectionPatterns 是常見的提示詞注入字串，用於偵測與記錄。
//
// 這些樣式只用於回報（見 ContainsInjectionPatterns），不會改寫使用者內容：
// 以字串比對擋注入的實際防護效果有限，而改寫內容會讓模型看到不存在的文字。
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

// backtickRun 比對足以關閉界線的連續反引號
var backtickRun = regexp.MustCompile("`{3,}")

// ============================================================================
// SanitizeInput: 使用者輸入處理
// ============================================================================
// 設計原則：**不修改使用者輸入的內容**。
//
// 先前的版本會把 {} [] <script {{ }} 轉義成 HTML entity，並就地把偵測到的注入
// 字串換成 [FILTERED ...]。但這個助理的主要用途正是處理程式碼與 JSON，改寫內容
// 會讓模型讀到不存在的文字，對防護卻幾乎沒有幫助（換句話說就繞過了）。
//
// 現在只做三件事：
//  1. 以 rune 為單位截斷過長輸入，不會切斷中文或 emoji
//  2. 中和足以關閉界線的連續反引號，避免使用者自行收尾後在界線外注入指示
//  3. 用界線包住輸入，讓模型能區分「使用者內容」與「系統指示」
//
// 參數：
//   - input: 原始使用者輸入
//
// 回傳：
//   - 包上界線、可直接送給模型的字串
//
// ============================================================================
func SanitizeInput(input string) string {
	input = truncateRunes(input, MaxInputLength)
	input = neutralizeFence(input)

	return inputFence + "user-input\n" + input + "\n" + inputFence
}

// truncateRunes 以 rune 為單位截斷字串，不會產生無效的 UTF-8。
func truncateRunes(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}

	count := 0
	for i := range s {
		if count == max {
			return s[:i]
		}
		count++
	}
	return s
}

// neutralizeFence 中和輸入中足以關閉界線的連續反引號。
//
// 界線是這個防護的主體：使用者只要能寫出 ``` 就能提前收尾，讓後續文字落在
// 界線之外而看起來像系統指示。這裡只改寫三連以上的反引號（一般程式碼裡的單一
// 或雙反引號不受影響，可讀性保留），長度也維持不變。
func neutralizeFence(s string) string {
	return backtickRun.ReplaceAllStringFunc(s, func(run string) string {
		return strings.Repeat("｀", utf8.RuneCountInString(run))
	})
}

// ContainsInjectionPatterns 回報輸入是否含常見的提示注入字串。
//
// 這是**偵測**而非處理。先前版本會就地把符合的字串換成 [FILTERED ...]，既改寫了
// 使用者原意，也因為大小寫不一致（用轉小寫的副本比對、卻用原字串取代）而經常
// 實際上沒有替換成功。偵測結果交由呼叫端決定是否記錄。
func ContainsInjectionPatterns(input string) bool {
	lower := strings.ToLower(input)

	for _, pattern := range injectionPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}

	return false
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
		for _, tool := range tools {
			toolDesc.WriteString(fmt.Sprintf("- %s: %s\n", tool.Function.Name, tool.Function.Description))
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
