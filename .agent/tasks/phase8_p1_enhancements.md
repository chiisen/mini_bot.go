# Phase 8 — P1 增強功能

> **前置依賴**：Phase 7（MVP 核心完成）  
> **說明**：以下為 MVP_SPEC.md 中標記為 P1（建議）的增強功能，在核心 MVP 驗收通過後實作

---

## 任務清單

### T8-1：網頁搜尋工具（DuckDuckGo）

- **狀態**：✅ 已完成  
- **檔案**：`pkg/tools/web.go`  
- **說明**：整合 DuckDuckGo Instant Answer API 作為網頁搜尋工具（**免費、無需 API Key**）

  **工具定義**：
  ```json
  {
    "name": "web_search",
    "description": "Search the web using DuckDuckGo. Use this to find current information.",
    "parameters": {
      "type": "object",
      "properties": {
        "query": {
          "type": "string",
          "description": "The search query"
        }
      },
      "required": ["query"]
    }
  }
  ```

  **API 端點**：
  ```
  GET https://api.duckduckgo.com/?q={query}&format=json&no_html=1&skip_disambig=1
  ```

- **注意**：DuckDuckGo Instant Answer API 限制：
  - 主要返回精選摘要（AbstractText），不是完整搜尋結果列表
  - 若無摘要，可考慮解析 `RelatedTopics` 列表作為補充

- **驗收**：`web_search` 工具能返回有意義的搜尋結果給 LLM

---

### T8-2：互動式 Onboard 引導

- **狀態**：✅ 已完成  
- **檔案**：`cmd/appname/cmd_onboard.go`  
- **說明**：強化 `app onboard` 指令，加入互動式引導流程

  **引導步驟**：
  1. 詢問使用者選擇模型供應商（顯示選項列表）
  2. 輸入對應的 API Key
  3. 詢問是否啟用 Telegram（若是，引導輸入 bot token 與 allow_from）
  4. 詢問偏好語言（中文/English）
  5. 自動產生 `config.json` 並初始化 workspace
  6. 顯示成功摘要

- **驗收**：`app onboard` 能引導新用戶完成完整設定

---

### T8-3：狀態檢視指令（status）

- **狀態**：✅ 已完成  
- **檔案**：`cmd/appname/main.go` + `cmd_status.go`（新增）  
- **說明**：實作 `app status` 指令，顯示目前系統狀態

  **輸出內容**：
  ```
  ✅ Config: ~/.minibot/config.json
  ✅ Workspace: ~/.minibot/workspace
  ✅ Model: openai/gpt-4 (via gpt4)
  ✅ Tools: read_file, write_file, list_dir, edit_file, append_file, exec, web_search
  ✅ Telegram: enabled (token: 12345...ABC)
  📁 Sessions: 3 saved sessions
  ```

- **驗收**：`app status` 輸出清晰易讀的狀態資訊

---

### T8-4：Docker 部署支援

- **狀態**：✅ 已完成  
- **說明**：建立 Docker 相關文件，支援容器化部署

  **T8-4a：Dockerfile**（多階段建置）：
  ```dockerfile
  # 建置階段
  FROM golang:1.21-alpine AS builder
  WORKDIR /app
  COPY go.mod go.sum ./
  RUN go mod download
  COPY . .
  RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o minibot ./cmd/appname/

  # 執行階段
  FROM alpine:latest
  RUN apk --no-cache add ca-certificates
  WORKDIR /root/
  COPY --from=builder /app/minibot .
  CMD ["./minibot", "gateway"]
  ```

  **T8-4b：docker-compose.yml**：
  ```yaml
  version: '3.8'
  services:
    minibot:
      build: .
      restart: unless-stopped
      volumes:
        - ~/.minibot:/root/.minibot
      environment:
        - MINIBOT_AGENTS_DEFAULTS_WORKSPACE=/root/.minibot/workspace
  ```

- **驗收**：
  - `docker build .` 成功
  - `docker run ./minibot version` 能正常輸出

---

### T8-5：多平台交叉編譯

- **狀態**：✅ 已完成  
- **說明**：在 Makefile 加入多平台編譯 target

  **目標平台**：
  ```makefile
  release:
      GOOS=linux   GOARCH=amd64   go build -ldflags="-s -w" -o dist/minibot-linux-amd64   ./cmd/appname/
      GOOS=linux   GOARCH=arm64   go build -ldflags="-s -w" -o dist/minibot-linux-arm64   ./cmd/appname/
      GOOS=linux   GOARCH=riscv64 go build -ldflags="-s -w" -o dist/minibot-linux-riscv64 ./cmd/appname/
      GOOS=darwin  GOARCH=arm64   go build -ldflags="-s -w" -o dist/minibot-darwin-arm64  ./cmd/appname/
      GOOS=windows GOARCH=amd64   go build -ldflags="-s -w" -o dist/minibot-windows-amd64.exe ./cmd/appname/
  ```

- **驗收**：`make release` 成功產生所有平台的 binary，且大小 < 15MB

---

## Phase 8 完成檢查

- [x] T8-1：`web_search` 工具能返回 DuckDuckGo 搜尋結果
- [x] T8-2：互動式 onboard 引導流程完整
- [x] T8-3：`app status` 正確顯示系統狀態
- [x] T8-4：Docker 建置成功
- [x] T8-5：所有平台的 binary 均能成功編譯且 < 15MB
