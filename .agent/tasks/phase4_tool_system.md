# Phase 4 — 工具系統

> **前置依賴**：Phase 1、Phase 3（型別定義）  
> **驗收標準**：Agent 能透過工具系統讀寫 workspace 內的檔案、執行指令，且無法逃逸 workspace 沙箱

---

## 任務清單

### T4-1：定義 Tool 介面

- **狀態**：⬜ 未開始  
- **檔案**：`pkg/tools/base.go`  
- **說明**：定義所有工具共用的介面

  ```go
  type Tool interface {
      Name() string
      Description() string
      Parameters() map[string]any  // JSON Schema 格式
      Execute(ctx context.Context, args map[string]any) *ToolResult
  }
  ```

- **驗收**：介面定義可被實作使用

---

### T4-2：實作 ToolRegistry（工具註冊表）

- **狀態**：⬜ 未開始  
- **檔案**：`pkg/tools/registry.go`  
- **說明**：集中管理工具的註冊與執行請求的派發

  ```go
  type ToolRegistry struct {
      tools map[string]Tool
  }

  func NewRegistry() *ToolRegistry
  func (r *ToolRegistry) Register(tool Tool)
  func (r *ToolRegistry) Execute(ctx context.Context, name string, args map[string]any) *ToolResult
  func (r *ToolRegistry) Definitions() []providers.ToolDefinition  // 產生給 LLM 的工具定義列表
  ```

- **驗收**：能正確註冊工具、依名稱執行、產生 ToolDefinition 列表

---

### T4-3：實作安全沙箱路徑檢查器

- **狀態**：⬜ 未開始  
- **檔案**：`pkg/tools/sandbox.go`（或整合在 base.go）  
- **說明**：提供路徑安全驗證邏輯，供所有檔案系統工具共用

  ```go
  type Sandbox struct {
      Workspace string
  }

  // CheckPath 驗證路徑是否在 workspace 內
  // 返回絕對路徑，若不合法則返回錯誤
  func (s *Sandbox) CheckPath(inputPath string) (string, error)
  ```

- **邏輯**（參考架構文件）：
  1. 將輸入路徑轉為絕對路徑（`filepath.Abs`）
  2. 確認絕對路徑以 workspace 路徑為前綴（`strings.HasPrefix`）
  3. 拒絕包含 `..` 的跳脫嘗試（`filepath.Clean`）

- **驗收**：
  - workspace 內的合法路徑通過檢查
  - `../` 跳脫嘗試被拒絕並回傳錯誤

---

### T4-4：實作檔案系統工具

- **狀態**：⬜ 未開始  
- **檔案**：`pkg/tools/filesystem.go`  
- **說明**：實作 5 個檔案系統工具，全部受 Sandbox 保護

  | 工具名 | 功能 | 參數 |
  |--------|------|------|
  | `read_file` | 讀取檔案全文 | `path: string` |
  | `write_file` | 寫入（覆蓋）檔案 | `path: string, content: string` |
  | `list_dir` | 列出目錄內容 | `path: string` |
  | `edit_file` | 依行號範圍替換內容 | `path: string, start_line: int, end_line: int, new_content: string` |
  | `append_file` | 追加內容至檔案結尾 | `path: string, content: string` |

- **各工具 JSON Schema Parameters 範例**（`read_file`）：
  ```json
  {
    "type": "object",
    "properties": {
      "path": {
        "type": "string",
        "description": "檔案路徑（相對於 workspace）"
      }
    },
    "required": ["path"]
  }
  ```

- **驗收**：
  - 每個工具能正常執行
  - workspace 沙箱保護有效（外部路徑被拒）
  - `ToolResult.IsError` 在失敗時為 true，`ForLLM` 包含可讀的錯誤訊息

---

### T4-5：實作 Shell 執行工具（exec）

- **狀態**：⬜ 未開始  
- **檔案**：`pkg/tools/shell.go`  
- **說明**：實作 `exec` 工具，在受限環境下執行 shell 指令

- **參數**：
  ```json
  {
    "type": "object",
    "properties": {
      "command": { "type": "string", "description": "要執行的 shell 指令" },
      "timeout": { "type": "integer", "description": "逾時秒數，預設 30", "default": 30 }
    },
    "required": ["command"]
  }
  ```

- **安全限制**：
  - 工作目錄鎖定在 workspace 內
  - 封鎖危險指令清單（正則比對）：
    ```
    rm -rf / | del /f | rmdir /s
    format | mkfs | diskpart
    dd if=
    shutdown | reboot | poweroff
    :(){ :|:& };:
    ```
  - 指令逾時（使用 `context.WithTimeout`）

- **驗收**：
  - 合法指令（如 `ls`, `pwd`, `cat file.txt`）能正常執行
  - 危險指令被拒絕並回傳錯誤訊息

---

## Phase 4 完成檢查

- [x] T4-1：Tool 介面定義
- [x] T4-2：ToolRegistry 能正確路由工具呼叫
- [x] T4-3：Sandbox 路徑檢查正確封鎖跳脫嘗試
- [x] T4-4：5 個檔案系統工具正常且受沙箱保護
- [x] T4-5：exec 工具正常且封鎖危險指令
