package providers

import (
	"context"
)

// Message represents a single message in the conversation.
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// ToolCall represents a tool call requested by the LLM.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"` // "function"
	Function FunctionCall `json:"function"`
}

// FunctionCall contains the function name and arguments.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// LLMResponse is the standardized response from the LLM.
type LLMResponse struct {
	Content   string
	ToolCalls []ToolCall
	Usage     UsageInfo
}

// UsageInfo represents token usage statistics.
type UsageInfo struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// ToolResult represents the result of executing a tool.
type ToolResult struct {
	ForLLM  string
	IsError bool
}

// ToolDefinition is the definition of a tool sent to the LLM.
type ToolDefinition struct {
	Type     string                 `json:"type"` // "function"
	Function ToolFunctionDefinition `json:"function"`
}

// ToolFunctionDefinition represents the function details of a tool.
type ToolFunctionDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"` // JSON Schema format
}

// LLMProvider is the interface that all LLM vendors must implement.
type LLMProvider interface {
	Chat(
		ctx context.Context,
		messages []Message,
		tools []ToolDefinition,
		model string,
		options map[string]any,
	) (*LLMResponse, error)

	GetDefaultModel() string
}
