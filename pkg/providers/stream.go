package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strings"
)

// StreamingProvider 是「支援逐步輸出」的 provider 的選擇性介面。
//
// 刻意不併入 LLMProvider：串流是加值能力，不支援的實作（測試替身、未來的其他供應商）
// 不需要跟著改。agent 迴圈以型別斷言偵測，不支援時退回 Chat。
type StreamingProvider interface {
	// ChatStream 與 Chat 相同，但會在收到片段時呼叫 onDelta，並在最後回傳完整回應。
	//
	// onDelta 可能被呼叫多次；呼叫順序即模型產生的順序。實作必須保證：
	// 一旦送出任何一個 delta 就不再重試（避免使用者看到重複內容）。
	ChatStream(
		ctx context.Context,
		messages []Message,
		tools []ToolDefinition,
		model string,
		options map[string]any,
		onDelta func(string),
	) (*LLMResponse, error)
}

// streamChunk 是 SSE 串流中的單一區塊。
type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
	} `json:"choices"`

	// 有些供應商會用 HTTP 200 搭配錯誤區塊回報問題
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// partialToolCall 累積同一個工具呼叫的分段內容。
//
// 串流模式下 tool_calls 是分批送來的：第一個 chunk 帶 id 與 function.name，後續
// chunk 只帶 function.arguments 的片段，必須全部拼回來才是完整的 JSON 參數。
type partialToolCall struct {
	id   string
	name string
	args strings.Builder
}

// ChatStream 以 SSE 逐步接收回應。
//
// 逾時仍是「整個請求」的上限（沿用 client 的 Timeout）；尚不區分「兩次 delta 之間的
// 閒置時間」。重試規則：只有在還沒有送出任何 delta 之前才重試（429/5xx/連線錯誤），
// 一旦已經輸出內容就讓錯誤直接往上傳，避免使用者看到重複的句子。
//
// 注意：串流模式下未必會回傳 usage（本實作不送 stream_options，以相容各種供應商），
// 因此 UsageInfo 可能是零值。
func (p *OpenAICompatProvider) ChatStream(
	ctx context.Context,
	messages []Message,
	tools []ToolDefinition,
	model string,
	options map[string]any,
	onDelta func(string),
) (*LLMResponse, error) {

	url := p.chatCompletionsURL()

	payload := buildPayload(messages, tools, model, options)
	payload["stream"] = true

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	var lastErr error

	for attempt := 1; attempt <= p.maxAttempts; attempt++ {
		resp, err := p.doStreamRequest(ctx, url, jsonData)
		if err != nil {
			lastErr = fmt.Errorf("http request failed: %w", err)

			if ctx.Err() != nil || isTimeoutError(err) || attempt == p.maxAttempts {
				return nil, lastErr
			}
			if waitErr := p.waitBeforeRetry(ctx, attempt, 0); waitErr != nil {
				return nil, waitErr
			}
			continue
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body := readAndClose(resp)
			lastErr = fmt.Errorf("API error (status %d): %s", resp.StatusCode, body)

			if !isRetryableStatus(resp.StatusCode) || attempt == p.maxAttempts {
				return nil, lastErr
			}
			if waitErr := p.waitBeforeRetry(ctx, attempt, retryAfterDelay(resp)); waitErr != nil {
				return nil, waitErr
			}
			continue
		}

		response, emitted, err := p.consumeStream(ctx, resp, onDelta)
		if err != nil {
			lastErr = err

			// 已經輸出過內容就不再重試，否則使用者會看到重複的片段
			if emitted || ctx.Err() != nil || attempt == p.maxAttempts {
				return nil, lastErr
			}
			slog.Debug("retrying stream before any output", "error", err, "attempt", attempt)
			if waitErr := p.waitBeforeRetry(ctx, attempt, 0); waitErr != nil {
				return nil, waitErr
			}
			continue
		}

		return response, nil
	}

	return nil, lastErr
}

// consumeStream 讀完整個 SSE 串流，回傳累積後的回應與「是否已送出過 delta」。
func (p *OpenAICompatProvider) consumeStream(ctx context.Context, resp *http.Response, onDelta func(string)) (*LLMResponse, bool, error) {
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	// 單一 SSE 行可能很長（例如一次寫入大檔案的工具參數），放寬預設的 64KB 上限
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var (
		content  strings.Builder
		calls    = make(map[int]*partialToolCall)
		order    []int
		emitted  bool
		finished bool
	)

	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return nil, emitted, err
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}

		data, ok := strings.CutPrefix(line, "data:")
		if !ok {
			continue
		}

		data = strings.TrimSpace(data)
		if data == "[DONE]" {
			finished = true
			break
		}

		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			slog.Debug("skipping malformed stream chunk", "error", err)
			continue
		}

		if chunk.Error != nil {
			return nil, emitted, fmt.Errorf("API error in stream: %s", chunk.Error.Message)
		}

		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				content.WriteString(choice.Delta.Content)
				emitted = true
				if onDelta != nil {
					onDelta(choice.Delta.Content)
				}
			}

			for _, tc := range choice.Delta.ToolCalls {
				call, ok := calls[tc.Index]
				if !ok {
					call = &partialToolCall{}
					calls[tc.Index] = call
					order = append(order, tc.Index)
				}
				if tc.ID != "" {
					call.id = tc.ID
				}
				if tc.Function.Name != "" {
					call.name = tc.Function.Name
				}
				call.args.WriteString(tc.Function.Arguments)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, emitted, fmt.Errorf("stream interrupted: %w", err)
	}
	if !finished && !emitted && len(calls) == 0 {
		return nil, false, fmt.Errorf("stream ended without any data")
	}

	sort.Ints(order)

	toolCalls := make([]ToolCall, 0, len(order))
	for _, index := range order {
		call := calls[index]
		toolCalls = append(toolCalls, ToolCall{
			ID:   call.id,
			Type: "function",
			Function: FunctionCall{
				Name:      call.name,
				Arguments: call.args.String(),
			},
		})
	}

	return &LLMResponse{
		Content:   content.String(),
		ToolCalls: toolCalls,
	}, emitted, nil
}

// doStreamRequest 送出串流請求並回傳尚未讀取的回應；呼叫端負責關閉 Body。
func (p *OpenAICompatProvider) doStreamRequest(ctx context.Context, url string, payload []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if p.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
	}

	return p.client.Do(req)
}

// readAndClose 讀完並關閉回應內容，用於錯誤路徑。
func readAndClose(resp *http.Response) string {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Sprintf("<failed to read body: %v>", err)
	}
	return string(body)
}
