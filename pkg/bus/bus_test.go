package bus

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chiisen/mini_bot/pkg/agent"
	"github.com/chiisen/mini_bot/pkg/config"
	"github.com/chiisen/mini_bot/pkg/providers"
	"github.com/chiisen/mini_bot/pkg/session"
	"github.com/chiisen/mini_bot/pkg/tools"
)

// alwaysToolProvider 每次都要求工具呼叫，讓 Run 產生兩則回覆（工具使用通知加上
// 達到迭代上限的通知），足以填滿容量 1 的回覆通道。
type alwaysToolProvider struct{}

func (p *alwaysToolProvider) Chat(
	ctx context.Context,
	messages []providers.Message,
	defs []providers.ToolDefinition,
	model string,
	options map[string]any,
) (*providers.LLMResponse, error) {
	return &providers.LLMResponse{
		ToolCalls: []providers.ToolCall{
			{ID: "call_1", Type: "function", Function: providers.FunctionCall{Name: "noop", Arguments: "{}"}},
		},
	}, nil
}

func (p *alwaysToolProvider) GetDefaultModel() string { return "fake" }

type noopTool struct{}

func (t *noopTool) Name() string               { return "noop" }
func (t *noopTool) Description() string        { return "noop tool for tests" }
func (t *noopTool) Parameters() map[string]any { return map[string]any{"type": "object"} }
func (t *noopTool) Execute(ctx context.Context, args map[string]any) *tools.ToolResult {
	return &tools.ToolResult{ForLLM: "ok"}
}

// newTestBus 建立一個不碰網路、不讀真實設定的 MessageBus，並縮短投遞等待時間。
func newTestBus(t *testing.T) *MessageBus {
	t.Helper()

	workspace := t.TempDir()
	sessionsDir := filepath.Join(workspace, "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		t.Fatalf("failed to create sessions dir: %v", err)
	}

	cfg := &config.Config{}
	cfg.Agents.Defaults.Model = "fake/model"
	cfg.Agents.Defaults.MaxTokens = 128
	cfg.Agents.Defaults.MaxToolIterations = 1

	registry := tools.NewRegistry()
	registry.Register(&noopTool{})

	instance := &agent.AgentInstance{
		Config:       cfg,
		Provider:     &alwaysToolProvider{},
		Registry:     registry,
		Sessions:     session.NewManager(sessionsDir),
		CtxBuilder:   agent.NewContextBuilder(workspace),
		WorkspaceDir: workspace,
	}

	b := New(instance)
	b.replyTimeout = 50 * time.Millisecond
	return b
}

// TestBus_StalledReplyChannelDoesNotBlockOtherChats 驗證某個沒有被排空的回覆通道
// 不會讓整個 bus 停止處理其他聊天室的訊息。
//
// 先前 bus 以同步方式送出回覆，只要通道沒被排空就卡在該次送出；實測下一個訊息
// 永遠不會被處理（見 issue #26）。
func TestBus_StalledReplyChannelDoesNotBlockOtherChats(t *testing.T) {
	b := newTestBus(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	b.Start(ctx)

	// 第一個訊息：容量 1 且沒有人讀取，而 Run 會產生兩則回覆
	stalled := make(chan string, 1)
	b.Send(InboundMessage{Channel: "telegram", ChatID: "1", Content: "first", SessionKey: "s1", ReplyChan: stalled})

	// 第二個訊息：不同聊天室、正常的通道
	ok := make(chan string, 10)
	b.Send(InboundMessage{Channel: "telegram", ChatID: "2", Content: "second", SessionKey: "s2", ReplyChan: ok})

	select {
	case reply := <-ok:
		if reply == "" {
			t.Error("expected a non-empty reply")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("第二個聊天室的訊息沒有被處理：bus 被前一個未排空的通道卡住")
	}
}

// TestBus_DeliverWithoutReplyChan 驗證沒有回覆通道時不阻塞也不 panic。
func TestBus_DeliverWithoutReplyChan(t *testing.T) {
	b := newTestBus(t)
	b.deliver(context.Background(), nil, "reply")
}

// TestBus_DeliverStopsWhenContextCancelled 驗證 bus 關閉時不會卡在投遞。
func TestBus_DeliverStopsWhenContextCancelled(t *testing.T) {
	b := newTestBus(t)
	b.replyTimeout = time.Minute

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ch := make(chan string, 1)
	ch <- "filled" // 讓通道滿載，投遞只能靠 context 結束收尾

	done := make(chan struct{})
	go func() {
		b.deliver(ctx, ch, "reply")
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("context 已取消時 deliver 仍卡住")
	}
}
