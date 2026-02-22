# Phase 5 — Agent 核心迴圈

> **前置依賴**：Phase 2（Config）、Phase 3（LLM Provider）、Phase 4（工具系統）  
> **驗收標準**：`app agent -m "What is 2+2?"` 能正確呼叫 LLM 並回傳答案；Agent 能自主使用工具

---

## 任務清單

### T5-1：實作 Session 管理器

- **狀態**：⬜ 未開始  
- **檔案**：`pkg/session/session.go`  
- **說明**：管理每個對話的歷史記錄，以 JSON 格式持久化

  ```go
  type Manager struct {
      StorageDir string
  }

  func NewManager(storageDir string) *Manager

  // Load 載入 Session 的對話歷史
  func (m *Manager) Load(sessionKey string) ([]providers.Message, error)

  // Save 儲存 Session 的對話歷史
  func (m *Manager) Save(sessionKey string, messages []providers.Message) error

  // 上下文壓縮：當 token 估計超過上限時摘要舊對話
  func (m *Manager) Compress(messages []providers.Message, maxTokens int) []providers.Message
  ```

- **SessionKey 格式**：`{channel}_{chatID}`（例：`cli_default`、`telegram_123456789`）  
- **儲存路徑**：`{workspace}/sessions/{sessionKey}.json`

- **驗收**：
  - 第一次載入不存在的 session 返回空列表（非錯誤）
  - 儲存後再載入資料一致

---

### T5-2：實作 Context 建構器

- **狀態**：⬜ 未開始  
- **檔案**：`pkg/agent/context.go`  
- **說明**：讀取 workspace 中的身份文件，組合成 LLM 的 system prompt

  ```go
  type Builder struct {
      WorkspacePath string
  }

  // Build 建構系統 Prompt
  // 讀取順序：IDENTITY.md → AGENT.md → SOUL.md → USER.md → memory/MEMORY.md
  // 並附加可用工具列表描述
  func (b *Builder) Build(tools []providers.ToolDefinition) (string, error)
  ```

- **System Prompt 組合格式**：
  ```
  [IDENTITY]
  {IDENTITY.md 內容}

  [AGENT GUIDELINES]
  {AGENT.md 內容}

  [PERSONALITY]
  {SOUL.md 內容}

  [USER PREFERENCES]
  {USER.md 內容}

  [MEMORY]
  {memory/MEMORY.md 內容（若存在）}
  ```

- **注意**：若文件不存在，該段落跳過（不視為錯誤）

- **驗收**：`Build()` 能正確讀取並組合現有文件

---

### T5-3：實作 AgentInstance

- **狀態**：⬜ 未開始  
- **檔案**：`pkg/agent/instance.go`  
- **說明**：Agent 實例，持有 workspace、session、工具等相依性

  ```go
  type AgentInstance struct {
      Config      *config.Config
      Provider    providers.LLMProvider
      Registry    *tools.ToolRegistry
      Sessions    *session.Manager
      CtxBuilder  *Builder
      WorkspaceDir string
  }

  func NewInstance(cfg *config.Config) (*AgentInstance, error)
  ```

- **`NewInstance` 職責**：
  1. 呼叫 `config.FindModel` 取得模型設定
  2. 呼叫 `providers.NewProvider` 建立 LLM Provider
  3. 建立 ToolRegistry 並註冊所有工具（含沙箱設定）
  4. 建立 Session Manager
  5. 建立 Context Builder

- **驗收**：`NewInstance` 能正確初始化所有組件（含錯誤處理）

---

### T5-4：實作 AgentLoop（核心迴圈）

- **狀態**：⬜ 未開始  
- **檔案**：`pkg/agent/loop.go`  
- **說明**：接收用戶訊息並執行完整的 LLM + 工具迴圈（**最關鍵的模組**）

  ```go
  // Run 執行一次完整的 Agent 對話反應
  // sessionKey: Session 識別鍵
  // userInput: 用戶輸入訊息
  // onReply: 每當有回覆時呼叫（支援中間過程回覆）
  func (a *AgentInstance) Run(
      ctx context.Context,
      sessionKey string,
      userInput string,
      onReply func(msg string),
  ) error
  ```

- **完整流程**（對應序列圖）：
  1. **建構系統 Prompt**：`CtxBuilder.Build(tools)`
  2. **載入對話歷史**：`Sessions.Load(sessionKey)`
  3. **組裝 messages**：`[system] + history + [user: userInput]`
  4. **工具迴圈**（最多 `MaxToolIterations` 次）：
     ```
     呼叫 LLM → 取得回應
     若回應包含文字 → 呼叫 onReply，結束迴圈
     若回應包含工具呼叫 →
       for each tool_call:
         Execute(tool_name, args) → ToolResult
         將 tool_call + result 加入 messages
       繼續迴圈
     ```
  5. **儲存更新的對話歷史**：`Sessions.Save(sessionKey, messages)`

- **邊界條件**：
  - 超過 `MaxToolIterations` 時返回錯誤（防止無限迴圈）
  - LLM 呼叫失敗應有重試或明確錯誤回報

- **驗收**：
  - 純文字問答正常
  - LLM 請求工具時能自動執行並繼續對話
  - 超過最大迭代次數時優雅退出

---

### T5-5：實作長期記憶管理

- **狀態**：⬜ 未開始  
- **檔案**：`pkg/agent/memory.go`  
- **說明**：`memory/MEMORY.md` 的讀寫管理，供 Agent 自主記錄重要資訊

  ```go
  type MemoryManager struct {
      MemoryPath string
  }

  func (m *MemoryManager) Read() (string, error)
  func (m *MemoryManager) Append(content string) error
  ```

- **注意**：MVP 階段僅需讀寫功能；自動記憶觸發機制為 P1

- **驗收**：能正確讀取與追加 MEMORY.md 內容

---

## Phase 5 完成檢查

- [x] T5-1：Session 儲存與載入正常
- [x] T5-2：Context Builder 正確組合 system prompt
- [x] T5-3：AgentInstance 正確初始化所有組件
- [x] T5-4：AgentLoop 完整流程（包含工具呼叫迴圈）正常
- [x] T5-5：MEMORY.md 讀寫正常
