package tools

import (
	"context"
)
// Tool Result wraps string response and error state for LLM
type ToolResult struct {
	ForLLM  string
	IsError bool
}

// Tool represents the interface for a callable tool for the Agent
type Tool interface {
	Name() string
	Description() string
	Parameters() map[string]any // JSON schema format map
	Execute(ctx context.Context, args map[string]any) *ToolResult
}
