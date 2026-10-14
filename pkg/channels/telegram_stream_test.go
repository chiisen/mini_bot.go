package channels

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chiisen/mini_bot/pkg/bus"
	"github.com/chiisen/mini_bot/pkg/config"
)

// tgCall 記錄一次 Telegram API 呼叫
type tgCall struct {
	Method    string
	Text      string
	MessageID int
	At        time.Time
}

// fakeTelegramAPI 是假的 Telegram Bot API
type fakeTelegramAPI struct {
	mu        sync.Mutex
	calls     []tgCall
	nextID    int
	failEdits bool // 讓 editMessageText 一律失敗
	rateLimit bool // 讓第一次 editMessageText 回 429（帶 retry_after 1 秒）
	retried   bool
}

func newFakeTelegramAPI(t *testing.T) (*fakeTelegramAPI, *TelegramChannel) {
	t.Helper()

	fake := &fakeTelegramAPI{nextID: 100}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)

		method := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]

		fake.mu.Lock()
		call := tgCall{Method: method, At: time.Now()}
		if text, ok := payload["text"].(string); ok {
			call.Text = text
		}
		if id, ok := payload["message_id"].(float64); ok {
			call.MessageID = int(id)
		}
		fake.calls = append(fake.calls, call)
		fake.nextID++
		nextID := fake.nextID

		respondRateLimit := method == "editMessageText" && fake.rateLimit && !fake.retried
		if respondRateLimit {
			fake.retried = true
		}
		failEdit := method == "editMessageText" && fake.failEdits
		fake.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")

		switch {
		case respondRateLimit:
			// 真實的 Telegram 會以 HTTP 429 回應
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"ok":false,"error_code":429,"description":"Too Many Requests: retry after 1","parameters":{"retry_after":1}}`)
		case failEdit:
			fmt.Fprint(w, `{"ok":false,"error_code":400,"description":"Bad Request: message to edit not found"}`)
		case method == "sendMessage":
			fmt.Fprintf(w, `{"ok":true,"result":{"message_id":%d}}`, nextID)
		default:
			fmt.Fprint(w, `{"ok":true,"result":{}}`)
		}
	}))
	t.Cleanup(server.Close)

	ch := NewTelegramChannel(&config.TelegramConfig{Token: "test-token"}, bus.New(nil), "")
	ch.apiBase = server.URL
	ch.editInterval = 10 * time.Millisecond

	return fake, ch
}

func (f *fakeTelegramAPI) snapshot() []tgCall {
	f.mu.Lock()
	defer f.mu.Unlock()

	out := make([]tgCall, len(f.calls))
	copy(out, f.calls)
	return out
}

// TestListenForReplies_SendsCompleteMessages 驗證完整訊息直接送出。
func TestListenForReplies_SendsCompleteMessages(t *testing.T) {
	fake, ch := newFakeTelegramAPI(t)

	replies := make(chan bus.Reply, 8)
	done := make(chan struct{})
	go func() {
		ch.listenForReplies(context.Background(), "123", replies)
		close(done)
	}()

	replies <- bus.Reply{Text: "[Agent uses tool: read_file...]"}
	replies <- bus.Reply{Text: "完成了"}
	close(replies)
	<-done

	calls := fake.snapshot()
	if len(calls) != 2 {
		t.Fatalf("expected 2 API calls, got %d: %+v", len(calls), calls)
	}
	for i, want := range []string{"[Agent uses tool: read_file...]", "完成了"} {
		if calls[i].Method != "sendMessage" || calls[i].Text != want {
			t.Errorf("call %d: expected sendMessage %q, got %+v", i, want, calls[i])
		}
	}
}

// TestListenForReplies_AccumulatesStreamAndUpdatesOnce 驗證大量片段只會產生少數幾次 API
// 呼叫，且最後一次帶有完整內容。
//
// 每個片段都更新會被 Telegram 以 429 拒絕，因此必須累積後間隔更新。
func TestListenForReplies_AccumulatesStreamAndUpdatesOnce(t *testing.T) {
	fake, ch := newFakeTelegramAPI(t)

	replies := make(chan bus.Reply, 128)
	done := make(chan struct{})
	go func() {
		ch.listenForReplies(context.Background(), "123", replies)
		close(done)
	}()

	start := time.Now()
	for i := 0; i < 100; i++ {
		replies <- bus.Reply{Text: "字", Delta: true}
	}
	close(replies)
	<-done
	elapsed := time.Since(start)

	calls := fake.snapshot()

	// 呼叫次數必須受間隔限制，而不是與片段數成正比
	maxAllowed := int(elapsed/ch.editInterval) + 2
	if len(calls) > maxAllowed {
		t.Errorf("%d 個片段產生了 %d 次 API 呼叫（上限 %d）", 100, len(calls), maxAllowed)
	}

	last := calls[len(calls)-1]
	if got := len([]rune(last.Text)); got != 100 {
		t.Errorf("最後一次更新應包含完整內容（100 字），得到 %d 字", got)
	}
}

// TestListenForReplies_RespectsRateLimit 驗證收到 429 後會依 retry_after 暫停更新。
func TestListenForReplies_RespectsRateLimit(t *testing.T) {
	fake, ch := newFakeTelegramAPI(t)
	fake.rateLimit = true

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	replies := make(chan bus.Reply, 128)
	done := make(chan struct{})
	go func() {
		ch.listenForReplies(ctx, "123", replies)
		close(done)
	}()

	// 持續送入片段，讓 ticker 有機會嘗試更新
	go func() {
		for i := 0; i < 60; i++ {
			select {
			case replies <- bus.Reply{Text: "x", Delta: true}:
			case <-ctx.Done():
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
		close(replies)
	}()

	<-done

	calls := fake.snapshot()

	// 找出第一次編輯（被 429 拒絕）與其後第一次成功的編輯，量測兩者間隔
	var firstEdit, nextEdit *tgCall
	for i := range calls {
		if calls[i].Method != "editMessageText" {
			continue
		}
		if firstEdit == nil {
			firstEdit = &calls[i]
			continue
		}
		nextEdit = &calls[i]
		break
	}
	if firstEdit == nil || nextEdit == nil {
		t.Fatalf("expected at least two edit attempts, got %+v", calls)
	}

	if gap := nextEdit.At.Sub(firstEdit.At); gap < 900*time.Millisecond {
		t.Errorf("429 之後應暫停約 1 秒（retry_after），實際只隔了 %s", gap)
	}
}

// TestListenForReplies_FallsBackWhenEditFails 驗證編輯失敗時退回送一則完整的訊息。
func TestListenForReplies_FallsBackWhenEditFails(t *testing.T) {
	fake, ch := newFakeTelegramAPI(t)
	fake.failEdits = true

	replies := make(chan bus.Reply, 32)
	done := make(chan struct{})
	go func() {
		ch.listenForReplies(context.Background(), "123", replies)
		close(done)
	}()

	replies <- bus.Reply{Text: "前半", Delta: true}
	time.Sleep(50 * time.Millisecond) // 讓第一次更新真的送出訊息
	replies <- bus.Reply{Text: "後半", Delta: true}
	time.Sleep(50 * time.Millisecond)
	close(replies)
	<-done

	calls := fake.snapshot()
	last := calls[len(calls)-1]

	if last.Method != "sendMessage" {
		t.Errorf("編輯失敗時應退回 sendMessage，得到 %s", last.Method)
	}
	if last.Text != "前半後半" {
		t.Errorf("退回的訊息應包含完整內容，得到 %q", last.Text)
	}
}

// TestListenForReplies_SendsTailForLongMessages 驗證超過長度上限的內容會補送。
func TestListenForReplies_SendsTailForLongMessages(t *testing.T) {
	fake, ch := newFakeTelegramAPI(t)

	replies := make(chan bus.Reply, 8)
	done := make(chan struct{})
	go func() {
		ch.listenForReplies(context.Background(), "123", replies)
		close(done)
	}()

	replies <- bus.Reply{Text: strings.Repeat("長", maxMessageRunes+250), Delta: true}
	close(replies)
	<-done

	calls := fake.snapshot()
	if len(calls) != 2 {
		t.Fatalf("expected 2 API calls (head + tail), got %d", len(calls))
	}
	if got := len([]rune(calls[0].Text)); got != maxMessageRunes {
		t.Errorf("第一則應為上限長度 %d，得到 %d", maxMessageRunes, got)
	}
	if got := len([]rune(calls[1].Text)); got != 250 {
		t.Errorf("第二則應為剩下的 250 字，得到 %d", got)
	}
}

// TestSplitTelegramMessage 驗證長度切分以 rune 為單位。
func TestSplitTelegramMessage(t *testing.T) {
	short := strings.Repeat("a", 10)
	if head, tail := splitTelegramMessage(short); head != short || tail != "" {
		t.Errorf("短訊息不應被切分：head=%q tail=%q", head, tail)
	}

	long := strings.Repeat("中", maxMessageRunes+1)
	head, tail := splitTelegramMessage(long)
	if len([]rune(head)) != maxMessageRunes {
		t.Errorf("head 長度錯誤：%d", len([]rune(head)))
	}
	if len([]rune(tail)) != 1 {
		t.Errorf("tail 長度錯誤：%d", len([]rune(tail)))
	}
	if head+tail != long {
		t.Error("切分後接回去應等於原文")
	}
}
