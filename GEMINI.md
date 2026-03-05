# mini_bot.go 專案規則

> 本檔案為 AI Agent 的專案執行指南，定義開發規範與重要參考資訊。

---

## 📋 專案概述

mini_bot.go 是一個極致輕量的本地端 AI 助理，採用 Go 語言開發，支援：
- 多 LLM 供應商（OpenAI、DeepSeek、MiniMax、Ollama 等）
- Telegram 機器人整合
- 本地工具呼叫（檔案操作、命令執行、網頁搜尋）
- 多語系支援（英文、繁體中文）

---

## 📁 重要檔案位置

| 檔案 | 用途 |
|------|------|
| `README.md` | 使用說明 |
| `docs/MVP_SPEC.md` | 功能規格與驗收標準 |
| `docs/mvp_architecture.md` | 架構設計與資料流 |
| `.agent/tasks/TASKS.md` | 任務總覽與分期 |
| `config/config.example.json` | 設定檔範本 |
| `pkg/tools/shell.go` | Shell 工具實作 |
| `lang/` | 多語系翻譯檔目錄 |

---

## 🛠️ 開發命令

```bash
# 編譯
go build -o app ./cmd/appname/

# 執行單次對話
./app agent -m "Hello"

# 互動模式
./app agent

# 啟動 Telegram Gateway
./app gateway

# 系統狀態
./app status

# 初始化設定
./app onboard
```

---

## 🌐 多語系設定

- **環境變數**：`MINIBOT_LANGUAGE=zh-tw` 或 `MINIBOT_LANGUAGE=en`
- **Config 設定**：在 `config.json` 中加入 `"language": "zh-tw"`
- **優先順序**：環境變數 > Config 設定 > 預設 (en)

詳見 `README.md` 多語系章節。

---

## 📝 程式碼規範

### Go 語言
- 遵循標準 Go 程式碼風格
- 使用 `gofmt` 格式化
- 全域變數使用註解說明

### 註解要求
- 公開 API 必須有繁體中文註解
- 重要函式說明用途、參數、回傳值

### 提交規範
- 使用中文撰寫 commit message
- 格式：`<type>: <subject>`
- Type：feat、fix、docs、refactor、test、chore

---

## ✅ 驗收標準

| 項目 | 目標 |
|------|------|
| 執行檔大小 | < 15 MB |
| 啟動速度 | < 1 秒 |
| 記憶體使用 | < 10 MB |
| 單元測試 | 全部通過 |

---

## 🔧 技術栈

- **語言**：Go 1.21+
- **依賴**：標準函式庫為主，極少第三方依賴
- **測試**：Go testing package

---

## 📌 當前狀態

- MVP 開發：✅ 已完成
- 單元測試：✅ 通過 (28 tests)
- 多語系支援：✅ 已實作

---

*最後更新：2026-03-05*
