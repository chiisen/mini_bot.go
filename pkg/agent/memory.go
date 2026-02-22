package agent

import (
	"os"
)

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
		return "", nil // Memory file not existing is not an error
	}
	data, err := os.ReadFile(m.MemoryPath)
	return string(data), err
}

func (m *MemoryManager) Append(content string) error {
	f, err := os.OpenFile(m.MemoryPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.WriteString(content + "\n"); err != nil {
		return err
	}
	return nil
}
