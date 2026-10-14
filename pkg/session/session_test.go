package session

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/chiisen/mini_bot/pkg/providers"
)

func TestManager_Load_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	messages, err := m.Load("nonexistent-session")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(messages) != 0 {
		t.Errorf("expected empty messages, got %d", len(messages))
	}
}

func TestManager_SaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	messages := []providers.Message{
		{Role: "system", Content: "You are a helpful assistant"},
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
	}

	sessionKey := "test-session"
	if err := m.Save(sessionKey, messages); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	loaded, err := m.Load(sessionKey)
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	if len(loaded) != len(messages) {
		t.Errorf("expected %d messages, got %d", len(messages), len(loaded))
	}

	for i, msg := range loaded {
		if msg.Role != messages[i].Role {
			t.Errorf("message[%d] role: expected %s, got %s", i, messages[i].Role, msg.Role)
		}
		if msg.Content != messages[i].Content {
			t.Errorf("message[%d] content: expected %s, got %s", i, messages[i].Content, msg.Content)
		}
	}
}

func TestManager_Save_Overwrites(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	sessionKey := "test-session"

	// Save first session
	messages1 := []providers.Message{
		{Role: "user", Content: "First message"},
	}
	if err := m.Save(sessionKey, messages1); err != nil {
		t.Fatalf("failed to save first: %v", err)
	}

	// Save second session
	messages2 := []providers.Message{
		{Role: "user", Content: "Second message"},
		{Role: "assistant", Content: "Response"},
	}
	if err := m.Save(sessionKey, messages2); err != nil {
		t.Fatalf("failed to save second: %v", err)
	}

	// Load should have second session
	loaded, err := m.Load(sessionKey)
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	if len(loaded) != 2 {
		t.Errorf("expected 2 messages, got %d", len(loaded))
	}

	if loaded[0].Content != "Second message" {
		t.Errorf("expected first content Second message, got %s", loaded[0].Content)
	}
}

func TestManager_Compress_NoChange(t *testing.T) {
	m := NewManager("/tmp")

	// Test with fewer than 12 messages - should not compress
	messages := []providers.Message{
		{Role: "system", Content: "System prompt"},
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi"},
	}

	compressed := m.Compress(messages, 1000)
	if len(compressed) != len(messages) {
		t.Errorf("expected %d messages, got %d", len(messages), len(compressed))
	}
}

func TestManager_Compress_Aggressive(t *testing.T) {
	m := NewManager("/tmp")

	// Create more than 12 messages
	messages := make([]providers.Message, 20)
	messages[0] = providers.Message{Role: "system", Content: "System prompt"}
	for i := 1; i < 20; i++ {
		messages[i] = providers.Message{Role: "user", Content: "Message"}
	}

	compressed := m.Compress(messages, 1000)

	// Should keep system message and last 10 messages = 11 total
	if len(compressed) != 11 {
		t.Errorf("expected 11 messages after compression, got %d", len(compressed))
	}

	// First should be system
	if compressed[0].Role != "system" {
		t.Errorf("expected first message to be system, got %s", compressed[0].Role)
	}

	// Last should be the last original message
	if compressed[len(compressed)-1].Content != "Message" {
		t.Errorf("expected last message to be Message, got %s", compressed[len(compressed)-1].Content)
	}
}

func TestManager_Compress_NoSystemMessage(t *testing.T) {
	m := NewManager("/tmp")

	// No system message
	messages := make([]providers.Message, 15)
	for i := 0; i < 15; i++ {
		messages[i] = providers.Message{Role: "user", Content: "Message " + string(rune('0'+i))}
	}

	compressed := m.Compress(messages, 1000)

	// Should keep last 10
	if len(compressed) != 10 {
		t.Errorf("expected 10 messages after compression, got %d", len(compressed))
	}
}

// TestManager_Save_CreatesStorageDir 驗證儲存目錄不存在時 Save 仍能成功。
//
// 只有走過 `app onboard` 的預設 workspace 才會有 sessions 目錄；以環境變數或
// config 指定其他 workspace 時，先前每次儲存都會以 no such file or directory
// 失敗——使用者會看到完整回覆，卻在最後收到錯誤且會話沒有被保存。
func TestManager_Save_CreatesStorageDir(t *testing.T) {
	nestedDir := filepath.Join(t.TempDir(), "nonexistent", "sessions")
	m := NewManager(nestedDir)

	messages := []providers.Message{{Role: "user", Content: "Hello"}}
	if err := m.Save("new-session", messages); err != nil {
		t.Fatalf("expected Save to create the storage dir, got error: %v", err)
	}

	loaded, err := m.Load("new-session")
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}
	if len(loaded) != 1 || loaded[0].Content != "Hello" {
		t.Errorf("unexpected loaded messages: %+v", loaded)
	}
}

// ============================================================================
// 工具呼叫邊界 (tool call boundary)
// ============================================================================

// assertNoOrphanToolMessages 驗證每個 tool 訊息都有對應的 assistant tool_call
// 出現在它之前。缺少配對時，OpenAI 相容 API 會以 400 拒絕整個請求。
func assertNoOrphanToolMessages(t *testing.T, messages []providers.Message) {
	t.Helper()

	pending := make(map[string]bool)
	for i, msg := range messages {
		switch msg.Role {
		case "assistant":
			for _, call := range msg.ToolCalls {
				pending[call.ID] = true
			}
		case "tool":
			if !pending[msg.ToolCallID] {
				t.Errorf("index %d: tool 訊息 %q 缺少對應的 assistant tool_call（孤兒 tool 訊息）", i, msg.ToolCallID)
			}
			delete(pending, msg.ToolCallID)
		}
	}
}

// historyWithToolLoop 產生一段歷史，其排列刻意讓「取最後 compressTailSize 則」
// 的裁切點剛好落在工具呼叫群組中間。
func historyWithToolLoop() []providers.Message {
	history := []providers.Message{
		{Role: "system", Content: "System prompt"},
		{Role: "user", Content: "start"},
	}

	for i := 1; i <= 6; i++ {
		id := fmt.Sprintf("call_%d", i)
		history = append(history,
			providers.Message{
				Role:      "assistant",
				ToolCalls: []providers.ToolCall{{ID: id, Type: "function"}},
			},
			providers.Message{Role: "tool", ToolCallID: id, Content: "result"},
		)
	}

	return append(history, providers.Message{Role: "assistant", Content: "done"})
}

// TestManager_Compress_AlignsToolBoundary 驗證裁切點不會落在工具呼叫群組中間。
//
// 修正前 Compress 以固定則數裁切，當裁切點剛好是 tool 訊息時，送給 LLM 的
// 序列會以孤兒 tool 訊息開頭，OpenAI 相容 API 直接回 400，長對話因此無法繼續。
func TestManager_Compress_AlignsToolBoundary(t *testing.T) {
	m := NewManager("/tmp")
	history := historyWithToolLoop()

	compressed := m.Compress(history, 1000)

	if compressed[0].Role != "system" {
		t.Errorf("expected first message to be system, got %s", compressed[0].Role)
	}
	if compressed[1].Role == "tool" {
		t.Fatal("壓縮後的序列以 tool 訊息開頭，這會讓 OpenAI 相容 API 回 400")
	}
	assertNoOrphanToolMessages(t, compressed)

	// 壓縮仍必須有意義：結果要比原始歷史短
	if len(compressed) >= len(history) {
		t.Errorf("expected compression to shrink the history: %d -> %d", len(history), len(compressed))
	}
}

// TestManager_Compress_ToolGroupLongerThanTail 驗證當尾段幾乎都屬於同一個工具
// 群組時，裁切點會一路往前推到發出這些呼叫的 assistant 訊息。
//
// 一個 assistant 訊息可以同時發出多個工具呼叫（平行工具呼叫），此時群組長度
// 可能超過 compressTailSize。這種情況下寧可整段保留、放棄壓縮，也不能切出
// 孤兒 tool 訊息。
func TestManager_Compress_ToolGroupLongerThanTail(t *testing.T) {
	m := NewManager("/tmp")

	messages := []providers.Message{
		{Role: "system", Content: "System prompt"},
		{Role: "user", Content: "start"},
	}

	calls := make([]providers.ToolCall, 0, 12)
	for i := 1; i <= 12; i++ {
		calls = append(calls, providers.ToolCall{ID: fmt.Sprintf("call_%d", i), Type: "function"})
	}
	messages = append(messages, providers.Message{Role: "assistant", ToolCalls: calls})
	for i := 1; i <= 12; i++ {
		messages = append(messages, providers.Message{
			Role:       "tool",
			ToolCallID: fmt.Sprintf("call_%d", i),
			Content:    "result",
		})
	}

	compressed := m.Compress(messages, 1000)

	if compressed[0].Role != "system" {
		t.Errorf("expected system message to be kept, got %s", compressed[0].Role)
	}
	if compressed[1].Role == "tool" {
		t.Fatal("壓縮後的序列以 tool 訊息開頭，這會讓 OpenAI 相容 API 回 400")
	}
	assertNoOrphanToolMessages(t, compressed)
}
