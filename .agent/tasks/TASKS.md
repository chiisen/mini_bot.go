# 📋 mini_bot.go — 專案任務總覽

> 依據 [MVP 規格書](../../docs/MVP_SPEC.md) 與 [架構設計](../../docs/mvp_architecture.md) 規劃。  
> 目標：超輕量級個人 AI 助手，單一 binary < 15MB、RAM < 10MB、冷啟動 < 1 秒。

---

## 🕒 目前狀態快照（2026-10-13 實測）

> 本節是**日期化的快照**，不是長期保證。重新驗證請執行：
>
> ```bash
> go build ./... && go vet ./... && go test -race ./...
> ```
>
> CI（`.github/workflows/ci.yml`）於每次 push 與 PR 執行相同指令；權威狀態以 CI 結果與 GitHub Issues 為準，本檔不重複維護逐項狀態。

**測試**：`go test ./...` 0 個失敗（60 個測試函式）。同日審查開始時為 12 個失敗。
**CI**：ubuntu-latest 與 macos-latest 各執行 gofmt / build / vet / `test -race`，另在 Linux 上交叉編譯 windows/amd64。
**追蹤**：[GitHub Issues](https://github.com/chiisen/mini_bot.go/issues) ＋ [Roadmap 看板](https://github.com/users/chiisen/projects/14)。

### 已修復並合併（2026-10-13）

| Issue | 內容 | PR |
|-------|------|----|
| #1 | 沙盒 symlink 解析不一致（macOS 下既有檔案操作全數被拒）；一併修掉可穿過 symlink **寫出沙盒外**的逃逸漏洞 | #12 |
| #2 | `MemoryManager.Append` 誤用 `SanitizeInput`：寫入 MEMORY.md 的記憶被包成 code fence，字元也被轉義 | #11 |
| #5 | 建立 CI 與 `AgentInstance.Run()` 測試（原本是 `t.Skip`），並清理未格式化的檔案 | #14 |
| #6 | `pkg/i18n` 並行載入的 data race：gateway 模式下可能以 `fatal error: concurrent map writes` 終止程式 | #13 |

### 待辦：不需決策

| Issue | 優先 | 內容 | 估時 |
|-------|------|------|------|
| #3 | P1 | `Compress` 無條件保留最後 10 則，會切斷 assistant/tool 訊息配對，長對話使 LLM API 回 400 | ~3h |
| #9 | P2 | 剩下 provider 的 429/5xx 重試、可配置逾時、streaming（權限與 Dockerfile 已完成，PR #17 待合併） | ~2-3h |
| #15 | P2 | `session.Save` 在 sessions 目錄不存在時失敗（PR #16 待合併） | — |
| #10 | P3 | 校正 README、GEMINI.md 與本檔剩餘的敘述落差（最後做；本檔已先部分更新） | ~2h |

### 待辦：需決策後才能動

| Issue | 優先 | 待決策事項 | 估時 |
|-------|------|-----------|------|
| #4 | P1 | `exec` 不受沙盒約束，且管線指令被靜默改寫：(a) 文件誠實標註／(b) 掃描指令中的絕對路徑／(c) 改為需明確 opt-in | ~4h |
| #7 | P2 | 未接線功能三取捨：長期記憶（接上或移除）、`RateLimiter`（接上或刪除）、`restrictToWorkspace`（實作或移除） | ~4h |
| #8 | P2 | `SanitizeInput` 是否允許改寫使用者輸入（現行會破壞貼上的程式碼與 JSON，且注入偵測與取代大小寫不一致） | ~3h |

### 已發現但尚未開票

- `pkg/i18n` 的惰性初始化只看當前工作目錄，`pkg/config.Load` 則以執行檔所在目錄為優先。兩條尋找路徑不一致，導致 `docker run -w <其他目錄> ... version` 會顯示原始 i18n key；統一後 Dockerfile 可只保留一份語言檔。
- `.coderabbit.yaml` 已設定，但 PR 上沒有出現 CodeRabbit 的評論，可能未安裝 App；PR 目前僅由 CI 把關。
- `docs/TDD.md` 要求使用 `testify`，與 `GEMINI.md`／`go.mod` 的「零第三方依賴」互相矛盾；目前所有測試都使用標準庫。

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
