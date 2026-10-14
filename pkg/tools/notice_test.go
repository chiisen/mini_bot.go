package tools

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chiisen/mini_bot/pkg/i18n"
)

// loadTestTranslations 載入專案的 lang 目錄並固定為英文，讓斷言能比對實際顯示給
// 使用者的文字（未載入時 T() 會回傳原始 key）。
func loadTestTranslations(t *testing.T) {
	t.Helper()

	inst := i18n.GetInstance()
	if err := inst.LoadFromDir(filepath.Join("..", "..", "lang")); err != nil {
		t.Fatalf("載入 lang 目錄失敗: %v", err)
	}
	inst.SetLang("en")

	if got := inst.T("notices.persona_file_written", "x"); got == "notices.persona_file_written" {
		t.Fatalf("語言檔未載入，T() 回傳原始 key")
	}
}

// TestPersonaFileNotice 驗證寫入人格／記憶檔時會回報給使用者。
//
// 這些檔案會被讀進系統提示詞，改寫等於改變助理的行為或記憶，而且影響後續每一次
// 對話；沒有這個提示，使用者完全不會察覺。
func TestPersonaFileNotice(t *testing.T) {
	sandbox, _ := NewSandbox(t.TempDir())

	cases := []struct {
		name string
		path string
		want bool
	}{
		{"人格檔", "IDENTITY.md", true},
		{"行為指南", "AGENT.md", true},
		{"個性檔案", "SOUL.md", true},
		{"使用者偏好", "USER.md", true},
		{"記憶檔", "memory/MEMORY.md", true},
		{"相對路徑寫法", "./SOUL.md", true},
		{"一般檔案", "notes.txt", false},
		{"其他目錄下的同名檔", "docs/IDENTITY.md", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := personaFileNotice(sandbox, tc.path)

			if tc.want && got == "" {
				t.Errorf("預期 %s 會產生提示", tc.path)
			}
			if !tc.want && got != "" {
				t.Errorf("預期 %s 不會產生提示，卻得到 %q", tc.path, got)
			}
		})
	}
}

// TestPersonaFileNotice_AbsolutePath 驗證以絕對路徑寫入時也能辨識。
func TestPersonaFileNotice_AbsolutePath(t *testing.T) {
	workspace := t.TempDir()
	sandbox, _ := NewSandbox(workspace)

	got := personaFileNotice(sandbox, workspace+"/AGENT.md")
	if got == "" {
		t.Error("以絕對路徑寫入 AGENT.md 時也應產生提示")
	}
}

// TestWriteFileTool_NotifiesOnPersonaFile 驗證工具的 Notice 有被帶出來。
func TestWriteFileTool_NotifiesOnPersonaFile(t *testing.T) {
	loadTestTranslations(t)

	sandbox, _ := NewSandbox(t.TempDir())
	tool := WriteFileTool{Sandbox: sandbox}

	persona := tool.Execute(context.Background(), map[string]any{"path": "AGENT.md", "content": "new rules"})
	if persona.Notice == "" {
		t.Fatal("寫入 AGENT.md 時應帶出使用者提示")
	}
	if !strings.Contains(persona.Notice, "AGENT.md") {
		t.Errorf("提示應指出是哪個檔案：%q", persona.Notice)
	}

	normal := tool.Execute(context.Background(), map[string]any{"path": "notes.txt", "content": "hi"})
	if normal.Notice != "" {
		t.Errorf("一般檔案不應產生提示：%q", normal.Notice)
	}
}

// TestAppendAndEditNotifyOnMemoryFile 驗證 append 與 edit 也涵蓋記憶檔。
func TestAppendAndEditNotifyOnMemoryFile(t *testing.T) {
	sandbox, _ := NewSandbox(t.TempDir())

	appendRes := (&AppendFileTool{Sandbox: sandbox}).Execute(context.Background(), map[string]any{
		"path":    "memory/MEMORY.md",
		"content": "note",
	})
	if appendRes.Notice == "" {
		t.Error("append 寫入記憶檔時應帶出提示")
	}

	// 注意：工具的數值參數一律來自 JSON，型別是 float64
	editRes := (&EditFileTool{Sandbox: sandbox}).Execute(context.Background(), map[string]any{
		"path":        "memory/MEMORY.md",
		"start_line":  float64(1),
		"end_line":    float64(1),
		"new_content": "changed",
	})
	if editRes.Notice == "" {
		t.Error("edit 寫入記憶檔時應帶出提示")
	}
}

// TestPreviewForNotice 驗證預覽以 rune 截斷，不切斷中文字。
func TestPreviewForNotice(t *testing.T) {
	short := PreviewForNotice("  短內容  ", 60)
	if short != "短內容" {
		t.Errorf("應去除前後空白：%q", short)
	}

	long := PreviewForNotice(strings.Repeat("中", 100), 60)
	if strings.Count(long, "中") != 60 || !strings.HasSuffix(long, "…") {
		t.Errorf("應截斷到 60 個字元並加上省略號：%q", long)
	}
}

// TestWriteFileTool_CreatesParentDirs 驗證寫入新目錄時會自動建立上層目錄。
//
// onboard 只會建立 workspace、sessions 與 memory；自訂 workspace 或要寫到新目錄時，
// 不建立就會以 no such file or directory 失敗（與 #15 修掉的是同一類問題）。
func TestWriteFileTool_CreatesParentDirs(t *testing.T) {
	sandbox, _ := NewSandbox(t.TempDir())
	tool := WriteFileTool{Sandbox: sandbox}

	for _, path := range []string{"nested/dir/note.md", "memory/MEMORY.md"} {
		result := tool.Execute(context.Background(), map[string]any{"path": path, "content": "hi"})
		if result.IsError {
			t.Errorf("寫入 %s 失敗：%s", path, result.ForLLM)
		}
	}
}
