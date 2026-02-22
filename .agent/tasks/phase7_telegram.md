# Phase 7 — Telegram 頻道整合

> **前置依賴**：Phase 6（MessageBus）  
> **驗收標準**：`app gateway` 啟動後，透過 Telegram Bot 能正常多輪對話，且只有 `allow_from` 白名單內的使用者可互動

---

## 任務清單

### T7-1：實作 Telegram Bot 頻道

- **狀態**：⬜ 未開始  
- **檔案**：`pkg/channels/telegram.go`  
- **說明**：使用 Telegram Bot API 實作訊息接收與發送（**不使用第三方 SDK**，直接用 `net/http` 呼叫 Telegram API）

- **Telegram API 端點**（使用 long polling）：
  ```
  GET  https://api.telegram.org/bot{token}/getUpdates?offset={offset}&timeout=30
  POST https://api.telegram.org/bot{token}/sendMessage
  ```

- **需實作的結構**：
  ```go
  type TelegramChannel struct {
      Token     string
      AllowFrom map[string]bool // 白名單 set
      Bus       *bus.MessageBus
  }

  func NewTelegramChannel(cfg *config.TelegramConfig, bus *bus.MessageBus) *TelegramChannel

  // Start 啟動 Long Polling 迴圈（blocking）
  func (t *TelegramChannel) Start(ctx context.Context) error

  // SendMessage 發送訊息到指定 ChatID
  func (t *TelegramChannel) SendMessage(chatID string, text string) error
  ```

- **Long Polling 流程**：
  ```
  loop:
    GET /getUpdates?offset={lastUpdateID+1}&timeout=30
    for each update:
      若 update.message.from.id 不在 allow_from → 忽略
      建立 InboundMessage{
          Channel:    "telegram",
          ChatID:     update.message.chat.id,
          Content:    update.message.text,
          SessionKey: "telegram_" + chatID,
      }
      Bus.Send(InboundMessage)
      更新 lastUpdateID
  ```

- **驗收**：
  - 能接收 Telegram 訊息並轉發給 AgentLoop
  - Agent 回應能正確發送回 Telegram 聊天

---

### T7-2：實作 allow_from 白名單過濾

- **狀態**：⬜ 未開始  
- **說明**：整合在 T7-1 的 Telegram Channel 中

- **白名單邏輯**：
  - `config.json` 中 `channels.telegram.allow_from` 為字串陣列（使用者 ID）
  - 啟動時將陣列轉為 `map[string]bool` 以 O(1) 查詢
  - 每則訊息到達時驗證 `update.message.from.id`（轉字串後比對）
  - 不在白名單的訊息：靜默忽略（不回覆，不記錄到 Agent）

- **驗收**：
  - 白名單內的使用者能正常對話
  - 白名單外的訊息被忽略（可在 debug 日誌中記錄）

---

### T7-3：實作頻道管理器

- **狀態**：⬜ 未開始  
- **檔案**：`pkg/channels/manager.go`  
- **說明**：管理多個頻道的統一啟動與停止

  ```go
  type Manager struct {
      channels []Channel
  }

  type Channel interface {
      Start(ctx context.Context) error
  }

  func (m *Manager) Register(ch Channel)
  func (m *Manager) StartAll(ctx context.Context) error  // 並發啟動所有頻道
  ```

- **驗收**：Manager 能並發啟動多個頻道

---

### T7-4：實作 gateway 指令

- **狀態**：⬜ 未開始  
- **檔案**：`cmd/appname/cmd_gateway.go`  
- **說明**：`app gateway` 指令啟動所有已設定的通訊頻道

  **流程**：
  1. 載入 Config
  2. 建立 AgentInstance
  3. 建立 MessageBus
  4. 依 Config 中 `channels.telegram.enabled` 決定是否啟動 Telegram 頻道
  5. 啟動所有頻道（`Manager.StartAll`）
  6. 等待 Ctrl+C 信號優雅退出

- **驗收**：
  - `app gateway` 能成功連接 Telegram 並開始監聽
  - Ctrl+C 能正常停止所有頻道

---

## Phase 7 完成檢查

- [x] T7-1：Telegram Long Polling 能接收訊息並轉發給 Agent
- [x] T7-2：allow_from 白名單過濾有效
- [x] T7-3：頻道管理器能並發管理多個頻道
- [x] T7-4：`app gateway` 能啟動 Telegram 並正常對話
