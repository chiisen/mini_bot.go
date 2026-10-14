package session

import (
	"fmt"
	"path/filepath"
	"strings"
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

// TestManager_Compress_KeepsManyShortMessages 驗證大量短訊息不會因為「則數多」被裁掉。
//
// 這是改用 token 預算的主要理由：先前的固定則數策略會丟掉這些便宜的上下文。
func TestManager_Compress_KeepsManyShortMessages(t *testing.T) {
	m := NewManager("/tmp")

	messages := []providers.Message{{Role: "system", Content: "System prompt"}}
	for i := 0; i < 100; i++ {
		messages = append(messages, providers.Message{Role: "user", Content: "hi"})
	}

	compressed := m.Compress(messages, DefaultContextBudgetTokens)

	if len(compressed) != len(messages) {
		t.Errorf("短訊息總量遠低於預算，不應被裁切：%d → %d", len(messages), len(compressed))
	}
}

// TestManager_Compress_TrimsWhenOverBudget 驗證超出預算時會裁掉中段並保留系統訊息
// 與最新的一則。
func TestManager_Compress_TrimsWhenOverBudget(t *testing.T) {
	m := NewManager("/tmp")

	messages := []providers.Message{{Role: "system", Content: "System prompt"}}
	for i := 0; i < 50; i++ {
		messages = append(messages, providers.Message{Role: "user", Content: strings.Repeat("x", 400)})
	}

	const budget = 500
	compressed := m.Compress(messages, budget)

	if compressed[0].Role != "system" {
		t.Errorf("系統提示詞必須保留，得到 %s", compressed[0].Role)
	}
	if len(compressed) >= len(messages) {
		t.Errorf("超出預算時應裁切：%d → %d", len(messages), len(compressed))
	}
	if len(compressed) < 2 {
		t.Fatalf("至少應保留系統提示詞與最新一則，得到 %d 則", len(compressed))
	}
	if got, want := compressed[len(compressed)-1].Content, messages[len(messages)-1].Content; got != want {
		t.Error("最新一則必須保留")
	}

	// 保留的內容應該大致落在預算內（允許對齊工具群組造成的少量超出）
	total := 0
	for _, msg := range compressed {
		total += estimateMessageTokens(msg)
	}
	if total > budget*2 {
		t.Errorf("裁切後仍遠超預算：%d tokens（預算 %d）", total, budget)
	}
}

// TestManager_Compress_KeepsLastMessageEvenIfOverBudget 驗證單則訊息就超出預算時，
// 仍會保留它——否則模型沒有東西可以回應。
func TestManager_Compress_KeepsLastMessageEvenIfOverBudget(t *testing.T) {
	m := NewManager("/tmp")
	huge := strings.Repeat("x", 100000)

	messages := []providers.Message{
		{Role: "system", Content: "System prompt"},
		{Role: "user", Content: "old question"},
		{Role: "assistant", Content: "old answer"},
		{Role: "user", Content: huge},
	}

	compressed := m.Compress(messages, 100)

	if len(compressed) != 2 {
		t.Fatalf("應只保留系統提示詞與最新一則，得到 %d 則", len(compressed))
	}
	if compressed[len(compressed)-1].Content != huge {
		t.Error("即使超出預算，最新一則仍必須保留")
	}
}

// TestManager_Compress_NoSystemMessage 驗證沒有系統訊息時仍會依預算裁切。
func TestManager_Compress_NoSystemMessage(t *testing.T) {
	m := NewManager("/tmp")

	messages := make([]providers.Message, 0, 15)
	for i := 0; i < 15; i++ {
		messages = append(messages, providers.Message{Role: "user", Content: strings.Repeat("x", 400)})
	}

	compressed := m.Compress(messages, 500)

	if len(compressed) >= len(messages) {
		t.Errorf("超出預算時應裁切：%d → %d", len(messages), len(compressed))
	}
	if got, want := compressed[len(compressed)-1].Content, messages[len(messages)-1].Content; got != want {
		t.Error("最新一則必須保留")
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

	// 用很小的預算強制裁切
	compressed := m.Compress(history, 60)

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

// TestEstimateTokens 驗證字元估算的規則與量級。
func TestEstimateTokens(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  int
	}{
		{"空字串", "", 0},
		{"純英文 400 字約 100 tokens", strings.Repeat("a", 400), 100},
		{"純中文 100 字約 100 tokens", strings.Repeat("中", 100), 100},
		{"混合", "hello 中文 world", 5},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := estimateTokens(tc.input); got != tc.want {
				t.Errorf("expected %d, got %d", tc.want, got)
			}
		})
	}
}

// TestEstimateMessageTokens_CountsToolCalls 驗證工具呼叫的名稱與參數也計入成本。
func TestEstimateMessageTokens_CountsToolCalls(t *testing.T) {
	plain := providers.Message{Role: "assistant", Content: "hi"}
	withCall := providers.Message{
		Role:    "assistant",
		Content: "hi",
		ToolCalls: []providers.ToolCall{
			{
				ID:       "c1",
				Type:     "function",
				Function: providers.FunctionCall{Name: "read_file", Arguments: `{"path":"a/b/c.txt"}`},
			},
		},
	}

	if estimateMessageTokens(withCall) <= estimateMessageTokens(plain) {
		t.Error("含工具呼叫的訊息成本應高於純文字訊息")
	}
}

// TestManager_Compress_ToolGroupKeptIntact 驗證當尾段幾乎都屬於同一個工具群組時，
// 裁切點會一路往前推到發出這些呼叫的 assistant 訊息。
//
// 一個 assistant 訊息可以同時發出多個工具呼叫（平行工具呼叫），此時群組長度可能
// 超過預算。這種情況下寧可超出預算，也不能切出孤兒 tool 訊息。
func TestManager_Compress_ToolGroupKeptIntact(t *testing.T) {
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

	// 預算只夠放其中一部分，迫使裁切點往前推到工具群組的起點
	compressed := m.Compress(messages, 150)

	if compressed[0].Role != "system" {
		t.Errorf("expected system message to be kept, got %s", compressed[0].Role)
	}
	if compressed[1].Role == "tool" {
		t.Fatal("壓縮後的序列以 tool 訊息開頭，這會讓 OpenAI 相容 API 回 400")
	}
	assertNoOrphanToolMessages(t, compressed)
}
