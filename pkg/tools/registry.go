package tools

import (
	"context"
	"fmt"
	"github.com/chiisen/mini_bot/pkg/providers"
)

type ToolRegistry struct {
	tools map[string]Tool
}

func NewRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]Tool),
	}
}

func (r *ToolRegistry) Register(tool Tool) {
	r.tools[tool.Name()] = tool
}

// Execute looks up the tool by name and executes it with provided arguments
func (r *ToolRegistry) Execute(ctx context.Context, name string, args map[string]any) *ToolResult {
	t, ok := r.tools[name]
	if !ok {
		return &ToolResult{
			ForLLM:  fmt.Sprintf("Tool '%s' not found.", name),
			IsError: true,
		}
	}

	// Capture panics gracefully
	var res *ToolResult
	func() {
		defer func() {
			if rec := recover(); rec != nil {
				res = &ToolResult{
					ForLLM:  fmt.Sprintf("Tool panic: %v", rec),
					IsError: true,
				}
			}
		}()
		res = t.Execute(ctx, args)
	}()
	return res
}

// Definitions returns the definitions of all registered tools for the LLM
func (r *ToolRegistry) Definitions() []providers.ToolDefinition {
	var defs []providers.ToolDefinition
	for _, t := range r.tools {
		defs = append(defs, providers.ToolDefinition{
			Type: "function",
			Function: providers.ToolFunctionDefinition{
				Name:        t.Name(),
				Description: t.Description(),
				Parameters:  t.Parameters(),
			},
		})
	}
	return defs
}
