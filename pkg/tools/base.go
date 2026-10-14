package tools

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/chiisen/mini_bot/pkg/i18n"
)

// ToolResult 是工具執行結果。
type ToolResult struct {
	ForLLM  string // 給模型看的內容
	IsError bool

	// Notice 是給「使用者」看的提示，非空時由 agent 迴圈輸出。
	//
	// 用於讓使用者看得見那些只影響未來行為、不會出現在回覆裡的副作用：寫入長期記憶
	// 或改寫會被讀進系統提示詞的檔案（見 personaPaths）。沒有這個機制時，這類寫入
	// 只會回饋給模型，使用者完全不會察覺。
	Notice string
}

// personaPaths 是會被讀進系統提示詞的檔案（相對於 workspace）。
//
// 改寫它們等於改變助理的人格、行為規則或長期記憶，而且效果會持續到後續每一次
// 對話，因此寫入時要讓使用者看得見。
var personaPaths = map[string]bool{
	"IDENTITY.md":      true,
	"AGENT.md":         true,
	"SOUL.md":          true,
	"USER.md":          true,
	"memory/MEMORY.md": true,
}

// personaFileNotice 在寫入目標是人格／記憶檔時回傳要顯示給使用者的提示。
//
// 相對路徑、絕對路徑與含 symlink 的寫法都要能辨識，因此交由沙盒正規化成
// workspace 內的相對路徑（CheckPath 對尚未存在的路徑也能處理）。
func personaFileNotice(sandbox *Sandbox, path string) string {
	rel := filepath.ToSlash(filepath.Clean(path))

	if sandbox != nil {
		if resolved, err := sandbox.CheckPath(path); err == nil {
			if r, err := filepath.Rel(sandbox.Workspace, resolved); err == nil {
				rel = filepath.ToSlash(filepath.Clean(r))
			}
		}
	}

	if !personaPaths[rel] {
		return ""
	}

	return i18n.GetInstance().T("notices.persona_file_written", rel)
}

// Tool represents the interface for a callable tool for the Agent
type Tool interface {
	Name() string
	Description() string
	Parameters() map[string]any // JSON schema format map
	Execute(ctx context.Context, args map[string]any) *ToolResult
}

// PreviewForNotice 產生給使用者看的內容預覽（以 rune 截斷，不切斷中文字）。
func PreviewForNotice(content string, max int) string {
	runes := []rune(strings.TrimSpace(content))
	if len(runes) <= max {
		return string(runes)
	}
	return string(runes[:max]) + "…"
}
