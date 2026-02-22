# Phase 1 — 專案基礎建設

> **前置依賴**：無  
> **預計完成**：Phase 2 啟動前  
> **驗收標準**：`make build` 能成功編譯，`./app version` 能正常輸出版本資訊

---

## 任務清單

### T1-1：初始化 Go Module

- **狀態**：⬜ 未開始  
- **說明**：建立 `go.mod`，模組名稱定為 `github.com/chiisen/mini_bot`  
- **指令**：
  ```bash
  go mod init github.com/chiisen/mini_bot
  ```
- **驗收**：`go.mod` 存在且模組名稱正確

---

### T1-2：建立專案目錄結構

- **狀態**：⬜ 未開始  
- **說明**：依照 MVP_SPEC.md 規劃建立目錄骨架  
- **目標結構**：
  ```
  cmd/appname/
  pkg/agent/
  pkg/config/
  pkg/providers/
  pkg/providers/openai_compat/
  pkg/tools/
  pkg/channels/
  pkg/session/
  pkg/bus/
  pkg/logger/
  pkg/state/
  workspace/
  config/
  ```
- **驗收**：所有目錄存在（可為空目錄，需含 `.gitkeep`）

---

### T1-3：建立 CLI 入口（main.go）

- **狀態**：⬜ 未開始  
- **說明**：在 `cmd/appname/main.go` 實作 CLI 路由，使用 Go 標準庫 `flag` 或輕量級 CLI 函式庫（如 `cobra`）  
- **支援指令**（骨架，不含實際邏輯）：
  ```
  app onboard     → 呼叫 cmd_onboard.go 邏輯
  app agent       → 呼叫 cmd_agent.go 邏輯
  app gateway     → 呼叫 cmd_gateway.go 邏輯
  app version     → 輸出版本字串
  app status      → 輸出狀態（P1，先佔位）
  ```
- **注意**：保持 **零外部依賴** 為目標，若使用 cobra 需評估二進制大小影響  
- **驗收**：
  - `go build ./cmd/appname/` 成功
  - `./app version` 輸出版本資訊
  - 未知指令顯示 help 訊息

---

### T1-4：建立各子指令骨架

- **狀態**：⬜ 未開始  
- **說明**：建立以下骨架檔案，各自包含函式簽名但不含實作邏輯：
  - `cmd/appname/cmd_agent.go` — `RunAgent(args []string) error`
  - `cmd/appname/cmd_gateway.go` — `RunGateway(args []string) error`
  - `cmd/appname/cmd_onboard.go` — `RunOnboard(args []string) error`
- **驗收**：`go build ./...` 成功，無編譯錯誤

---

### T1-5：建立 Makefile

- **狀態**：⬜ 未開始  
- **說明**：建立 `Makefile`，包含以下 target：
  ```makefile
  build:    go build -ldflags="-s -w" -o app ./cmd/appname/
  run:      ./app $(ARGS)
  clean:    rm -f app
  test:     go test ./...
  lint:     go vet ./...
  size:     ls -lh app  # 確認 binary 大小
  ```
- **驗收**：`make build` 成功產生 `app` binary

---

### T1-6：建立 workspace 模板文件

- **狀態**：⬜ 未開始  
- **說明**：在 `workspace/` 目錄建立初始模板 Markdown 文件  

  | 檔案 | 用途 |
  |------|------|
  | `workspace/IDENTITY.md` | Agent 自我描述（名稱、能力、目標） |
  | `workspace/AGENT.md` | Agent 行為指引（何時使用工具、如何回應） |
  | `workspace/SOUL.md` | Agent 個性特質（友善、簡潔、誠實） |
  | `workspace/USER.md` | 使用者偏好（語言、時區、風格） |

- **驗收**：4 個 Markdown 文件存在，內容為合理的模板範例

---

### T1-7：建立 config.example.json

- **狀態**：⬜ 未開始  
- **說明**：在 `config/config.example.json` 建立設定範例  
- **最小結構**（參考 MVP_SPEC.md §5）：
  ```json
  {
    "agents": {
      "defaults": {
        "workspace": "~/.minibot/workspace",
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
        "enabled": false,
        "token": "YOUR_BOT_TOKEN",
        "allow_from": ["YOUR_USER_ID"]
      }
    }
  }
  ```
- **驗收**：檔案存在，JSON 格式合法（可用 `cat config/config.example.json | python -m json.tool` 驗證）

---

## Phase 1 完成檢查

- [x] T1-1：go.mod 存在
- [x] T1-2：所有目錄結構存在
- [x] T1-3：`./app version` 正常運作
- [x] T1-4：`go build ./...` 無錯誤
- [x] T1-5：`make build` 成功
- [x] T1-6：workspace 模板文件完整
- [x] T1-7：config.example.json 合法
