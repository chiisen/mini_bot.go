package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// okBody 產生一個最小但完整的 OpenAI 相容回應。
func okBody(content string) string {
	return `{"choices":[{"message":{"role":"assistant","content":"` + content + `"}}],` +
		`"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`
}

// newTestProvider 建立指向測試伺服器的 provider，並把退避時間縮到最短，
// 讓重試測試不必真的等待。
func newTestProvider(t *testing.T, baseURL string, timeout time.Duration) *OpenAICompatProvider {
	t.Helper()

	p := NewOpenAICompatProvider(baseURL, "test-key", timeout)
	p.retryBaseDelay = time.Millisecond
	return p
}

// countingServer 回傳測試伺服器與其被呼叫次數，handler 會收到當前的嘗試次數。
func countingServer(t *testing.T, handler func(w http.ResponseWriter, r *http.Request, attempt int32)) (*httptest.Server, *int32) {
	t.Helper()

	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler(w, r, atomic.AddInt32(&attempts, 1))
	}))
	t.Cleanup(server.Close)

	return server, &attempts
}

func TestChat_SendsExpectedRequest(t *testing.T) {
	var path, auth string
	var body map[string]any

	server, _ := countingServer(t, func(w http.ResponseWriter, r *http.Request, _ int32) {
		path = r.URL.Path
		auth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&body)
		fmt.Fprint(w, okBody("hello"))
	})

	p := newTestProvider(t, server.URL+"/v1", 5*time.Second)

	resp, err := p.Chat(context.Background(),
		[]Message{{Role: "user", Content: "hi"}},
		[]ToolDefinition{{Type: "function", Function: ToolFunctionDefinition{Name: "read_file"}}},
		"gpt-test",
		map[string]any{"temperature": 0.2},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if path != "/v1/chat/completions" {
		t.Errorf("expected path /v1/chat/completions, got %s", path)
	}
	if auth != "Bearer test-key" {
		t.Errorf("expected bearer token, got %q", auth)
	}
	if body["model"] != "gpt-test" {
		t.Errorf("expected model gpt-test, got %v", body["model"])
	}
	if body["temperature"] != 0.2 {
		t.Errorf("expected the temperature option to be applied, got %v", body["temperature"])
	}
	if _, ok := body["tools"]; !ok {
		t.Error("expected tools to be included in the payload")
	}
	if resp.Content != "hello" {
		t.Errorf("expected content hello, got %q", resp.Content)
	}
	if resp.Usage.TotalTokens != 3 {
		t.Errorf("expected total tokens 3, got %d", resp.Usage.TotalTokens)
	}
}

func TestChat_DoesNotDuplicateEndpointPath(t *testing.T) {
	var path string

	server, _ := countingServer(t, func(w http.ResponseWriter, r *http.Request, _ int32) {
		path = r.URL.Path
		fmt.Fprint(w, okBody("ok"))
	})

	p := newTestProvider(t, server.URL+"/v1/chat/completions", 5*time.Second)

	if _, err := p.Chat(context.Background(), nil, nil, "m", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != "/v1/chat/completions" {
		t.Errorf("expected path /v1/chat/completions, got %s", path)
	}
}

func TestChat_RetriesOnServerError(t *testing.T) {
	server, attempts := countingServer(t, func(w http.ResponseWriter, r *http.Request, attempt int32) {
		if attempt == 1 {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		fmt.Fprint(w, okBody("recovered"))
	})

	p := newTestProvider(t, server.URL, 5*time.Second)

	resp, err := p.Chat(context.Background(), nil, nil, "m", nil)
	if err != nil {
		t.Fatalf("expected the retry to succeed, got: %v", err)
	}
	if resp.Content != "recovered" {
		t.Errorf("expected recovered content, got %q", resp.Content)
	}
	if got := atomic.LoadInt32(attempts); got != 2 {
		t.Errorf("expected 2 attempts, got %d", got)
	}
}

func TestChat_RetriesOnRateLimit(t *testing.T) {
	server, attempts := countingServer(t, func(w http.ResponseWriter, r *http.Request, attempt int32) {
		if attempt == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		fmt.Fprint(w, okBody("ok"))
	})

	p := newTestProvider(t, server.URL, 5*time.Second)

	if _, err := p.Chat(context.Background(), nil, nil, "m", nil); err != nil {
		t.Fatalf("expected the retry to succeed, got: %v", err)
	}
	if got := atomic.LoadInt32(attempts); got != 2 {
		t.Errorf("expected 2 attempts, got %d", got)
	}
}

func TestChat_RetriesTransportError(t *testing.T) {
	server, attempts := countingServer(t, func(w http.ResponseWriter, r *http.Request, attempt int32) {
		if attempt == 1 {
			// 直接中斷連線，模擬網路層失敗
			panic(http.ErrAbortHandler)
		}
		fmt.Fprint(w, okBody("ok"))
	})

	p := newTestProvider(t, server.URL, 5*time.Second)

	if _, err := p.Chat(context.Background(), nil, nil, "m", nil); err != nil {
		t.Fatalf("expected the retry to recover from a transport error, got: %v", err)
	}
	if got := atomic.LoadInt32(attempts); got != 2 {
		t.Errorf("expected 2 attempts, got %d", got)
	}
}

func TestChat_DoesNotRetryClientError(t *testing.T) {
	server, attempts := countingServer(t, func(w http.ResponseWriter, r *http.Request, _ int32) {
		http.Error(w, "bad request", http.StatusBadRequest)
	})

	p := newTestProvider(t, server.URL, 5*time.Second)

	_, err := p.Chat(context.Background(), nil, nil, "m", nil)
	if err == nil {
		t.Fatal("expected an error for HTTP 400")
	}
	if !strings.Contains(err.Error(), "status 400") {
		t.Errorf("expected the status in the error, got %v", err)
	}
	if got := atomic.LoadInt32(attempts); got != 1 {
		t.Errorf("客戶端錯誤不應重試，實際嘗試 %d 次", got)
	}
}

func TestChat_ExhaustsAttempts(t *testing.T) {
	server, attempts := countingServer(t, func(w http.ResponseWriter, r *http.Request, _ int32) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	})

	p := newTestProvider(t, server.URL, 5*time.Second)

	_, err := p.Chat(context.Background(), nil, nil, "m", nil)
	if err == nil {
		t.Fatal("expected an error after exhausting attempts")
	}
	if !strings.Contains(err.Error(), "status 503") {
		t.Errorf("expected the last status in the error, got %v", err)
	}
	if got := atomic.LoadInt32(attempts); got != int32(DefaultMaxAttempts) {
		t.Errorf("expected %d attempts, got %d", DefaultMaxAttempts, got)
	}
}

func TestChat_ContextCancelDuringBackoff(t *testing.T) {
	server, attempts := countingServer(t, func(w http.ResponseWriter, r *http.Request, _ int32) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	})

	p := newTestProvider(t, server.URL, 5*time.Second)
	p.retryBaseDelay = 5 * time.Second // 遠長於取消時機

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err := p.Chat(ctx, nil, nil, "m", nil)
	elapsed := time.Since(start)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	if elapsed > 2*time.Second {
		t.Errorf("等待退避時應立即被中斷，實際耗時 %s", elapsed)
	}
	if got := atomic.LoadInt32(attempts); got != 1 {
		t.Errorf("取消前應只有 1 次嘗試，實際 %d 次", got)
	}
}

func TestChat_DoesNotRetryTimeout(t *testing.T) {
	server, attempts := countingServer(t, func(w http.ResponseWriter, r *http.Request, _ int32) {
		time.Sleep(200 * time.Millisecond)
		fmt.Fprint(w, okBody("late"))
	})

	p := newTestProvider(t, server.URL, 50*time.Millisecond)

	_, err := p.Chat(context.Background(), nil, nil, "m", nil)
	if err == nil {
		t.Fatal("expected a timeout error")
	}
	if got := atomic.LoadInt32(attempts); got != 1 {
		t.Errorf("逾時不應重試，實際嘗試 %d 次", got)
	}
}
