package providers

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"
)

const (
	// DefaultRequestTimeout 是單次 LLM 請求的預設逾時。
	DefaultRequestTimeout = 60 * time.Second
	// DefaultMaxAttempts 是含首次在內的總嘗試次數。
	DefaultMaxAttempts = 3
	// DefaultRetryBaseDelay 是重試退避的基準，每次重試倍增（500ms、1s…）。
	DefaultRetryBaseDelay = 500 * time.Millisecond
	// maxRetryAfter 限制伺服器 Retry-After 造成的等待上限，
	// 避免單一請求被要求等待過久。
	maxRetryAfter = 30 * time.Second
)

type tlsLoggingTransport struct {
	transport *http.Transport
}

func (t *tlsLoggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.transport.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	if resp.TLS != nil {
		slog.Debug("TLS connection established", "version", tlsVersionName(resp.TLS.Version))
	}
	return resp, nil
}

func tlsVersionName(v uint16) string {
	switch v {
	case tls.VersionTLS10:
		return "TLS 1.0"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS13:
		return "TLS 1.3"
	default:
		return fmt.Sprintf("unknown (%d)", v)
	}
}

type OpenAICompatProvider struct {
	BaseURL string
	APIKey  string
	client  *http.Client

	// 可由同 package 的測試調整，讓重試測試不必真的等待退避時間
	maxAttempts    int
	retryBaseDelay time.Duration
}

// NewOpenAICompatProvider 建立 OpenAI 相容協定的 provider。
// timeout 為單次請求逾時，<= 0 時採用 DefaultRequestTimeout。
func NewOpenAICompatProvider(baseURL, apiKey string, timeout time.Duration) *OpenAICompatProvider {
	if timeout <= 0 {
		timeout = DefaultRequestTimeout
	}

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	baseTransport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	transport := &tlsLoggingTransport{transport: baseTransport}

	return &OpenAICompatProvider{
		BaseURL:        baseURL,
		APIKey:         apiKey,
		client:         &http.Client{Transport: transport, Timeout: timeout},
		maxAttempts:    DefaultMaxAttempts,
		retryBaseDelay: DefaultRetryBaseDelay,
	}
}

func (p *OpenAICompatProvider) GetDefaultModel() string {
	return "gpt-4"
}

// Chat 送出對話請求，並對暫時性失敗自動重試。
//
// 重試範圍（見 isRetryableStatus）：
//   - HTTP 429（速率限制）與 408
//   - HTTP 5xx（伺服器端錯誤）
//   - 連線層錯誤（DNS、連線中斷等）
//
// 其他 4xx 屬於請求本身的問題，重試沒有意義，會直接回傳錯誤。
// 退避採指數成長，並優先採用伺服器回傳的 Retry-After（上限 maxRetryAfter）；
// 等待期間若 context 被取消會立即返回，不會讓使用者無謂等待。
func (p *OpenAICompatProvider) Chat(
	ctx context.Context,
	messages []Message,
	tools []ToolDefinition,
	model string,
	options map[string]any,
) (*LLMResponse, error) {

	url := p.chatCompletionsURL()

	payload := map[string]any{
		"model":    model,
		"messages": messages,
	}

	if len(tools) > 0 {
		payload["tools"] = tools
	}

	// Apply options (temperature, max_tokens, etc.)
	for k, v := range options {
		payload[k] = v
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	var lastErr error

	for attempt := 1; attempt <= p.maxAttempts; attempt++ {
		resp, bodyText, err := p.doRequest(ctx, url, jsonData)
		if err != nil {
			lastErr = fmt.Errorf("http request failed: %w", err)

			// 逾時或 context 結束都不再重試：使用者設定的逾時就是等待上限，
			// 重試會讓總等待時間變成數倍。
			if ctx.Err() != nil || isTimeoutError(err) || attempt == p.maxAttempts {
				return nil, lastErr
			}
			if waitErr := p.waitBeforeRetry(ctx, attempt, 0); waitErr != nil {
				return nil, waitErr
			}
			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return parseChatResponse(bodyText)
		}

		lastErr = fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(bodyText))

		if !isRetryableStatus(resp.StatusCode) || attempt == p.maxAttempts {
			return nil, lastErr
		}

		slog.Debug("retrying LLM request", "status", resp.StatusCode, "attempt", attempt)
		if waitErr := p.waitBeforeRetry(ctx, attempt, retryAfterDelay(resp)); waitErr != nil {
			return nil, waitErr
		}
	}

	return nil, lastErr
}

// chatCompletionsURL 組合出 chat completions 端點，並避免重複附加路徑。
func (p *OpenAICompatProvider) chatCompletionsURL() string {
	url := p.BaseURL

	if !bytes.HasSuffix([]byte(url), []byte("/chat/completions")) {
		if url != "" && url[len(url)-1] != '/' {
			url += "/"
		}
		url += "chat/completions"
	}

	return url
}

// doRequest 送出單次請求並讀完回應；狀態碼由呼叫端判斷。
func (p *OpenAICompatProvider) doRequest(ctx context.Context, url string, payload []byte) (*http.Response, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if p.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp, nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return resp, body, nil
}

// parseChatResponse 解析 OpenAI 相容的回應格式。
func parseChatResponse(bodyText []byte) (*LLMResponse, error) {
	var response struct {
		Choices []struct {
			Message struct {
				Content   string     `json:"content"`
				ToolCalls []ToolCall `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(bodyText, &response); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w - body: %s", err, string(bodyText))
	}

	if len(response.Choices) == 0 {
		return nil, fmt.Errorf("no choices returned from API")
	}

	msg := response.Choices[0].Message
	return &LLMResponse{
		Content:   msg.Content,
		ToolCalls: msg.ToolCalls,
		Usage: UsageInfo{
			PromptTokens:     response.Usage.PromptTokens,
			CompletionTokens: response.Usage.CompletionTokens,
			TotalTokens:      response.Usage.TotalTokens,
		},
	}, nil
}

// isRetryableStatus 指出該狀態碼是否屬於「稍後再試可能成功」的暫時性失敗。
func isRetryableStatus(code int) bool {
	return code == http.StatusTooManyRequests ||
		code == http.StatusRequestTimeout ||
		code >= 500
}

// isTimeoutError 判斷錯誤是否來自逾時（含 http.Client.Timeout 與 context deadline）。
func isTimeoutError(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return errors.Is(err, context.DeadlineExceeded)
}

// retryAfterDelay 解析 Retry-After 標頭（秒數），並限制在 maxRetryAfter 之內。
func retryAfterDelay(resp *http.Response) time.Duration {
	value := resp.Header.Get("Retry-After")
	if value == "" {
		return 0
	}

	seconds, err := strconv.Atoi(value)
	if err != nil || seconds <= 0 {
		return 0
	}

	delay := time.Duration(seconds) * time.Second
	if delay > maxRetryAfter {
		return maxRetryAfter
	}
	return delay
}

// waitBeforeRetry 等待退避時間；等待期間若 context 結束則立即返回錯誤。
func (p *OpenAICompatProvider) waitBeforeRetry(ctx context.Context, attempt int, hint time.Duration) error {
	delay := hint
	if delay <= 0 {
		delay = p.retryBaseDelay << (attempt - 1)
		// 位移過大或設定異常時退回上限，避免負值或無限等待
		if delay <= 0 || delay > maxRetryAfter {
			delay = maxRetryAfter
		}
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
