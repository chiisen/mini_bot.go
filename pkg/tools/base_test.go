package tools

import (
	"context"
	"testing"
)

type mockTool struct {
	name        string
	description string
	executeFunc func(ctx context.Context, args map[string]any) *ToolResult
}

func (m *mockTool) Name() string        { return m.name }
func (m *mockTool) Description() string { return m.description }
func (m *mockTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"arg": map[string]any{"type": "string"},
		},
	}
}
func (m *mockTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	return m.executeFunc(ctx, args)
}

func TestToolResult(t *testing.T) {
	result := ToolResult{
		ForLLM:  "success output",
		IsError: false,
	}

	if result.ForLLM != "success output" {
		t.Errorf("expected 'success output', got '%s'", result.ForLLM)
	}
	if result.IsError {
		t.Error("expected IsError to be false")
	}

	errorResult := ToolResult{
		ForLLM:  "error occurred",
		IsError: true,
	}

	if !errorResult.IsError {
		t.Error("expected IsError to be true")
	}
}

func TestToolInterface(t *testing.T) {
	tool := &mockTool{
		name:        "test_tool",
		description: "A test tool",
		executeFunc: func(ctx context.Context, args map[string]any) *ToolResult {
			return &ToolResult{ForLLM: "executed", IsError: false}
		},
	}

	if tool.Name() != "test_tool" {
		t.Errorf("expected name 'test_tool', got '%s'", tool.Name())
	}
	if tool.Description() != "A test tool" {
		t.Errorf("expected description 'A test tool', got '%s'", tool.Description())
	}

	params := tool.Parameters()
	if params["type"] != "object" {
		t.Errorf("expected type 'object', got '%v'", params["type"])
	}

	ctx := context.Background()
	result := tool.Execute(ctx, map[string]any{"arg": "value"})
	if result.ForLLM != "executed" {
		t.Errorf("expected 'executed', got '%s'", result.ForLLM)
	}
}
