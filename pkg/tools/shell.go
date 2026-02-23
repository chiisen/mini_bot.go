package tools

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"time"
)

type ExecTool struct {
	Sandbox *Sandbox
}

func (t *ExecTool) Name() string        { return "exec" }
func (t *ExecTool) Description() string { return "Execute a shell command inside the workspace" }
func (t *ExecTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{"type": "string", "description": "The shell command to execute"},
			"timeout": map[string]any{"type": "integer", "description": "Timeout in seconds, default 30"},
		},
		"required": []string{"command"},
	}
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

func (t *ExecTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	cmdStr, _ := args["command"].(string)

	if !isCommandSafe(cmdStr) {
		return &ToolResult{ForLLM: "Error: command rejected due to safety sandbox restrictions.", IsError: true}
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
