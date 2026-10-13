package agent

import (
	"os"
)

const MaxMessageLength = 10000

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
