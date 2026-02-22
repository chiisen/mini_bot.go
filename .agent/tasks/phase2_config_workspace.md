# Phase 2 — 設定檔與 Workspace 系統

> **前置依賴**：Phase 1 完成  
> **驗收標準**：能正確讀取 `config.json`，支援環境變數覆寫，`app onboard` 能初始化 workspace

---

## 任務清單

### T2-1：定義設定結構（Config struct）

- **狀態**：⬜ 未開始  
- **檔案**：`pkg/config/config.go`  
- **說明**：定義與 JSON 設定檔對應的 Go struct  
- **核心結構**：
  ```go
  type Config struct {
      Agents    AgentsConfig           `json:"agents"`
      ModelList []ModelConfig          `json:"model_list"`
      Channels  ChannelsConfig         `json:"channels"`
  }

  type AgentsConfig struct {
      Defaults AgentDefaults `json:"defaults"`
  }

  type AgentDefaults struct {
      Workspace           string  `json:"workspace"`
      Model               string  `json:"model"`
      MaxTokens           int     `json:"max_tokens"`
      Temperature         float64 `json:"temperature"`
      MaxToolIterations   int     `json:"max_tool_iterations"`
      RestrictToWorkspace bool    `json:"restrict_to_workspace"`
  }

  type ModelConfig struct {
      ModelName string `json:"model_name"`
      Model     string `json:"model"`
      APIKey    string `json:"api_key"`
      APIBase   string `json:"api_base,omitempty"`
  }

  type ChannelsConfig struct {
      Telegram TelegramConfig `json:"telegram"`
  }

  type TelegramConfig struct {
      Enabled   bool     `json:"enabled"`
      Token     string   `json:"token"`
      AllowFrom []string `json:"allow_from"`
  }
  ```
- **驗收**：`go build ./pkg/config/` 成功

---

### T2-2：定義預設值（defaults.go）

- **狀態**：⬜ 未開始  
- **檔案**：`pkg/config/defaults.go`  
- **說明**：提供預設設定值（程式碼層最低優先級）  
- **預設值**：
  ```go
  const (
      DefaultWorkspace         = "~/.minibot/workspace"
      DefaultModel             = "gpt4"
      DefaultMaxTokens         = 8192
      DefaultTemperature       = 0.7
      DefaultMaxToolIterations = 20
      DefaultRestrictToWS      = true
      DefaultConfigDir         = "~/.minibot"
      DefaultConfigFile        = "~/.minibot/config.json"
  )
  ```
- **驗收**：常數可被其他套件引用

---

### T2-3：實作設定檔載入邏輯

- **狀態**：⬜ 未開始  
- **檔案**：`pkg/config/config.go`  
- **說明**：依照三層優先序載入設定
  ```
  1. 預設值（程式碼內建）
       ↓
  2. config.json 檔案
       ↓
  3. 環境變數覆寫（MINIBOT_SECTION_KEY）
  ```
- **函式簽名**：
  ```go
  func Load(configPath string) (*Config, error)
  func applyDefaults(cfg *Config)
  func applyEnvOverrides(cfg *Config)
  ```
- **環境變數格式**：`MINIBOT_AGENTS_DEFAULTS_WORKSPACE`、`MINIBOT_AGENTS_DEFAULTS_MODEL` 等  
- **驗收**：
  - 無 config.json 時使用預設值
  - config.json 存在時正確載入
  - 設定環境變數後能正確覆寫

---

### T2-4：實作 Workspace 初始化

- **狀態**：⬜ 未開始  
- **檔案**：`cmd/appname/cmd_onboard.go`  
- **說明**：`app onboard` 指令執行以下流程：
  1. 建立 `~/.minibot/` 目錄
  2. 若 `config.json` 不存在，複製 `config.example.json` 範本
  3. 建立 workspace 目錄結構：
     ```
     ~/.minibot/workspace/
     ├── sessions/
     ├── memory/
     ├── AGENT.md
     ├── IDENTITY.md
     ├── SOUL.md
     └── USER.md
     ```
  4. 從內嵌（`embed.FS`）或程式碼中複製模板文件至 workspace
- **驗收**：`app onboard` 執行後 workspace 目錄結構完整

---

### T2-5：實作 ModelConfig 查詢工具

- **狀態**：⬜ 未開始  
- **檔案**：`pkg/config/config.go`  
- **說明**：提供函式依 `model_name` 查詢對應的模型設定  
- **函式簽名**：
  ```go
  func (c *Config) FindModel(modelName string) (*ModelConfig, error)
  ```
- **驗收**：能正確返回或回報 "model not found" 錯誤

---

## Phase 2 完成檢查

- [x] T2-1：Config struct 定義完整
- [x] T2-2：預設值常數定義
- [x] T2-3：三層優先序設定載入正常
- [x] T2-4：`app onboard` 正確初始化 workspace
- [x] T2-5：`FindModel` 函式正常查詢
