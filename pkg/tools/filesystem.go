package tools

// ============================================================================
// 檔案系統工具 (Filesystem Tools)
// ============================================================================
// 本檔案提供一系列檔案操作工具，供 Agent 用於處理檔案任務。
// 所有工具都受到沙盒 (Sandbox) 的保護，確保操作限制在工作區目錄內。
//
// 提供的工具列表：
//   1. ReadFileTool   (read_file)   : 讀取檔案內容
//   2. WriteFileTool  (write_file)  : 覆寫或建立檔案
//   3. AppendFileTool (append_file) : 追加內容到檔案
//   4. EditFileTool   (edit_file)   : 編輯特定行範圍的內容
//   5. ListDirTool    (list_dir)    : 列出目錄內容
//
// 安全機制：
//   - 所有路徑都會經過 Sandbox 檢查，防止目錄穿越攻擊
//   - 路徑必須符合安全字元規則 (只能包含字母、數字、底線、斜線等)
//   - 不允許使用 ".." 進行目錄回溯
// ============================================================================

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/chiisen/mini_bot/pkg/i18n"
)

// ============================================================================
// 安全驗證常數和函數
// ============================================================================

// safePathChars 定義允許在路徑中使用的字元
// 這是一個正則表達式，確保路徑只包含安全字元
var safePathChars = regexp.MustCompile(`^[a-zA-Z0-9_/\\.:\-]+$`)

// isValidPathChars 驗證路徑字串是否安全
//
// 安全規則：
//  1. 不能包含 ".." (防止目錄穿越)
//  2. 只能包含允許的字元 (字母、數字、底線、斜線、點、冒號、連字符)
//
// 參數：
//   - path: 要驗證的路徑字串
//
// 回傳：
//   - bool: 是否安全
func isValidPathChars(path string) bool {
	// 檢查是否包含目錄回溯
	if strings.Contains(path, "..") {
		return false
	}
	// 檢查是否只包含允許的字元
	return safePathChars.MatchString(path)
}

// ============================================================================
// ReadFileTool: 讀取檔案工具
// 功能：讀取指定檔案的完整內容
// 工具名稱：read_file
type ReadFileTool struct {
	Sandbox *Sandbox // 沙盒實例，用於路徑安全檢查
}

// Name 返回工具名稱
func (t *ReadFileTool) Name() string { return "read_file" }

// Description 返回工具描述 (多語系)
func (t *ReadFileTool) Description() string { return i18n.GetInstance().T("tools.read_file") }

// Parameters 返回工具參數定義 (JSON Schema 格式)
func (t *ReadFileTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": i18n.GetInstance().T("tool_params.path"),
			},
		},
		"required": []string{"path"}, // path 是必填參數
	}
}

// Execute 執行讀取檔案操作
//
// 執行步驟：
//  1. 驗證路徑安全性
//  2. 透過沙盒檢查轉換為絕對路徑
//  3. 讀取檔案內容
//  4. 為每行添加行號 (方便 AI 定位和替換)
func (t *ReadFileTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	// 取得參數
	path, _ := args["path"].(string)

	// 驗證路徑安全性
	if !isValidPathChars(path) {
		return &ToolResult{ForLLM: i18n.GetInstance().T("errors.path_invalid"), IsError: true}
	}

	// 透過沙盒取得安全路徑
	safePath, err := t.Sandbox.CheckPath(path)
	if err != nil {
		return &ToolResult{ForLLM: err.Error(), IsError: true}
	}

	// 讀取檔案
	content, err := os.ReadFile(safePath)
	if err != nil {
		return &ToolResult{ForLLM: fmt.Sprintf(i18n.GetInstance().T("errors.read_failed"), err), IsError: true}
	}

	// 為輸出添加行號
	// 這對於 AI 進行精確的替換操作非常重要
	lines := strings.Split(string(content), "\n")
	var numbered strings.Builder
	for i, line := range lines {
		// 處理最後一個空行的特殊情況
		if i == len(lines)-1 && line == "" && len(lines) > 1 {
			break
		}
		// 格式: "行號: 內容"
		numbered.WriteString(fmt.Sprintf("%d: %s\n", i+1, line))
	}

	return &ToolResult{ForLLM: numbered.String(), IsError: false}
}

// ============================================================================
// WriteFileTool: 寫入檔案工具
// ============================================================================
// 功能：覆寫或建立新檔案
// 工具名稱：write_file
// 注意：這會覆蓋現有檔案，使用時要小心
type WriteFileTool struct {
	Sandbox *Sandbox
}

func (t *WriteFileTool) Name() string        { return "write_file" }
func (t *WriteFileTool) Description() string { return i18n.GetInstance().T("tools.write_file") }
func (t *WriteFileTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": i18n.GetInstance().T("tool_params.path"),
			},
			"content": map[string]any{
				"type":        "string",
				"description": i18n.GetInstance().T("tool_params.content"),
			},
		},
		"required": []string{"path", "content"}, // path 和 content 都是必填
	}
}

// Execute 執行寫入檔案操作
func (t *WriteFileTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	path, _ := args["path"].(string)
	content, _ := args["content"].(string)

	// 驗證路徑安全性
	if !isValidPathChars(path) {
		return &ToolResult{ForLLM: i18n.GetInstance().T("errors.path_invalid"), IsError: true}
	}

	// 取得安全路徑
	safePath, err := t.Sandbox.CheckPath(path)
	if err != nil {
		return &ToolResult{ForLLM: err.Error(), IsError: true}
	}

	// 寫入檔案
	// 權限 0600: rw------- (只有所有者可讀寫)
	if err := os.WriteFile(safePath, []byte(content), 0600); err != nil {
		return &ToolResult{ForLLM: fmt.Sprintf(i18n.GetInstance().T("errors.write_failed"), err), IsError: true}
	}
	return &ToolResult{ForLLM: "File written successfully.", IsError: false}
}

// ============================================================================
// AppendFileTool: 追加檔案工具
// ============================================================================
// 功能：將內容追加到現有檔案的末尾
// 工具名稱：append_file
// 適用場景：日誌記錄、對話歷史追加等
type AppendFileTool struct {
	Sandbox *Sandbox
}

func (t *AppendFileTool) Name() string        { return "append_file" }
func (t *AppendFileTool) Description() string { return i18n.GetInstance().T("tools.append_file") }
func (t *AppendFileTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": i18n.GetInstance().T("tool_params.path"),
			},
			"content": map[string]any{
				"type":        "string",
				"description": i18n.GetInstance().T("tool_params.content"),
			},
		},
		"required": []string{"path", "content"},
	}
}

// Execute 執行追加檔案操作
func (t *AppendFileTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	path, _ := args["path"].(string)
	content, _ := args["content"].(string)

	// 驗證路徑安全性
	if !isValidPathChars(path) {
		return &ToolResult{ForLLM: i18n.GetInstance().T("errors.path_invalid"), IsError: true}
	}

	// 取得安全路徑
	safePath, err := t.Sandbox.CheckPath(path)
	if err != nil {
		return &ToolResult{ForLLM: err.Error(), IsError: true}
	}

	// 以追加模式開啟檔案
	// os.O_APPEND: 追加模式
	// os.O_CREATE: 如果檔案不存在則建立
	// os.O_WRONLY: 僅寫入
	f, err := os.OpenFile(safePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return &ToolResult{ForLLM: fmt.Sprintf("Failed to open file: %v", err), IsError: true}
	}
	defer f.Close() // 確保函數返回時關閉檔案

	// 寫入內容
	if _, err := f.WriteString(content); err != nil {
		return &ToolResult{ForLLM: fmt.Sprintf("Failed to append: %v", err), IsError: true}
	}
	return &ToolResult{ForLLM: "Content appended successfully.", IsError: false}
}

// ============================================================================
// EditFileTool: 編輯檔案工具
// ============================================================================
// 功能：替換檔案中指定行範圍的內容
// 工具名稱：edit_file
// 參數：
//   - path:        檔案路徑
//   - start_line:  起始行號 (1-indexed，包含)
//   - end_line:    結束行號 (1-indexed，包含)
//   - new_content: 要替換的新內容
type EditFileTool struct {
	Sandbox *Sandbox
}

func (t *EditFileTool) Name() string { return "edit_file" }
func (t *EditFileTool) Description() string {
	return i18n.GetInstance().T("tools.edit_file")
}
func (t *EditFileTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": i18n.GetInstance().T("tool_params.path"),
			},
			"start_line": map[string]any{
				"type":        "integer",
				"description": i18n.GetInstance().T("tool_params.start_line"),
			},
			"end_line": map[string]any{
				"type":        "integer",
				"description": i18n.GetInstance().T("tool_params.end_line"),
			},
			"new_content": map[string]any{
				"type":        "string",
				"description": i18n.GetInstance().T("tool_params.new_content"),
			},
		},
		"required": []string{"path", "start_line", "end_line", "new_content"},
	}
}

// Execute 執行編輯檔案操作
//
// 編輯策略：
//  1. 讀取原檔案
//  2. 根據行號切割為三部分：開頭、編輯區、結尾
//  3. 用新內容替換編輯區
//  4. 合併並寫回檔案
func (t *EditFileTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	path, _ := args["path"].(string)
	startLineFl, _ := args["start_line"].(float64) // JSON number 在 Go 中會被解析為 float64
	endLineFl, _ := args["end_line"].(float64)
	newContent, _ := args["new_content"].(string)

	// 驗證路徑安全性
	if !isValidPathChars(path) {
		return &ToolResult{ForLLM: i18n.GetInstance().T("errors.path_invalid"), IsError: true}
	}

	// 轉換為整數
	startLine := int(startLineFl)
	endLine := int(endLineFl)

	// 取得安全路徑
	safePath, err := t.Sandbox.CheckPath(path)
	if err != nil {
		return &ToolResult{ForLLM: err.Error(), IsError: true}
	}

	// 讀取原檔案
	bytesData, err := os.ReadFile(safePath)
	if err != nil {
		return &ToolResult{ForLLM: fmt.Sprintf("Failed to read file: %v", err), IsError: true}
	}

	// 按行分割
	lines := strings.Split(string(bytesData), "\n")

	// 驗證行號範圍
	if startLine < 1 || startLine > len(lines) {
		return &ToolResult{ForLLM: "start_line out of bounds", IsError: true}
	}
	if endLine < startLine || endLine > len(lines) {
		return &ToolResult{ForLLM: "end_line out of bounds", IsError: true}
	}

	// 執行替換
	// 注意：行號是 1-indexed，但切片是 0-indexed
	var newLines []string
	newLines = append(newLines, lines[:startLine-1]...) // 開頭部分 (不包括 start_line)
	if newContent != "" {
		newLines = append(newLines, newContent) // 新內容
	}
	newLines = append(newLines, lines[endLine:]...) // 結尾部分 (從 end_line+1 開始)

	// 寫回檔案
	if err := os.WriteFile(safePath, []byte(strings.Join(newLines, "\n")), 0600); err != nil {
		return &ToolResult{ForLLM: fmt.Sprintf("Failed to write changes: %v", err), IsError: true}
	}

	return &ToolResult{ForLLM: "File edited successfully.", IsError: false}
}

// ============================================================================
// ListDirTool: 列出目錄工具
// ============================================================================
// 功能：列出指定目錄的內容
// 工具名稱：list_dir
// 輸出格式：每行顯示 [類型] 名稱 (大小)
//   - [D] 表示目錄 (Directory)
//   - [F] 表示檔案 (File)
type ListDirTool struct {
	Sandbox *Sandbox
}

func (t *ListDirTool) Name() string        { return "list_dir" }
func (t *ListDirTool) Description() string { return i18n.GetInstance().T("tools.list_dir") }
func (t *ListDirTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": i18n.GetInstance().T("tool_params.directory_path"),
			},
		},
		"required": []string{"path"},
	}
}

// Execute 執行列出目錄操作
func (t *ListDirTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	path, _ := args["path"].(string)

	// 驗證路徑安全性
	if !isValidPathChars(path) {
		return &ToolResult{ForLLM: i18n.GetInstance().T("errors.path_invalid"), IsError: true}
	}

	// 取得安全路徑
	safePath, err := t.Sandbox.CheckPath(path)
	if err != nil {
		return &ToolResult{ForLLM: err.Error(), IsError: true}
	}

	// 讀取目錄內容
	entries, err := os.ReadDir(safePath)
	if err != nil {
		return &ToolResult{ForLLM: fmt.Sprintf("Failed to read dir: %v", err), IsError: true}
	}

	// 構建輸出
	var sb strings.Builder
	for _, entry := range entries {
		info, _ := entry.Info()
		t := "F" // 預設為檔案
		if entry.IsDir() {
			t = "D" // 目錄
		}
		// 格式: [類型] 名稱 (大小: xxx bytes)
		sb.WriteString(fmt.Sprintf("[%s] %s (Size: %d bytes)\n", t, entry.Name(), info.Size()))
	}

	return &ToolResult{ForLLM: sb.String(), IsError: false}
}
