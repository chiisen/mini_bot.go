package providers

import (
	"testing"
)

func TestMessage_JSON(t *testing.T) {
	msg := Message{
		Role:    "user",
		Content: "Hello world",
	}

	if msg.Role != "user" {
		t.Errorf("expected role user, got %s", msg.Role)
	}
	if msg.Content != "Hello world" {
		t.Errorf("expected content Hello world, got %s", msg.Content)
	}
}

func TestToolCall_JSON(t *testing.T) {
	tc := ToolCall{
		ID:   "call_123",
		Type: "function",
		Function: FunctionCall{
			Name:      "read_file",
			Arguments: `{"path": "/test.txt"}`,
		},
	}

	if tc.ID != "call_123" {
		t.Errorf("expected id call_123, got %s", tc.ID)
	}
	if tc.Function.Name != "read_file" {
		t.Errorf("expected function name read_file, got %s", tc.Function.Name)
	}
	if tc.Function.Arguments != `{"path": "/test.txt"}` {
		t.Errorf("expected arguments, got %s", tc.Function.Arguments)
	}
}

func TestLLMResponse(t *testing.T) {
	resp := LLMResponse{
		Content: "This is a response",
		ToolCalls: []ToolCall{
			{ID: "call_1", Type: "function", Function: FunctionCall{Name: "exec", Arguments: "ls"}},
		},
		Usage: UsageInfo{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
		},
	}

	if resp.Content != "This is a response" {
		t.Errorf("expected content, got %s", resp.Content)
	}
	if len(resp.ToolCalls) != 1 {
		t.Errorf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	if resp.Usage.TotalTokens != 150 {
		t.Errorf("expected 150 total tokens, got %d", resp.Usage.TotalTokens)
	}
}

func TestToolDefinition(t *testing.T) {
	def := ToolDefinition{
		Type: "function",
		Function: ToolFunctionDefinition{
			Name:        "read_file",
			Description: "Read a file from the filesystem",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "The path to the file",
					},
				},
				"required": []string{"path"},
			},
		},
	}

	if def.Type != "function" {
		t.Errorf("expected type function, got %s", def.Type)
	}
	if def.Function.Name != "read_file" {
		t.Errorf("expected function name read_file, got %s", def.Function.Name)
	}
	if def.Function.Description != "Read a file from the filesystem" {
		t.Errorf("expected description, got %s", def.Function.Description)
	}
}

func TestToolResult(t *testing.T) {
	result := ToolResult{
		ForLLM:  "File content here",
		IsError: false,
	}

	if result.ForLLM != "File content here" {
		t.Errorf("expected content, got %s", result.ForLLM)
	}
	if result.IsError {
		t.Error("expected no error")
	}

	errorResult := ToolResult{
		ForLLM:  "Error: file not found",
		IsError: true,
	}

	if !errorResult.IsError {
		t.Error("expected error")
	}
}
