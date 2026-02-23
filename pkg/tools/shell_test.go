package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestExecTool_SafeCommand(t *testing.T) {
	tmpDir := t.TempDir()
	sandbox, _ := NewSandbox(tmpDir)

	// Create a test script
	scriptPath := filepath.Join(tmpDir, "test.sh")
	os.WriteFile(scriptPath, []byte("#!/bin/bash\necho hello"), 0755)

	tool := ExecTool{Sandbox: sandbox}
	result := tool.Execute(context.Background(), map[string]any{
		"command": "echo 'Hello World'",
	})

	if result.IsError {
		t.Errorf("unexpected error: %s", result.ForLLM)
	}
	if result.ForLLM == "" {
		t.Error("expected output")
	}
}

func TestExecTool_DangerousCommands(t *testing.T) {
	tmpDir := t.TempDir()
	sandbox, _ := NewSandbox(tmpDir)

	tool := ExecTool{Sandbox: sandbox}

	dangerousCommands := []string{
		"rm -rf /",
		"rm -rf /home",
		"del /f C:\\Windows",
		"format D:",
		"mkfs /dev/sda",
		"dd if=/dev/zero of=/dev/sda",
		"shutdown now",
		"reboot",
		"poweroff",
		":(){ :|:& };:",
	}

	for _, cmd := range dangerousCommands {
		result := tool.Execute(context.Background(), map[string]any{"command": cmd})
		if !result.IsError {
			t.Errorf("expected error for dangerous command: %s", cmd)
		}
	}
}

func TestExecTool_Timeout(t *testing.T) {
	tmpDir := t.TempDir()
	sandbox, _ := NewSandbox(tmpDir)

	tool := ExecTool{Sandbox: sandbox}
	result := tool.Execute(context.Background(), map[string]any{
		"command": "sleep 10",
		"timeout": 1.0,
	})

	if !result.IsError {
		t.Error("expected error for timeout")
	}
}

func TestExecTool_NonZeroExit(t *testing.T) {
	tmpDir := t.TempDir()
	sandbox, _ := NewSandbox(tmpDir)

	tool := ExecTool{Sandbox: sandbox}
	result := tool.Execute(context.Background(), map[string]any{
		"command": "exit 1",
	})

	// Should return error for non-zero exit
	if result.ForLLM == "" {
		t.Error("expected output")
	}
}

func TestIsCommandSafe(t *testing.T) {
	safeCommands := []string{
		"ls",
		"ls -la",
		"cat file.txt",
		"grep 'pattern' file",
		"echo hello",
		"pwd",
		"mkdir testdir",
		"cp file1 file2",
	}

	for _, cmd := range safeCommands {
		if !isCommandSafe(cmd) {
			t.Errorf("expected safe command to pass: %s", cmd)
		}
	}

	unsafeCommands := []string{
		"rm -rf /",
		"rm -rf /home/user",
		"format C:",
		"dd if=/dev/zero of=/dev/sda",
		"shutdown -h now",
		"reboot",
		":(){ :|:& };:",
	}

	for _, cmd := range unsafeCommands {
		if isCommandSafe(cmd) {
			t.Errorf("expected unsafe command to fail: %s", cmd)
		}
	}
}
