package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/chiisen/mini_bot/pkg/bus"
	"github.com/chiisen/mini_bot/pkg/config"
	"github.com/chiisen/mini_bot/pkg/logger"
)

type TelegramChannel struct {
	Token     string
	AllowFrom map[string]bool
	Bus       *bus.MessageBus
	client    *http.Client
}

func NewTelegramChannel(cfg *config.TelegramConfig, b *bus.MessageBus) *TelegramChannel {
	allowMap := make(map[string]bool)
	for _, id := range cfg.AllowFrom {
		allowMap[id] = true
	}

	return &TelegramChannel{
		Token:     cfg.Token,
		AllowFrom: allowMap,
		Bus:       b,
		client:    &http.Client{Timeout: 60 * time.Second}, // Longer timeout for long polling
	}
}

func (t *TelegramChannel) Start(ctx context.Context) error {
	logger.Info("Starting Telegram Channel (Long Polling)...")

	offset := 0

	for {
		select {
		case <-ctx.Done():
			logger.Info("Telegram Channel shutting down")
			return nil
		default:
		}

		// Perform long polling update request
		updates, err := t.getUpdates(ctx, offset)
		if err != nil {
			logger.Error("Telegram getUpdates failed", "error", err)
			time.Sleep(5 * time.Second) // backoff on error
			continue
		}

		for _, update := range updates {
			// Update offset to acknowledge receipt
			if update.UpdateID >= offset {
				offset = update.UpdateID + 1
			}

			// We only process valid text messages
			if update.Message == nil || update.Message.Text == "" {
				continue
			}

			userIDStr := strconv.FormatInt(update.Message.From.ID, 10)
			
			// T7-2: whitelist check
			if len(t.AllowFrom) > 0 && !t.AllowFrom[userIDStr] {
				logger.Warn("Ignored message from unauthorized user", "user_id", userIDStr)
				continue
			}

			logger.Debug("Received message from Telegram", "user_id", userIDStr, "text", update.Message.Text)

			chatIDStr := strconv.FormatInt(update.Message.Chat.ID, 10)
			sessionKey := "telegram_" + chatIDStr

			replyChan := make(chan string, 5)

			// Send to Bus
			t.Bus.Send(bus.InboundMessage{
				Channel:    "telegram",
				ChatID:     chatIDStr,
				Content:    update.Message.Text,
				SessionKey: sessionKey,
				ReplyChan:  replyChan,
			})

			// Handle replies asynchronously directly in the loop or spawn a worker
			// Since MVP, we spawn a simple goroutine to listen to the reply and send back
			go t.listenForReplies(ctx, chatIDStr, replyChan)
		}
	}
}

func (t *TelegramChannel) listenForReplies(ctx context.Context, chatID string, replyChan <-chan string) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-replyChan:
			if !ok {
				// channel closed, agent is done processing this message interaction
				return
			}
			if msg != "" {
				if err := t.SendMessage(ctx, chatID, msg); err != nil {
					logger.Error("Failed to send message to Telegram", "chat_id", chatID, "error", err)
				}
			}
		}
	}
}

func (t *TelegramChannel) SendMessage(ctx context.Context, chatID string, text string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.Token)

	payload := map[string]any{
		"chat_id": chatID,
		"text":    text,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status: %d body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Structs for parsing Telegram API
type tgUpdateResponse struct {
	Ok     bool       `json:"ok"`
	Result []tgUpdate `json:"result"`
}

type tgUpdate struct {
	UpdateID int        `json:"update_id"`
	Message  *tgMessage `json:"message,omitempty"`
}

type tgMessage struct {
	MessageID int    `json:"message_id"`
	From      tgUser `json:"from"`
	Chat      tgChat `json:"chat"`
	Date      int    `json:"date"`
	Text      string `json:"text"`
}

type tgUser struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	Username  string `json:"username,omitempty"`
}

type tgChat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}

func (t *TelegramChannel) getUpdates(ctx context.Context, offset int) ([]tgUpdate, error) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?offset=%d&timeout=30", t.Token, offset)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status code: %d", resp.StatusCode)
	}

	var data tgUpdateResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	if !data.Ok {
		return nil, fmt.Errorf("telegram returned ok: false")
	}

	return data.Result, nil
}
