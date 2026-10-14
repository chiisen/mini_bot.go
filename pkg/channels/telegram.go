package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/chiisen/mini_bot/pkg/bus"
	"github.com/chiisen/mini_bot/pkg/config"
	"github.com/chiisen/mini_bot/pkg/logger"
)

const (
	// defaultTelegramAPIBase 是 Telegram Bot API 的位址（測試會覆寫）
	defaultTelegramAPIBase = "https://api.telegram.org"

	// defaultEditInterval 是逐步輸出時更新訊息的間隔。
	// Telegram 對編輯有速率限制（同一聊天室約每秒 1 次），更新太快會收到 429
	defaultEditInterval = time.Second

	// maxMessageRunes 是單則訊息的長度上限（Telegram 限制 4096 字元，留一點餘裕）
	maxMessageRunes = 4000

	// defaultRateLimitBackoff 是收到 429 但沒有 retry_after 時的退避時間
	defaultRateLimitBackoff = 5 * time.Second

	// replyBufferSize 是每個訊息的回覆通道緩衝；片段來得快，緩衝太小會讓 bus 等待
	replyBufferSize = 32
)

// telegramError 是 Telegram API 回傳的錯誤。
type telegramError struct {
	status     int
	errorCode  int
	body       string
	retryAfter time.Duration
}

func (e *telegramError) Error() string {
	return fmt.Sprintf("telegram api error (status %d): %s", e.status, e.body)
}

// rateLimited 回報是否為速率限制。
//
// 真實的 Telegram 會以 HTTP 429 回應，但有些代理或相容服務會把狀態碼換成 200 而只在
// JSON 的 error_code 標示，因此兩者都看。
func (e *telegramError) rateLimited() bool {
	return e.status == http.StatusTooManyRequests || e.errorCode == http.StatusTooManyRequests
}

type TelegramChannel struct {
	Token     string
	AllowFrom map[string]bool
	Bus       *bus.MessageBus
	client    *http.Client

	// offsetPath 是長輪詢 offset 的持久化位置；空字串代表不持久化
	offsetPath string

	// apiBase 與 editInterval 可由測試覆寫
	apiBase      string
	editInterval time.Duration
}

func NewTelegramChannel(cfg *config.TelegramConfig, b *bus.MessageBus, offsetPath string) *TelegramChannel {
	allowMap := make(map[string]bool)
	for _, id := range cfg.AllowFrom {
		allowMap[id] = true
	}

	return &TelegramChannel{
		Token:        cfg.Token,
		AllowFrom:    allowMap,
		Bus:          b,
		client:       &http.Client{Timeout: 60 * time.Second}, // Longer timeout for long polling
		offsetPath:   offsetPath,
		apiBase:      defaultTelegramAPIBase,
		editInterval: defaultEditInterval,
	}
}

// loadOffset 讀回上次的長輪詢 offset。
//
// Telegram 要等到下一次 getUpdates 帶入更大的 offset 才會確認前一批更新，因此
// 行程若在兩次輪詢之間結束，那些更新會重新送達並被當成新訊息處理（使用者會收到
// 重複回覆）。把 offset 持久化可以把這個視窗縮到最小。
func (t *TelegramChannel) loadOffset() int {
	if t.offsetPath == "" {
		return 0
	}

	data, err := os.ReadFile(t.offsetPath)
	if err != nil {
		return 0
	}

	offset, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || offset < 0 {
		return 0
	}

	return offset
}

// saveOffset 寫入目前的 offset；失敗只記錄，不中斷輪詢。
func (t *TelegramChannel) saveOffset(offset int) {
	if t.offsetPath == "" {
		return
	}

	if err := os.MkdirAll(filepath.Dir(t.offsetPath), 0755); err != nil {
		logger.Error("failed to create telegram offset directory", "error", err)
		return
	}

	if err := os.WriteFile(t.offsetPath, []byte(strconv.Itoa(offset)), 0600); err != nil {
		logger.Error("failed to persist telegram offset", "error", err)
	}
}

func (t *TelegramChannel) Start(ctx context.Context) error {
	logger.Info("Starting Telegram Channel (Long Polling)...")

	offset := t.loadOffset()

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

		previousOffset := offset

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

			replyChan := make(chan bus.Reply, replyBufferSize)

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

		// 整批都送進 bus 之後才確認 offset：避免「已確認但從未被處理」的靜默遺失
		// （取捨是崩潰時可能重複處理這一批，但這段視窗遠小於完全不持久化）
		if offset != previousOffset {
			t.saveOffset(offset)
		}
	}
}

// listenForReplies 接收 agent 的輸出並送回 Telegram。
//
// 完整訊息直接送出；串流片段則累積起來，以固定間隔更新同一則訊息（editMessageText），
// 讓長回覆不會只剩「沒有反應」的畫面。間隔是必要的：Telegram 對編輯有速率限制，
// 每個片段都更新會立刻收到 429。
func (t *TelegramChannel) listenForReplies(ctx context.Context, chatID string, replies <-chan bus.Reply) {
	var (
		buf       []rune    // 目前訊息已累積的文字
		messageID int       // 已送出的訊息 ID；0 表示尚未送出
		sentText  string    // 最後一次送出的內容，避免無謂的編輯
		throttled time.Time // 429 退避的結束時間
	)

	interval := t.editInterval
	if interval <= 0 {
		interval = defaultEditInterval
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case reply, ok := <-replies:
			if !ok {
				// agent 結束這次互動：做最後一次更新
				t.finishStream(ctx, chatID, &messageID, buf, &sentText)
				return
			}

			if reply.Delta {
				buf = append(buf, []rune(reply.Text)...)
				continue
			}

			// 完整訊息（工具通知、Notice、非串流回覆）：先讓串流中的訊息收尾
			t.finishStream(ctx, chatID, &messageID, buf, &sentText)
			buf = nil

			if reply.Text != "" {
				if err := t.SendMessage(ctx, chatID, reply.Text); err != nil {
					logger.Error("Failed to send message to Telegram", "chat_id", chatID, "error", err)
				}
			}

		case <-ticker.C:
			if time.Now().Before(throttled) {
				continue
			}

			err := t.updateStreamMessage(ctx, chatID, &messageID, buf, &sentText)
			if err == nil {
				continue
			}

			var apiErr *telegramError
			if errors.As(err, &apiErr) && apiErr.rateLimited() {
				backoff := apiErr.retryAfter
				if backoff <= 0 {
					backoff = defaultRateLimitBackoff
				}
				throttled = time.Now().Add(backoff)
				logger.Warn("Telegram rate limited, pausing stream updates", "chat_id", chatID, "backoff", backoff)
				continue
			}

			logger.Warn("Failed to update Telegram stream message", "chat_id", chatID, "error", err)
		}
	}
}

// updateStreamMessage 依目前累積的內容更新（或首次送出）訊息。
func (t *TelegramChannel) updateStreamMessage(ctx context.Context, chatID string, messageID *int, buf []rune, sentText *string) error {
	if len(buf) == 0 {
		return nil
	}

	head, _ := splitTelegramMessage(string(buf))
	if head == *sentText {
		return nil
	}

	if *messageID == 0 {
		id, err := t.sendMessageReturningID(ctx, chatID, head)
		if err != nil {
			return err
		}
		*messageID = id
	} else if err := t.editMessageText(ctx, chatID, *messageID, head); err != nil {
		return err
	}

	*sentText = head
	return nil
}

// finishStream 在訊息結束時做最後一次更新，並補送超出長度上限的部分。
//
// 編輯失敗（訊息被刪除、太舊等）時退回直接送一則完整訊息，避免內容遺失。
func (t *TelegramChannel) finishStream(ctx context.Context, chatID string, messageID *int, buf []rune, sentText *string) {
	if len(buf) == 0 {
		return
	}

	head, tail := splitTelegramMessage(string(buf))

	switch {
	case *messageID == 0:
		if err := t.SendMessage(ctx, chatID, head); err != nil {
			logger.Error("Failed to send message to Telegram", "chat_id", chatID, "error", err)
		}
	case head != *sentText:
		if err := t.editMessageText(ctx, chatID, *messageID, head); err != nil {
			logger.Warn("Failed to finalize streamed message, sending a full one instead",
				"chat_id", chatID, "error", err)
			if err := t.SendMessage(ctx, chatID, head); err != nil {
				logger.Error("Failed to send message to Telegram", "chat_id", chatID, "error", err)
			}
		}
	}

	if tail != "" {
		if err := t.SendMessage(ctx, chatID, tail); err != nil {
			logger.Error("Failed to send message tail to Telegram", "chat_id", chatID, "error", err)
		}
	}

	*messageID = 0
	*sentText = ""
}

// splitTelegramMessage 依 Telegram 的長度上限切成兩段（第二段可能為空）。
//
// 逐步輸出時先顯示前段，超過上限的部分在最後補送；以 rune 為單位才不會切斷中文。
func splitTelegramMessage(text string) (head, tail string) {
	runes := []rune(text)
	if len(runes) <= maxMessageRunes {
		return text, ""
	}

	return string(runes[:maxMessageRunes]), string(runes[maxMessageRunes:])
}

func (t *TelegramChannel) SendMessage(ctx context.Context, chatID string, text string) error {
	_, err := t.post(ctx, "sendMessage", map[string]any{
		"chat_id": chatID,
		"text":    text,
	})
	return err
}

// sendMessageReturningID 送出一則訊息並回傳它的 message_id（供後續編輯使用）。
func (t *TelegramChannel) sendMessageReturningID(ctx context.Context, chatID, text string) (int, error) {
	result, err := t.post(ctx, "sendMessage", map[string]any{
		"chat_id": chatID,
		"text":    text,
	})
	if err != nil {
		return 0, err
	}

	var sent struct {
		MessageID int `json:"message_id"`
	}
	if err := json.Unmarshal(result, &sent); err != nil {
		return 0, fmt.Errorf("failed to parse sendMessage result: %w", err)
	}

	return sent.MessageID, nil
}

// editMessageText 更新已送出訊息的內容。
func (t *TelegramChannel) editMessageText(ctx context.Context, chatID string, messageID int, text string) error {
	_, err := t.post(ctx, "editMessageText", map[string]any{
		"chat_id":    chatID,
		"message_id": messageID,
		"text":       text,
	})
	return err
}

// tgAPIResponse 是 Telegram Bot API 的統一回應格式
type tgAPIResponse struct {
	Ok          bool            `json:"ok"`
	ErrorCode   int             `json:"error_code"`
	Description string          `json:"description"`
	Result      json.RawMessage `json:"result"`
	Parameters  *struct {
		RetryAfter int `json:"retry_after"`
	} `json:"parameters"`
}

// post 呼叫 Telegram Bot API 並回傳回應中的 result。
//
// 失敗時回傳 *telegramError，讓呼叫端能分辨 429 並依 retry_after 退避。
func (t *TelegramChannel) post(ctx context.Context, method string, payload map[string]any) (json.RawMessage, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal %s payload: %w", method, err)
	}

	apiBase := t.apiBase
	if apiBase == "" {
		apiBase = defaultTelegramAPIBase
	}
	url := fmt.Sprintf("%s/bot%s/%s", apiBase, t.Token, method)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create %s request: %w", method, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s request failed: %w", method, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s response: %w", method, err)
	}

	var apiResp tgAPIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, &telegramError{status: resp.StatusCode, body: string(body)}
	}

	if !apiResp.Ok {
		// 內容與現有訊息相同時 Telegram 會回 400；這不是真的失敗
		if strings.Contains(apiResp.Description, "message is not modified") {
			return nil, nil
		}

		apiErr := &telegramError{status: resp.StatusCode, errorCode: apiResp.ErrorCode, body: apiResp.Description}
		if apiResp.Parameters != nil && apiResp.Parameters.RetryAfter > 0 {
			apiErr.retryAfter = time.Duration(apiResp.Parameters.RetryAfter) * time.Second
		}
		return nil, apiErr
	}

	return apiResp.Result, nil
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
