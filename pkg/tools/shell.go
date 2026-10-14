package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/chiisen/mini_bot/pkg/i18n"
)

type ExecTool struct {
	Sandbox *Sandbox
}

func (t *ExecTool) Name() string        { return "exec" }
func (t *ExecTool) Description() string { return i18n.GetInstance().T("tools.execute_command") }
func (t *ExecTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{"type": "string", "description": i18n.GetInstance().T("tool_params.command")},
			"timeout": map[string]any{"type": "integer", "description": "Timeout in seconds, default 30"},
		},
		"required": []string{"command"},
	}
}

// allowedCommands 是唯讀檢查類指令的白名單，這些指令不需要再進 denylist 判斷。
var allowedCommands = map[string]bool{
	"ls": true, "dir": true, "cd": true, "pwd": true,
	"cat": true, "head": true, "tail": true, "less": true,
	"grep": true, "egrep": true, "fgrep": true,
	"find": true, "xargs": true,
	"wc": true, "sort": true, "uniq": true, "cut": true,
	"echo": true, "printf": true,
	"stat": true, "file": true, "md5sum": true, "sha256sum": true,
	"tree": true,
}

var dangerPatterns = []string{
	`rm\s+-rf\s+/`,
	`del\s+/f`,
	`rmdir\s+/s`,
	`format\b`,
	`mkfs\b`,
	`diskpart\b`,
	`dd\s+if=`,
	`shutdown\b`,
	`reboot\b`,
	`poweroff\b`,
	`:\(\)\{\s+:\|:&\s+\};:`,
	`;\s*rm\s+`,
	`\|\s*rm\s+`,
	`&&\s*rm\s+`,
	`\|\|\s*rm\s+`,
	`\$\(.*\)`,
	"`.*`",
}

// dangerCommands 是會解譯程式碼或大範圍修改系統的指令。
//
// 注意這是 denylist：不在名單內、也不在 allowedCommands 的指令預設會被放行
// （例如 sleep）。真正的邊界由 validatePaths 的路徑檢查與 shell 控制字元的
// 拒絕負責，兩者合起來才是 exec 的防線（見 README 的安全性章節）。
var dangerCommands = []string{
	"curl", "wget", "nc", "ncat", "bash", "powershell", "sh",
	"python", "python3", "perl", "ruby", "php", "node",
	"npm", "pip", "cargo", "go", "rustc",
	"ssh", "scp", "sftp", "ftp", "telnet",
	"chmod", "chown", "chgrp",
	"kill", "killall", "pkill",
	"mount", "umount",
	"fdisk", "parted", "sfdisk",
	"sudo", "su",
	"git", "svn", "hg",
	"docker", "kubectl", "helm",
	"tar", "gzip", "bzip2", "xz", "zip", "unzip",
	"sed", "awk", "vim", "nano", "emacs",
	"ln", "unlink", "touch", "mkdir", "rmdir",
}

// shellMetaCharacters 是會改變指令結構的字元。
//
// 出現時直接拒絕，而不是像先前那樣把它們悄悄移除：靜默改寫會讓指令變成
// 另一個東西（例如 `find . | wc -l` 變成 `find . wc -l`），使用者拿到與原意
// 無關的結果卻不知道為什麼。
const shellMetaCharacters = ";|&$`()<>\n"

func containsShellMeta(cmd string) bool {
	return strings.ContainsAny(cmd, shellMetaCharacters)
}

func validateCommand(cmd string) bool {
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return false
	}

	if len(cmd) > 500 {
		return false
	}

	baseCmd := strings.ToLower(fields[0])

	if _, ok := allowedCommands[baseCmd]; ok {
		return true
	}

	for _, dc := range dangerCommands {
		if baseCmd == dc {
			return false
		}
	}

	return true
}

func isCommandSafe(cmd string) bool {
	for _, pattern := range dangerPatterns {
		matched, _ := regexp.MatchString(pattern, cmd)
		if matched {
			return false
		}
	}
	return true
}

// looksLikePath 判斷欄位是否可能是路徑，避免把純文字參數（例如 grep 的樣式）
// 也當成路徑處理。
func looksLikePath(field string) bool {
	return field == "." || field == ".." ||
		strings.ContainsAny(field, `/\`)
}

// validatePaths 檢查指令中出現的路徑是否都落在 workspace 之內。
//
// 這是 exec 唯一能做的越界防護：它以空白切分指令，因此不處理引號與轉義，
// 屬於盡力而為的防線，不是作業系統層級的隔離。真正的隔離要靠容器執行
// （專案已附 Dockerfile）。
func (t *ExecTool) validatePaths(cmd string) error {
	for _, field := range strings.Fields(cmd) {
		candidate := field

		// 支援 --flag=/path 形式
		if strings.HasPrefix(candidate, "-") {
			_, value, found := strings.Cut(candidate, "=")
			if !found {
				continue
			}
			candidate = value
		}

		// ~ 會被 shell 展開成家目錄，這裡先自行展開再檢查；無法解析時一律拒絕
		if strings.HasPrefix(candidate, "~") {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("cannot resolve home directory for %s", field)
			}
			candidate = filepath.Join(home, strings.TrimPrefix(candidate, "~"))
		}

		if !looksLikePath(candidate) {
			continue
		}

		if _, err := t.Sandbox.CheckPath(candidate); err != nil {
			return fmt.Errorf("path outside workspace: %s", field)
		}
	}

	return nil
}

func (t *ExecTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	cmdStr, _ := args["command"].(string)

	if strings.TrimSpace(cmdStr) == "" {
		return &ToolResult{ForLLM: "Error: command rejected by sandbox: empty command.", IsError: true}
	}

	if len(cmdStr) > 500 {
		return &ToolResult{ForLLM: "Error: command rejected by sandbox: command too long (max 500 characters).", IsError: true}
	}

	if containsShellMeta(cmdStr) {
		return &ToolResult{
			ForLLM:  "Error: command rejected by sandbox: shell metacharacters (; | & $ ` ( ) < >) are not allowed. Split it into separate simple commands.",
			IsError: true,
		}
	}

	if !validateCommand(cmdStr) || !isCommandSafe(cmdStr) {
		return &ToolResult{ForLLM: "Error: command rejected due to safety sandbox restrictions.", IsError: true}
	}

	if err := t.validatePaths(cmdStr); err != nil {
		return &ToolResult{ForLLM: fmt.Sprintf("Error: command rejected by sandbox: %v", err), IsError: true}
	}

	timeoutSec := float64(30)
	if to, ok := args["timeout"].(float64); ok && to > 0 {
		timeoutSec = to
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	// Determine the shell based on OS
	var c *exec.Cmd
	if runtime.GOOS == "windows" {
		c = exec.CommandContext(timeoutCtx, "cmd", "/c", cmdStr)
	} else {
		c = exec.CommandContext(timeoutCtx, "sh", "-c", cmdStr)
	}
	c.Dir = t.Sandbox.Workspace

	outBytes, err := c.CombinedOutput()
	resultStr := string(outBytes)

	if timeoutCtx.Err() == context.DeadlineExceeded {
		return &ToolResult{ForLLM: fmt.Sprintf("Timeout after %.0f seconds.\nOutput: %s", timeoutSec, resultStr), IsError: true}
	}

	if err != nil {
		return &ToolResult{ForLLM: fmt.Sprintf("Error: %v\nOutput: %s", err, resultStr), IsError: true}
	}

	return &ToolResult{ForLLM: fmt.Sprintf("Command exited successfully.\nOutput: %s", resultStr), IsError: false}
}
