package bus

import (
	"context"

	"github.com/chiisen/mini_bot/pkg/agent"
	"github.com/chiisen/mini_bot/pkg/logger"
)

// InboundMessage represents a standardized message arriving from any channel.
type InboundMessage struct {
	Channel    string // "telegram" | "cli"
	ChatID     string // Used for routing reply back
	Content    string // Message content
	SessionKey string // Session key for conversation context
	ReplyChan  chan string // Optional: channel to send synchronous replies directly back
}

type MessageBus struct {
	inbound chan InboundMessage
	agent   *agent.AgentInstance
}

func New(a *agent.AgentInstance) *MessageBus {
	return &MessageBus{
		inbound: make(chan InboundMessage, 100),
		agent:   a,
	}
}

func (b *MessageBus) Send(msg InboundMessage) {
	b.inbound <- msg
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
				// Process the message via Agent loop
				err := b.agent.Run(ctx, msg.SessionKey, msg.Content, func(reply string) {
					// Route the reply back to the appropriate channel
					if msg.ReplyChan != nil {
						msg.ReplyChan <- reply
					} else {
						// For asynchronous channels (like telegram), we will handle it inside the specific channel adapter.
						// The MVP architecture delegates telegram replies to the telegram package listening to an outbound bus or via callbacks.
						// To keep it simple, we use ReplyChan or callbacks.
					}
				})
				
				if err != nil {
					logger.Error("Agent Run failed", "error", err, "session", msg.SessionKey)
					if msg.ReplyChan != nil {
						msg.ReplyChan <- "Agent Encountered an Error: " + err.Error()
					}
				}
				
				// Signal completion if ReplyChan is provided
				if msg.ReplyChan != nil {
					close(msg.ReplyChan)
				}
			}
		}
	}()
}
