# 超輕量級個人 AI 助手 — 最小可執行功能規格書 (MVP)

## 📌 專案概述

本專案旨在開發一個**超輕量級個人 AI 助手**，以極低的資源佔用（目標 **<10MB RAM**、**<1 秒啟動**）運行在幾乎任何 Linux 裝置上，包含低至 $10 的開發板。

### 設計原則

| 原則 | 說明 |
|------|------|
| **極致輕量** | 記憶體占用 <10MB，單一可執行檔，無外部依賴 |
| **極速啟動** | 即使在 0.6GHz 單核心上也能 <1 秒啟動 |
| **單一執行檔** | 編譯成靜態連結的單一 binary，跨平台部署 |
| **安全沙箱** | Agent 僅能存取指定 workspace 內的檔案與指令 |

### 推薦技術棧

| 項目 | 推薦選擇 | 理由 |
|------|----------|------|
| **語言** | Go 1.21+ | 編譯成靜態 binary、低記憶體、快速啟動、跨平台原生支援 |
| **LLM 協定** | OpenAI 相容 API | 業界標準，絕大多數 LLM 供應商皆支援 |
| **設定格式** | JSON | 無額外依賴、Go 原生支援 |
| **通訊頻道** | Telegram Bot API | 設定簡單、只需一個 token |

---

## 🏗️ MVP 功能清單

以下功能依優先級分為 **P0（必須）**、**P1（建議）** 兩個等級。

### P0 — 核心必須功能

#### 1. CLI 入口與指令

程式提供以下 CLI 指令：

```
app onboard          # 初始化設定檔與 workspace
app agent -m "..."   # 單次對話模式
app agent            # 互動式聊天模式
app gateway          # 啟動 Telegram 訊息閘道
app version          # 顯示版本資訊
```

#### 2. Agent 核心迴圈

Agent 核心迴圈是系統的心臟，負責以下流程：

```
使用者訊息
    ↓
建構系統 Prompt（載入 IDENTITY.md, AGENT.md 等）
    ↓
附加對話歷史（Session）
    ↓
呼叫 LLM API（附帶工具定義）
    ↓
┌─ LLM 回應文字 → 回傳給使用者
└─ LLM 要求呼叫工具 → 執行工具 → 將結果回饋 LLM → 重複迴圈
```

**關鍵參數：**

| 參數 | 預設值 | 說明 |
|------|--------|------|
| `max_tool_iterations` | 20 | 單次對話中工具呼叫的最大次數 |
| `max_tokens` | 8192 | LLM 回應的最大 token 數 |
| `temperature` | 0.7 | LLM 生成的隨機性 |

#### 3. LLM Provider 抽象層

定義統一的 `LLMProvider` 介面：

```
介面 LLMProvider:
    Chat(context, messages, tools, model, options) → (response, error)
    GetDefaultModel() → string
```

**必須支援的資料結構：**
- `Message`: 角色（system/user/assistant/tool）+ 內容 + 工具呼叫
- `ToolCall`: 工具呼叫請求（ID、函式名、參數）
- `ToolDefinition`: 工具定義（名稱、描述、JSON Schema 參數）
- `LLMResponse`: LLM 回應（內容、工具呼叫列表、token 用量）

**MVP 至少需實作一個 Provider：**
- **OpenAI 相容協定** — 涵蓋 OpenAI、Zhipu、DeepSeek、Groq、OpenRouter 等大部分供應商

採用 `vendor/model` 格式（如 `openai/gpt-4`、`zhipu/glm-4`）來零代碼新增供應商。

#### 4. 工具系統

所有工具實作統一介面：

```
介面 Tool:
    Name() → string
    Description() → string
    Parameters() → JSON Schema
    Execute(context, args) → ToolResult
```

**MVP 必須實作的工具：**

| 工具名 | 功能 | 安全限制 |
|--------|------|----------|
| `read_file` | 讀取檔案內容 | 僅限 workspace 內 |
| `write_file` | 寫入檔案 | 僅限 workspace 內 |
| `list_dir` | 列出目錄內容 | 僅限 workspace 內 |
| `edit_file` | 編輯檔案（行號範圍替換）| 僅限 workspace 內 |
| `append_file` | 追加內容到檔案 | 僅限 workspace 內 |
| `exec` | 執行 shell 指令 | 僅限 workspace 內，封鎖危險指令 |

**安全防護 — 危險指令封鎖清單：**
- `rm -rf`, `del /f`, `rmdir /s`（批量刪除）
- `format`, `mkfs`, `diskpart`（磁碟格式化）
- `dd if=`（磁碟映像）
- `shutdown`, `reboot`, `poweroff`（關機）
- Fork bomb `:(){ :|:& };:`

#### 5. 設定檔系統

設定檔路徑：`~/.appname/config.json`

**最小設定結構：**

```json
{
  "agents": {
    "defaults": {
      "workspace": "~/.appname/workspace",
      "model": "gpt4",
      "max_tokens": 8192,
      "temperature": 0.7,
      "max_tool_iterations": 20,
      "restrict_to_workspace": true
    }
  },
  "model_list": [
    {
      "model_name": "gpt4",
      "model": "openai/gpt-4",
      "api_key": "your-api-key"
    }
  ],
  "channels": {
    "telegram": {
      "enabled": true,
      "token": "YOUR_BOT_TOKEN",
      "allow_from": ["YOUR_USER_ID"]
    }
  }
}
```

支援**環境變數覆寫**，格式為 `APPNAME_SECTION_KEY`（例如 `APPNAME_AGENTS_DEFAULTS_WORKSPACE`）。

#### 6. Workspace 結構與身份系統

初始化時自動建立以下 workspace 結構：

```
~/.appname/workspace/
├── sessions/         # 對話歷史
├── memory/           # 長期記憶 (MEMORY.md)
├── AGENT.md          # Agent 行為指引
├── IDENTITY.md       # Agent 身份描述
├── SOUL.md           # Agent 個性特質
└── USER.md           # 使用者偏好設定
```

Agent 啟動時會讀取這些 Markdown 檔案，組成系統 Prompt 的一部分，讓使用者可自訂 Agent 的行為風格。

#### 7. Session 對話管理

- 每個對話以 Session Key 區分（頻道 + 聊天 ID）
- 對話歷史以 JSON 格式儲存在 `sessions/` 目錄
- 實作**上下文壓縮**：當歷史超過 token 上限時，自動摘要舊對話

#### 8. Telegram 頻道整合

**啟動方式：**
```
app gateway
```

**功能需求：**
- 透過 Telegram Bot API 接收/發送訊息
- 支援 `allow_from` 白名單（限制可互動的使用者 ID）
- 將 Telegram 訊息轉發給 Agent 核心迴圈處理
- 將 Agent 回應發送回 Telegram 聊天

**設定範例：**
```json
{
  "channels": {
    "telegram": {
      "enabled": true,
      "token": "123456:ABC-DEF...",
      "allow_from": ["123456789"]
    }
  }
}
```

---

### P1 — 建議功能（增強體驗）

| 功能 | 說明 |
|------|------|
| **網頁搜尋工具** | 整合 DuckDuckGo（免費、無需 API Key）作為 Web Search 工具 |
| **Onboard 初始化引導** | 互動式引導使用者設定 API Key 與偏好 |
| **狀態檢視指令** | `app status` 顯示設定狀態、已載入工具、workspace 路徑等 |
| **Docker 部署** | 提供 Dockerfile 與 docker-compose.yml |
| **多平台編譯** | 支援 Linux (amd64/arm64/riscv64)、macOS (arm64)、Windows |

---

## 📂 專案目錄結構（建議）

```
project-root/
├── cmd/
│   └── appname/
│       ├── main.go              # 程式入口、CLI 指令路由
│       ├── cmd_agent.go         # agent 指令實作
│       ├── cmd_gateway.go       # gateway 指令實作
│       └── cmd_onboard.go       # onboard 初始化實作
├── pkg/
│   ├── agent/
│   │   ├── instance.go          # AgentInstance：持有 workspace、session、工具
│   │   ├── loop.go              # 核心 Agent 迴圈（LLM 呼叫 + 工具執行）
│   │   ├── context.go           # 系統 Prompt 建構器
│   │   └── memory.go            # 長期記憶管理
│   ├── config/
│   │   ├── config.go            # 設定結構與載入邏輯
│   │   └── defaults.go          # 預設值定義
│   ├── providers/
│   │   ├── types.go             # LLMProvider 介面、Message、ToolCall 等型別
│   │   ├── factory.go           # Provider 工廠：依 vendor/model 建立 provider
│   │   └── openai_compat/       # OpenAI 相容協定實作
│   ├── tools/
│   │   ├── base.go              # Tool 介面定義
│   │   ├── registry.go          # ToolRegistry：工具註冊與執行
│   │   ├── filesystem.go        # read_file, write_file, list_dir, edit_file, append_file
│   │   ├── shell.go             # exec 工具
│   │   └── web.go               # 網頁搜尋工具 (P1)
│   ├── channels/
│   │   ├── manager.go           # 頻道管理器
│   │   └── telegram.go          # Telegram Bot 整合
│   ├── session/
│   │   └── session.go           # Session 管理（歷史記錄存取）
│   ├── bus/
│   │   └── bus.go               # 訊息匯流排（頻道 ↔ Agent 溝通）
│   ├── logger/
│   │   └── logger.go            # 結構化日誌
│   └── state/
│       └── state.go             # 持久狀態管理
├── workspace/                    # 預設 workspace 模板
│   ├── AGENT.md
│   ├── IDENTITY.md
│   ├── SOUL.md
│   └── USER.md
├── config/
│   └── config.example.json      # 設定檔範例
├── Makefile                      # 建置腳本
├── Dockerfile                    # Docker 建置 (P1)
├── go.mod
└── go.sum
```

---

## 🔌 核心介面規格

詳細的介面規格與資料流請參考 [docs/mvp_architecture.md](docs/mvp_architecture.md)。

---

## ✅ 驗收標準

MVP 完成時應達成以下目標：

| 項目 | 驗收條件 |
|------|----------|
| **記憶體** | 靜置時 RSS < 10MB |
| **啟動速度** | 冷啟動 < 1 秒（在一般 x86_64 主機上）|
| **執行檔大小** | 單一 binary < 15MB（strip 後）|
| **基本對話** | `app agent -m "What is 2+2?"` 能正確回應 |
| **工具執行** | Agent 能自主使用工具讀寫檔案、執行指令 |
| **安全沙箱** | 工具無法存取 workspace 以外的路徑 |
| **Telegram** | 透過 Telegram Bot 能正常對話 |
| **設定載入** | config.json 能正確載入並支援環境變數覆寫 |
| **Session** | 對話歷史能持久化並在後續對話中載入 |

---

## 📎 補充文件

- [架構設計與資料流](docs/mvp_architecture.md) — 詳細的模組互動、介面定義與資料流圖
