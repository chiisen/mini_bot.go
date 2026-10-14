package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// writeSSE 送出 SSE 區塊並立即 flush（否則用戶端會等到整個回應結束才收到）。
func writeSSE(w http.ResponseWriter, chunks ...string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)

	for _, chunk := range chunks {
		fmt.Fprintf(w, "data: %s\n\n", chunk)
	}

	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

func TestChatStream_TextDeltas(t *testing.T) {
	var body map[string]any

	server, _ := countingServer(t, func(w http.ResponseWriter, r *http.Request, _ int32) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		writeSSE(w,
			`{"choices":[{"delta":{"content":"Hello"}}]}`,
			`{"choices":[{"delta":{"content":" world"}}]}`,
			`[DONE]`,
		)
	})

	p := newTestProvider(t, server.URL, 5*time.Second)

	var deltas []string
	resp, err := p.ChatStream(context.Background(), nil, nil, "m", nil, func(text string) {
		deltas = append(deltas, text)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Content != "Hello world" {
		t.Errorf("expected accumulated content %q, got %q", "Hello world", resp.Content)
	}
	if len(deltas) != 2 || deltas[0] != "Hello" || deltas[1] != " world" {
		t.Errorf("unexpected deltas: %v", deltas)
	}
	if body["stream"] != true {
		t.Errorf("串流請求應帶 stream=true，實際 payload: %v", body)
	}
}

// TestChatStream_ToolCallAcrossChunks 驗證工具呼叫的參數由多個 chunk 拼回。
//
// 串流模式下 tool_calls 是分批送來的：第一個 chunk 帶 id 與名稱，後續只帶 arguments
// 的片段，必須全部拼回來才是合法的 JSON。
func TestChatStream_ToolCallAcrossChunks(t *testing.T) {
	server, _ := countingServer(t, func(w http.ResponseWriter, r *http.Request, _ int32) {
		writeSSE(w,
			`{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"read_file","arguments":"{\"pa"}}]}}]}`,
			`{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"th\":\"a.txt\"}"}}]}}]}`,
			`[DONE]`,
		)
	})

	p := newTestProvider(t, server.URL, 5*time.Second)

	resp, err := p.ChatStream(context.Background(), nil, nil, "m", nil, func(string) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}

	call := resp.ToolCalls[0]
	if call.ID != "call_1" || call.Function.Name != "read_file" {
		t.Errorf("unexpected tool call identity: %+v", call)
	}
	if call.Function.Arguments != `{"path":"a.txt"}` {
		t.Errorf("參數必須由分段拼回完整 JSON，得到 %q", call.Function.Arguments)
	}
}

// TestChatStream_MultipleToolCallsKeepIndexOrder 驗證多個工具呼叫依 index 排序，
// 不因送達順序而錯亂。
func TestChatStream_MultipleToolCallsKeepIndexOrder(t *testing.T) {
	server, _ := countingServer(t, func(w http.ResponseWriter, r *http.Request, _ int32) {
		writeSSE(w,
			`{"choices":[{"delta":{"tool_calls":[{"index":1,"id":"b","type":"function","function":{"name":"second","arguments":"{}"}}]}}]}`,
			`{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"a","type":"function","function":{"name":"first","arguments":"{}"}}]}}]}`,
			`[DONE]`,
		)
	})

	p := newTestProvider(t, server.URL, 5*time.Second)

	resp, err := p.ChatStream(context.Background(), nil, nil, "m", nil, func(string) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.ToolCalls) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Function.Name != "first" || resp.ToolCalls[1].Function.Name != "second" {
		t.Errorf("工具呼叫應依 index 排序，得到 %q, %q",
			resp.ToolCalls[0].Function.Name, resp.ToolCalls[1].Function.Name)
	}
}

// TestChatStream_IgnoresNoise 驗證註解、空行與無法解析的區塊會被略過。
func TestChatStream_IgnoresNoise(t *testing.T) {
	server, _ := countingServer(t, func(w http.ResponseWriter, r *http.Request, _ int32) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, ": keep-alive\n\n")
		fmt.Fprint(w, "data: not json\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	})

	p := newTestProvider(t, server.URL, 5*time.Second)

	resp, err := p.ChatStream(context.Background(), nil, nil, "m", nil, func(string) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "ok" {
		t.Errorf("expected content ok, got %q", resp.Content)
	}
}

// TestChatStream_ErrorChunk 驗證以 HTTP 200 回傳的錯誤區塊會被視為失敗。
func TestChatStream_ErrorChunk(t *testing.T) {
	server, attempts := countingServer(t, func(w http.ResponseWriter, r *http.Request, _ int32) {
		writeSSE(w, `{"error":{"message":"model overloaded"}}`)
	})

	p := newTestProvider(t, server.URL, 5*time.Second)

	_, err := p.ChatStream(context.Background(), nil, nil, "m", nil, func(string) {})
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "model overloaded") {
		t.Errorf("錯誤訊息應包含供應商的說明，得到 %v", err)
	}
	// 尚未輸出任何內容，因此允許重試
	if got := atomic.LoadInt32(attempts); got != int32(DefaultMaxAttempts) {
		t.Errorf("尚未輸出內容時應重試到上限，實際嘗試 %d 次", got)
	}
}

// TestChatStream_RetriesServerErrorBeforeOutput 驗證輸出前的一般失敗會重試。
func TestChatStream_RetriesServerErrorBeforeOutput(t *testing.T) {
	server, attempts := countingServer(t, func(w http.ResponseWriter, r *http.Request, attempt int32) {
		if attempt == 1 {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		writeSSE(w, `{"choices":[{"delta":{"content":"recovered"}}]}`, `[DONE]`)
	})

	p := newTestProvider(t, server.URL, 5*time.Second)

	resp, err := p.ChatStream(context.Background(), nil, nil, "m", nil, func(string) {})
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

// TestChatStream_DoesNotRetryAfterOutput 驗證一旦輸出過片段就不再重試。
//
// 否則使用者會看到重複的句子。
func TestChatStream_DoesNotRetryAfterOutput(t *testing.T) {
	server, attempts := countingServer(t, func(w http.ResponseWriter, r *http.Request, _ int32) {
		// 先送出一個片段，再中斷連線
		writeSSE(w, `{"choices":[{"delta":{"content":"partial"}}]}`)
		panic(http.ErrAbortHandler)
	})

	p := newTestProvider(t, server.URL, 5*time.Second)

	var deltas []string
	_, err := p.ChatStream(context.Background(), nil, nil, "m", nil, func(text string) {
		deltas = append(deltas, text)
	})

	if err == nil {
		t.Fatal("expected an error after the stream was interrupted")
	}
	if len(deltas) != 1 || deltas[0] != "partial" {
		t.Errorf("中斷前的片段應已送出，得到 %v", deltas)
	}
	if got := atomic.LoadInt32(attempts); got != 1 {
		t.Errorf("已輸出內容後不應重試，實際嘗試 %d 次", got)
	}
}

// TestChatStream_ContextCancelInterruptsStalledStream 驗證串流停滯時可以被取消。
func TestChatStream_ContextCancelInterruptsStalledStream(t *testing.T) {
	server, _ := countingServer(t, func(w http.ResponseWriter, r *http.Request, _ int32) {
		writeSSE(w, `{"choices":[{"delta":{"content":"first"}}]}`)

		// 之後不再送出資料；用戶端取消時請求的 context 也會結束，handler 隨之返回，
		// 否則 server.Close() 會一直等下去
		<-r.Context().Done()
	})

	p := newTestProvider(t, server.URL, 30*time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err := p.ChatStream(ctx, nil, nil, "m", nil, func(string) {})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected an error after cancellation")
	}
	if elapsed > 3*time.Second {
		t.Errorf("取消後應立即結束，實際耗時 %s", elapsed)
	}
}
