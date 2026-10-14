package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/chiisen/mini_bot/pkg/i18n"
	"github.com/chiisen/mini_bot/pkg/tools"
)

// MaxMessageLength 是單筆記憶的長度上限
const MaxMessageLength = 10000

// MemoryManager 負責讀寫長期記憶檔（workspace/memory/MEMORY.md）。
//
// 檔案的讀取端是 ContextBuilder（見 context.go），它會把內容放進系統提示詞的
// [MEMORY] 區段；寫入端是 MemoryWriteTool。
type MemoryManager struct {
	MemoryPath string
}

func NewMemoryManager(memoryPath string) *MemoryManager {
	return &MemoryManager{
		MemoryPath: memoryPath,
	}
}

func (m *MemoryManager) Read() (string, error) {
	if _, err := os.Stat(m.MemoryPath); os.IsNotExist(err) {
		return "", nil
	}
	data, err := os.ReadFile(m.MemoryPath)
	return string(data), err
}

// Append 將內容以原始文字附加到記憶檔。
//
// 記憶檔會被 ContextBuilder 讀進系統提示詞（見 context.go 的 memory/MEMORY.md），
// 因此內容必須原樣保存：任何轉義或 code fence 包裝都會讓模型讀到破損的記憶。
func (m *MemoryManager) Append(content string) error {
	if len(content) > MaxMessageLength {
		content = content[:MaxMessageLength]
	}

	// 目錄不一定存在（只有 onboard 會建立 workspace/memory），寫入前自行建立
	// 權限 0700：記憶內容會被讀進系統提示詞
	if err := os.MkdirAll(filepath.Dir(m.MemoryPath), 0700); err != nil {
		return fmt.Errorf("failed to create memory directory: %w", err)
	}

	f, err := os.OpenFile(m.MemoryPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.WriteString(content + "\n"); err != nil {
		return err
	}
	return nil
}

// ============================================================================
// MemoryWriteTool: 長期記憶的寫入端
// ============================================================================
// 在加入這個工具之前，MEMORY.md 只會被 ContextBuilder 讀進系統提示詞，專案裡
// 沒有任何地方寫入它——等於長期記憶是一條空管線。這個工具補上寫入端。
// ============================================================================
type MemoryWriteTool struct {
	Memory *MemoryManager
}

func (t *MemoryWriteTool) Name() string { return "memory_write" }

func (t *MemoryWriteTool) Description() string {
	return i18n.GetInstance().T("tools.memory_write")
}

func (t *MemoryWriteTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"content": map[string]any{
				"type":        "string",
				"description": i18n.GetInstance().T("tool_params.memory_content"),
			},
		},
		"required": []string{"content"},
	}
}

func (t *MemoryWriteTool) Execute(ctx context.Context, args map[string]any) *tools.ToolResult {
	content, _ := args["content"].(string)

	if strings.TrimSpace(content) == "" {
		return &tools.ToolResult{ForLLM: "Error: content is required", IsError: true}
	}

	if err := t.Memory.Append(content); err != nil {
		return &tools.ToolResult{ForLLM: fmt.Sprintf("Failed to write memory: %v", err), IsError: true}
	}

	// 記憶會被讀進之後每一次的系統提示詞，屬於使用者該知道的副作用，因此附上 Notice
	return &tools.ToolResult{
		ForLLM: "Memory saved. It will be available in future conversations.",
		Notice: i18n.GetInstance().T("notices.memory_written", tools.PreviewForNotice(content, 60)),
	}
}
