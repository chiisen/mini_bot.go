package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/chiisen/mini_bot/pkg/providers"
)

// Run executes a full LLM + Tool calling loop.
func (a *AgentInstance) Run(
	ctx context.Context,
	sessionKey string,
	userInput string,
	onReply func(msg string),
) error {

	// 1. Build System Prompt
	toolDefs := a.Registry.Definitions()
	systemPrompt, err := a.CtxBuilder.Build(toolDefs)
	if err != nil {
		return fmt.Errorf("failed to build system context: %w", err)
	}

	// 2. Load Session History
	history, err := a.Sessions.Load(sessionKey)
	if err != nil {
		return fmt.Errorf("failed to load session %s context: %w", sessionKey, err)
	}

	// 3. Assemble complete messages list
	// We put SYSTEM prompt at index 0 every time to ensure it is fresh
	messages := make([]providers.Message, 0, len(history)+2)

	messages = append(messages, providers.Message{
		Role:    "system",
		Content: systemPrompt,
	})

	messages = append(messages, history...)

	// Append new user input
	messages = append(messages, providers.Message{
		Role:    "user",
		Content: userInput,
	})

	// Optional: Compress context if needed
	messages = a.Sessions.Compress(messages, a.Config.Agents.Defaults.MaxTokens)

	// 4. Tool Execution Loop
	maxIters := a.Config.Agents.Defaults.MaxToolIterations
	iterations := 0

	for iterations < maxIters {
		if err := ctx.Err(); err != nil {
			return err
		}
		iterations++

		// Strip vendor prefix if present
		modelName := a.Config.Agents.Defaults.Model
		if parts := strings.SplitN(modelName, "/", 2); len(parts) == 2 {
			modelName = parts[1]
		}

		// Send request to LLM
		response, err := a.Provider.Chat(ctx, messages, toolDefs, modelName, map[string]any{
			"temperature": a.Config.Agents.Defaults.Temperature,
			"max_tokens":  a.Config.Agents.Defaults.MaxTokens,
		})
		
		if err != nil {
			return fmt.Errorf("llm chat provider error: %w", err)
		}

		// Append LLM's raw message to our conversational state so the LLM remembers what it said/called.
		// Note that OpenAI API requires tool_calls to be echoed back in the conversational history precisely.
		messages = append(messages, providers.Message{
			Role:      "assistant",
			Content:   response.Content,
			ToolCalls: response.ToolCalls,
		})

		// If AI replies with text, we invoke onReply.
		if response.Content != "" {
			onReply(response.Content)
			// Return here normally if tools are not called, OR we can let it proceed to check tool calls if any...
			// Some LLMs output both Content AND Tool calls at the same time.
			// If ToolCalls is empty, we break.
			if len(response.ToolCalls) == 0 {
				break
			}
		}

		// Handle ToolCalls if AI requested any
		if len(response.ToolCalls) > 0 {
			for _, call := range response.ToolCalls {
				// Parse arguments
				var args map[string]any
				if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
					// Add an error back into the loop
					messages = append(messages, providers.Message{
						Role:       "tool",
						Content:    fmt.Sprintf("Failed to parse tool arguments: %v", err),
						ToolCallID: call.ID,
					})
					continue
				}

				// Execute tool
				onReply(fmt.Sprintf("[Agent uses tool: %s...]", call.Function.Name))
				result := a.Registry.Execute(ctx, call.Function.Name, args)

				// Report result
				messages = append(messages, providers.Message{
					Role:       "tool",
					Content:    result.ForLLM,
					ToolCallID: call.ID,
				})
			}
			// Let the loop continue to feed tool output back to the LLM
		} else {
			// No tools requested, and we've printed logic, break loop.
			break
		}
	}

	if iterations >= maxIters {
		onReply("[Agent stopped: reached maximum tool iteration limit]")
	}

	// 5. Save updated conversation history.
	// Strip the `system` prompt before saving to prevent duplicating it.
	var historyToSave []providers.Message
	for _, m := range messages {
		if m.Role != "system" {
			historyToSave = append(historyToSave, m)
		}
	}

	if err := a.Sessions.Save(sessionKey, historyToSave); err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}

	return nil
}
