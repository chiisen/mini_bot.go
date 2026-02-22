package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/chiisen/mini_bot/pkg/providers"
)

type Builder struct {
	WorkspacePath string
}

func NewContextBuilder(workspacePath string) *Builder {
	return &Builder{
		WorkspacePath: workspacePath,
	}
}

// Build constructs the system prompt from markdown files in the workspace
func (b *Builder) Build(tools []providers.ToolDefinition) (string, error) {
	var parts []string

	// Define load order and section headers
	filesToLoad := []struct {
		FileName string
		Header   string
	}{
		{"IDENTITY.md", "[IDENTITY]"},
		{"AGENT.md", "[AGENT GUIDELINES]"},
		{"SOUL.md", "[PERSONALITY]"},
		{"USER.md", "[USER PREFERENCES]"},
		{filepath.Join("memory", "MEMORY.md"), "[MEMORY]"},
	}

	for _, req := range filesToLoad {
		path := filepath.Join(b.WorkspacePath, req.FileName)
		if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
			parts = append(parts, req.Header)
			parts = append(parts, string(data))
		}
	}

	// Append tool usage guidelines to the system prompt
	if len(tools) > 0 {
		var toolDesc strings.Builder
		toolDesc.WriteString("[AVAILABLE TOOLS]\n")
		toolDesc.WriteString("You have access to the following tools:\n")
		for _, t := range tools {
			toolDesc.WriteString(fmt.Sprintf("- %s: %s\n", t.Function.Name, t.Function.Description))
		}
		toolDesc.WriteString("\nWhen you need to perform an action, output a tool call request. Do not try to hallucinate commands execution in plain text, use the provided tools.\n")
		parts = append(parts, toolDesc.String())
	}

	return strings.Join(parts, "\n\n"), nil
}
