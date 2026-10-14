# 📋 mini_bot.go — 專案任務總覽

> 依據 [MVP 規格書](../../docs/MVP_SPEC.md) 與 [架構設計](../../docs/mvp_architecture.md) 規劃。  
> 目標：超輕量級個人 AI 助手，單一 binary < 15MB、RAM < 10MB、冷啟動 < 1 秒。

---

## 🕒 目前狀態快照（2026-10-14 實測）

> 本節是**日期化的快照**，不是長期保證。重新驗證請執行：
>
> ```bash
> go build ./... && go vet ./... && go test -race ./...
> ```
>
> CI（`.github/workflows/ci.yml`）於每次 push 與 PR 執行相同指令；權威狀態以 CI 結果與 GitHub Issues 為準，本檔不重複維護逐項狀態。

**測試**：`go test ./...` 0 個失敗。審查開始（2026-10-13）時為 12 個失敗。
**CI**：ubuntu-latest 與 macos-latest 各執行 gofmt / build / vet / `test -race`，另在 Linux 上交叉編譯 windows/amd64。
**開發流程**：2026-10-14 起改為**直接在 `main` 開發**（不開 PR）；每個 commit 推送時由 CI 把關。
**追蹤**：[GitHub Issues](https://github.com/chiisen/mini_bot.go/issues) ＋ [Roadmap 看板](https://github.com/users/chiisen/projects/14)。

### 已修復（2026-10-13 ~ 10-14）

| Issue | 內容 |
|-------|------|
| #23 | `app onboard` 產生的 config 範本與現行 schema 不符，導致照著 README 的第一步做就無法啟動（第一道流程就壞） |
| #1 | 沙盒 symlink 解析不一致（macOS 下既有檔案操作全數被拒）；一併修掉可穿過 symlink **寫出沙盒外**的逃逸漏洞 |
| #2 | `MemoryManager.Append` 誤用 `SanitizeInput`：記憶被包成 code fence，字元也被轉義 |
| #3 | `Compress` 切斷 assistant/tool 訊息配對，長對話會讓 LLM API 回 400 |
| #4 | `exec` 越界讀取不受阻（`cat /etc/passwd` 真的讀得到）、且管線指令被靜默改寫；改為拒絕並新增路徑圍堵 |
| #6 | `pkg/i18n` 並行載入的 data race：gateway 可能以 `concurrent map writes` 終止 |
| #7 | 長期記憶補上寫入端（`memory_write`）；刪除未接線的 `RateLimiter`、`restrictToWorkspace`、`memoryWindow` |
| #8 | `SanitizeInput` 不再改寫使用者內容（原本會弄壞程式碼與 JSON），改以界線為防護主體 |
| #9 | session 與 config 權限改 0600；LLM 請求加入重試與可配置逾時；Dockerfile 版本與容器內語言檔 |
| #5 | 建立 CI 與 `AgentInstance.Run()` 測試（原本是 `t.Skip`） |
| #15 | `session.Save` 在目錄不存在時失敗（自訂 workspace 的使用者每次對話都在結尾噴錯） |

### 待辦

| Issue | 優先 | 內容 |
|-------|------|------|
| #10 | P3 | 本檔與文件索引的持續校正（本節已更新，其他文件段落見該 issue） |
| #20 | P3 | `Compress` 改用 token 預算而非固定訊息數 |
| #21 | P3 | 評估 streaming 回應支援（需先決定範圍，會動到 `LLMProvider` 介面） |

### 已發現但尚未開票

- `pkg/i18n` 的惰性初始化只看當前工作目錄，`pkg/config.Load` 則以執行檔所在目錄為優先。兩條尋找路徑不一致，導致 `docker run -w <其他目錄> ... version` 會顯示原始 i18n key。
- `.coderabbit.yaml` 已設定，但 PR 上沒有出現 CodeRabbit 的評論，可能未安裝 App。
- 第二輪審查的候選：`pkg/bus` 的 backpressure、`pkg/channels/telegram.go` 的 update offset 未持久化、`app gateway` 在 Telegram 未啟用時的行為。

---

## 🗂️ 任務分期索引

> ⚠️ **歷史區塊**：下表與各 phase 檔案的狀態欄位撰寫於 2026-02，部分陳述已被 2026-10 的審查推翻（例如「28 個測試全部通過」——實際為 60 個測試，且審查開始時有 12 個失敗）。內容保留作為歷史紀錄，逐項校正見 issue #10。

| Phase | 名稱 | 說明 | 狀態 |
|-------|------|------|------|
| [Phase 1](phase1_project_foundation.md) | 專案基礎建設 | Go module、目錄結構、CLI 骨架、Makefile | ✅ 已完成 |
| [Phase 2](phase2_config_workspace.md) | 設定檔與 Workspace 系統 | config.json 載入、環境變數覆寫、workspace 初始化 | ✅ 已完成 |
| [Phase 3](phase3_llm_provider.md) | LLM Provider 抽象層 | LLMProvider 介面、資料結構、OpenAI 相容實作、Provider 工廠 | ✅ 已完成 |
| [Phase 4](phase4_tool_system.md) | 工具系統 | Tool 介面、ToolRegistry、檔案系統工具、shell 工具、安全沙箱 | ✅ 已完成 |
| [Phase 5](phase5_agent_core.md) | Agent 核心迴圈 | AgentInstance、AgentLoop、Context 建構器、Session 管理 | ✅ 已完成 |
| [Phase 6](phase6_message_bus.md) | 訊息匯流排與 CLI 整合 | MessageBus、CLI agent 指令、互動式模式 | ✅ 已完成 |
| [Phase 7](phase7_telegram.md) | Telegram 頻道整合 | Telegram Bot API、gateway 指令、allow_from 白名單 | ✅ 已完成 |
| [Phase 8](phase8_p1_enhancements.md) | P1 增強功能 | 網頁搜尋、onboard、status 指令、Docker、多平台編譯 | ✅ 已完成 |

---

## ✅ 驗收標準

| 項目 | 驗收條件 | 狀態 | 備註 |
|------|----------|------|------|
| 記憶體 | 靜置時 RSS < 10MB | ✅ | 測試結果：6.21 MB |
| 啟動速度 | 冷啟動 < 1 秒（x86_64） | ✅ | 測試結果：41 ms |
| 執行檔大小 | 單一 binary < 15MB（strip 後） | ✅ | 測試結果：8.1 MB |
| 基本對話 | `app agent -m "What is 2+2?"` 正確回應 | ✅ | 代碼完整 |
| 工具執行 | Agent 能自主使用工具讀寫檔案、執行指令 | ✅ | 代碼完整，單元測試通過 |
| 安全沙箱 | 工具無法存取 workspace 以外的路徑 | ✅ | 8 個單元測試通過（另見下方註記） |
| Telegram | 透過 Telegram Bot 能正常對話 | ✅ | gateway 正常運行 |
| 設定載入 | config.json 正確載入並支援環境變數覆寫 | ✅ | 已驗證 |
| Session | 對話歷史能持久化並在後續對話中載入 | ✅ | 代碼完整 |

> **測試日期**：2026-02-23，**測試環境**：Windows 11, Go 1.23.5
>
> **註記（2026-10-13）**：「安全沙箱」一列在當時成立，但沙盒路徑檢查在 workspace 含 symlink 時會誤判（已於 #1 修正），且 `exec` 工具不受沙盒約束（見 #4）。

---

## ✅ 已完成任務

- **Windows Shell 兼容性** — 已修復，自動選擇 `cmd /c` (Windows) 或 `sh -c` (Unix)
- **單元測試** — 28 個測試全部通過（2026-02 的紀錄；現況見上方快照）
- **所有 MVP 驗收標準** — 全部通過（同上，為 2026-02 的紀錄）

---

## 📌 狀態說明

- ⬜ 未開始
- 🔄 進行中
- ✅ 已完成
- ❌ 封鎖中（需解決依賴）
