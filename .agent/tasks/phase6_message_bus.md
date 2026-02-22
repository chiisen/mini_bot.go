# Phase 6 — 訊息匯流排與 CLI 整合

> **前置依賴**：Phase 5（AgentInstance / AgentLoop）  
> **驗收標準**：`app agent -m "..."` 單次對話正常；`app agent`（無參數）進入互動式聊天模式

---

## 任務清單

### T6-1：實作訊息匯流排（MessageBus）

- **狀態**：⬜ 未開始  
- **檔案**：`pkg/bus/bus.go`  
- **說明**：MessageBus 是頻道（來源）與 AgentLoop 之間的中間層，解耦通訊頻道與 Agent 邏輯

  ```go
  // InboundMessage 為入站訊息的統一結構
  type InboundMessage struct {
      Channel    string // "telegram" | "cli"
      ChatID     string // 用於路由回覆
      Content    string // 訊息內容
      SessionKey string // 對話 Session 識別鍵
  }

  type MessageBus struct {
      inbound  chan InboundMessage
      agent    *agent.AgentInstance
  }

  func New(a *agent.AgentInstance) *MessageBus
  func (b *MessageBus) Send(msg InboundMessage)
  func (b *MessageBus) Start(ctx context.Context)  // 啟動處理迴圈
  ```

- **處理邏輯**：
  ```
  接收 InboundMessage
    → 呼叫 agent.Run(sessionKey, content, onReply)
    → onReply 回呼根據 ChatID 路由回覆到對應頻道
  ```

- **驗收**：訊息能正確流經 Bus 到達 Agent 並取得回應

---

### T6-2：實作 CLI agent 指令（單次模式）

- **狀態**：⬜ 未開始  
- **檔案**：`cmd/appname/cmd_agent.go`  
- **說明**：實作 `app agent -m "..."` 單次對話模式

  **流程**：
  1. 載入 Config
  2. 建立 AgentInstance
  3. 以 `cli_default` 為 SessionKey
  4. 呼叫 `agent.Run(ctx, "cli_default", message, printReply)`
  5. 等待回應後退出

- **驗收**：`app agent -m "What is 2+2?"` 能輸出正確回應並退出

---

### T6-3：實作 CLI agent 指令（互動式模式）

- **狀態**：⬜ 未開始  
- **檔案**：`cmd/appname/cmd_agent.go`  
- **說明**：`app agent`（無 `-m` 參數）進入互動式聊天迴圈

  **流程**：
  ```
  顯示歡迎訊息
  loop:
    顯示提示符 "You: "
    讀取用戶輸入（bufio.Scanner）
    if 輸入 == "exit" | "quit" | Ctrl+C → 退出
    呼叫 agent.Run(ctx, sessionKey, input, printReply)
    顯示 "Agent: {回應}"
  ```

- **細節**：
  - 處理 Ctrl+C 優雅退出（`os.Signal`）
  - 每次對話歷史通過 Session 自動持久化

- **驗收**：
  - `app agent` 能進入互動模式
  - 多輪對話中 Agent 能記住上下文
  - 輸入 `exit` 能正常退出

---

### T6-4：實作結構化日誌器

- **狀態**：⬜ 未開始  
- **檔案**：`pkg/logger/logger.go`  
- **說明**：提供簡單的結構化日誌功能（使用 Go 標準庫 `log/slog`，Go 1.21+）

  ```go
  func Init(debug bool)
  func Debug(msg string, args ...any)
  func Info(msg string, args ...any)
  func Warn(msg string, args ...any)
  func Error(msg string, args ...any)
  ```

- **注意**：保持輕量，**不引入** 第三方日誌庫（如 zap、zerolog）

- **驗收**：日誌能正常輸出，debug 模式下顯示更多細節

---

## Phase 6 完成檢查

- [x] T6-1：MessageBus 能正確路由訊息到 Agent
- [x] T6-2：`app agent -m "..."` 單次模式正常
- [x] T6-3：`app agent` 互動式模式正常（含多輪上下文）
- [x] T6-4：結構化日誌正常輸出
