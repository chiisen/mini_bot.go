# Phase 3 — LLM Provider 抽象層

> **前置依賴**：Phase 1 完成  
> **驗收標準**：能透過 OpenAI 相容 API 發送一則訊息並取得回應

---

## 任務清單

### T3-1：定義核心資料結構與介面

- **狀態**：⬜ 未開始  
- **檔案**：`pkg/providers/types.go`  
- **說明**：定義所有 LLM 相關的核心型別  
- **必須定義的結構**：

  ```go
  // Message 代表對話中的一則訊息
  type Message struct {
      Role       string     // "system" | "user" | "assistant" | "tool"
      Content    string
      ToolCalls  []ToolCall
      ToolCallID string
  }

  // ToolCall 代表 LLM 要求呼叫的工具
  type ToolCall struct {
      ID       string
      Type     string       // "function"
      Function FunctionCall
  }

  // FunctionCall 包含函式名稱與 JSON 格式的參數
  type FunctionCall struct {
      Name      string
      Arguments string // JSON 字串
  }

  // LLMResponse 是 LLM 的回應
  type LLMResponse struct {
      Content   string
      ToolCalls []ToolCall
      Usage     UsageInfo
  }

  // UsageInfo Token 用量統計
  type UsageInfo struct {
      PromptTokens     int
      CompletionTokens int
      TotalTokens      int
  }

  // ToolResult 工具執行結果
  type ToolResult struct {
      ForLLM  string
      IsError bool
  }

  // ToolDefinition 給 LLM 的工具定義
  type ToolDefinition struct {
      Type     string
      Function ToolFunctionDefinition
  }

  type ToolFunctionDefinition struct {
      Name        string
      Description string
      Parameters  map[string]any // JSON Schema
  }
  ```

- **驗收**：`go build ./pkg/providers/` 成功

---

### T3-2：定義 LLMProvider 介面

- **狀態**：⬜ 未開始  
- **檔案**：`pkg/providers/types.go`（同上）  
- **說明**：定義統一的 Provider 介面（參考架構文件）

  ```go
  type LLMProvider interface {
      Chat(
          ctx context.Context,
          messages []Message,
          tools []ToolDefinition,
          model string,
          options map[string]any,
      ) (*LLMResponse, error)

      GetDefaultModel() string
  }
  ```

- **驗收**：介面定義可被其他套件使用

---

### T3-3：實作 OpenAI 相容 Provider

- **狀態**：⬜ 未開始  
- **檔案**：`pkg/providers/openai_compat/provider.go`  
- **說明**：實作 OpenAI Chat Completions API（`/v1/chat/completions`）相容協定，涵蓋以下供應商：
  - OpenAI（`api.openai.com`）
  - Zhipu（`open.bigmodel.cn`）
  - DeepSeek（`api.deepseek.com`）
  - Groq、OpenRouter（標準 OpenAI 相容端點）
  - Ollama（`localhost:11434/v1`）

- **實作要點**：
  - 使用 Go 標準庫 `net/http`（**不使用** OpenAI SDK，維持零依賴）
  - 支援 `api_base` 自訂端點
  - 正確處理工具呼叫（tool_use）的請求與回應格式
  - 處理串流（stream）預備：MVP 先用非串流模式

- **重要 JSON 映射**（OpenAI API ↔ 內部結構）：
  ```
  messages[].role          ↔ Message.Role
  messages[].content       ↔ Message.Content
  messages[].tool_calls    ↔ Message.ToolCalls
  messages[].tool_call_id  ↔ Message.ToolCallID
  choices[0].message       ↔ LLMResponse.Content / ToolCalls
  usage                    ↔ LLMResponse.Usage
  tools[]                  ↔ []ToolDefinition
  ```

- **驗收**：能對 OpenAI 或相容 API 發送請求並正確解析回應（含工具呼叫）

---

### T3-4：實作 Provider 工廠

- **狀態**：⬜ 未開始  
- **檔案**：`pkg/providers/factory.go`  
- **說明**：依據 `vendor/model` 格式字串自動路由到對應 Provider 與 API Base

- **Vendor 預設對應表**：
  ```
  "openai"    → https://api.openai.com/v1
  "zhipu"     → https://open.bigmodel.cn/api/paas/v4
  "deepseek"  → https://api.deepseek.com/v1
  "groq"      → https://api.groq.com/openai/v1
  "openrouter"→ https://openrouter.ai/api/v1
  "ollama"    → http://localhost:11434/v1
  ```

- **函式簽名**：
  ```go
  func NewProvider(modelCfg *config.ModelConfig) (LLMProvider, error)
  ```

- **邏輯**：
  1. 解析 `modelCfg.Model` 字串（格式 `vendor/model_id`）
  2. 依 vendor 取得預設 API Base
  3. 若 `modelCfg.APIBase` 非空則覆寫
  4. 建立並返回 OpenAI 相容 Provider 實例

- **驗收**：
  - `NewProvider` 能正確解析 `openai/gpt-4`、`zhipu/glm-4` 等格式
  - 未知 vendor 返回可識別錯誤

---

## Phase 3 完成檢查

- [x] T3-1：核心資料結構定義完整
- [x] T3-2：LLMProvider 介面定義
- [x] T3-3：OpenAI 相容 Provider 能正常呼叫 API
- [x] T3-4：Provider 工廠能正確路由各 vendor
