package bus

import (
	"context"
	"time"

	"github.com/chiisen/mini_bot/pkg/agent"
	"github.com/chiisen/mini_bot/pkg/logger"
)

// DefaultReplyTimeout 是投遞單則回覆的等待上限。
//
// 超過就放棄並記錄，避免某個沒有被排空的回覆通道讓整個 bus（包含其他頻道與其他
// 聊天室）停止處理。可由測試縮短。
const DefaultReplyTimeout = 10 * time.Second

// Reply 是 Agent 產生的一段輸出。
//
// Delta 為 true 時代表這是串流片段：接收端應累積後更新同一則訊息（Telegram 用
// editMessageText），而不是每收到一段就送一則新訊息。Delta 為 false 時是一則完整
// 訊息（回覆、工具通知、Notice）。
//
// 注意：串流片段遺失只會讓顯示的文字少一段，但完整訊息遺失會讓使用者完全看不到
// 回覆；因此頻道端必須能即時排空通道（見 telegram.go 的 listenForReplies）。
type Reply struct {
	Text  string
	Delta bool
}

// InboundMessage represents a standardized message arriving from any channel.
type InboundMessage struct {
	Channel    string     // "telegram" | "cli"
	ChatID     string     // Used for routing reply back
	Content    string     // Message content
	SessionKey string     // Session key for conversation context
	ReplyChan  chan Reply // Optional: channel to send replies directly back
}

type MessageBus struct {
	inbound chan InboundMessage
	agent   *agent.AgentInstance

	// replyTimeout 是投遞回覆的等待上限，可由測試縮短
	replyTimeout time.Duration
}

func New(a *agent.AgentInstance) *MessageBus {
	return &MessageBus{
		inbound:      make(chan InboundMessage, 100),
		agent:        a,
		replyTimeout: DefaultReplyTimeout,
	}
}

func (b *MessageBus) Send(msg InboundMessage) {
	b.inbound <- msg
}

// deliver 把一段輸出送進指定的通道。
//
// 這裡是 bus 唯一的阻塞點：先前直接以 `ch <- reply` 送出，只要該通道沒有被排空
// （例如通道另一端已結束，或 Telegram API 變慢），bus 的處理迴圈就會卡住，其他
// 頻道的訊息也跟著停止處理。現在改為有上限的等待，逾時就丟棄並記錄。
func (b *MessageBus) deliver(ctx context.Context, ch chan Reply, reply Reply) {
	if ch == nil {
		// 非同步頻道（例如 telegram）自行處理回覆，不經由這裡
		return
	}

	timer := time.NewTimer(b.replyTimeout)
	defer timer.Stop()

	select {
	case ch <- reply:
	case <-ctx.Done():
		logger.Warn("dropping reply: bus is shutting down", "reply_len", len(reply.Text))
	case <-timer.C:
		logger.Warn("dropping reply: reply channel is not being drained", "reply_len", len(reply.Text))
	}
}

func (b *MessageBus) Start(ctx context.Context) {
	logger.Info("MessageBus started")
	go func() {
		for {
			select {
			case <-ctx.Done():
				logger.Info("MessageBus shutting down")
				return
			case msg := <-b.inbound:
				// Process the message via Agent loop；支援串流的 provider 會逐段
				// 輸出（Delta），其餘仍是一次一個完整訊息
				err := b.agent.RunStream(ctx, msg.SessionKey, msg.Content,
					func(reply string) {
						b.deliver(ctx, msg.ReplyChan, Reply{Text: reply})
					},
					func(delta string) {
						b.deliver(ctx, msg.ReplyChan, Reply{Text: delta, Delta: true})
					})

				if err != nil {
					logger.Error("Agent Run failed", "error", err, "session", msg.SessionKey)
					b.deliver(ctx, msg.ReplyChan, Reply{Text: "Agent Encountered an Error: " + err.Error()})
				}

				// Signal completion if ReplyChan is provided
				if msg.ReplyChan != nil {
					close(msg.ReplyChan)
				}
			}
		}
	}()
}
