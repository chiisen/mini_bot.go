package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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

// TestExecTool_RejectsShellMetacharacters 驗證含 shell 控制字元的指令會被明確拒絕。
//
// 先前這些字元會被悄悄剝除後照樣執行（`find . | wc -l` 變成 `find . wc -l`），
// 使用者拿到與原指令無關的結果；現在改為拒絕並說明原因。
func TestExecTool_RejectsShellMetacharacters(t *testing.T) {
	tmpDir := t.TempDir()
	sandbox, _ := NewSandbox(tmpDir)
	tool := ExecTool{Sandbox: sandbox}

	rejected := []string{
		"ls; whoami",
		"find . | wc -l",
		"echo hi > out.txt",
		"cat a && cat b",
		"echo $(whoami)",
		"echo `id`",
		"ls < input.txt",
	}

	for _, cmd := range rejected {
		result := tool.Execute(context.Background(), map[string]any{"command": cmd})
		if !result.IsError {
			t.Errorf("expected %q to be rejected", cmd)
			continue
		}
		if !strings.Contains(result.ForLLM, "metacharacters") {
			t.Errorf("expected a metacharacter rejection for %q, got: %s", cmd, result.ForLLM)
		}
	}
}

// TestExecTool_RejectsPathEscape 驗證指令中指向 workspace 之外的路徑會被拒絕。
//
// exec 無法真正隔離 shell，但至少必須擋掉最常見的越界存取
// （cat /etc/passwd、find /、../ 或 ~ 展開後的家目錄）。
func TestExecTool_RejectsPathEscape(t *testing.T) {
	base := t.TempDir()
	workspace := filepath.Join(base, "workspace")
	if err := os.MkdirAll(workspace, 0755); err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}
	sandbox, _ := NewSandbox(workspace)
	tool := ExecTool{Sandbox: sandbox}

	outside := []string{
		"cat /etc/passwd",
		"find / -name passwd",
		"cat ../secret.txt",
		"ls ~",
		"grep -r secret /Users",
	}

	for _, cmd := range outside {
		result := tool.Execute(context.Background(), map[string]any{"command": cmd})
		if !result.IsError || !strings.Contains(result.ForLLM, "outside workspace") {
			t.Errorf("expected %q to be rejected as outside workspace, got: %s", cmd, result.ForLLM)
		}
	}
}

// TestExecTool_AllowsWorkspacePaths 驗證 workspace 內的相對路徑仍可正常執行。
func TestExecTool_AllowsWorkspacePaths(t *testing.T) {
	tmpDir := t.TempDir()
	sandbox, _ := NewSandbox(tmpDir)
	tool := ExecTool{Sandbox: sandbox}

	if err := os.WriteFile(filepath.Join(tmpDir, "note.txt"), []byte("hello from workspace"), 0600); err != nil {
		t.Fatalf("failed to prepare file: %v", err)
	}

	for _, cmd := range []string{"cat note.txt", "cat ./note.txt", "ls -la", "grep -r hello ."} {
		result := tool.Execute(context.Background(), map[string]any{"command": cmd})
		if result.IsError {
			t.Errorf("expected %q to be allowed, got: %s", cmd, result.ForLLM)
		}
	}
}
