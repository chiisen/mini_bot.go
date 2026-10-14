package tools

import (
	"strings"
	"testing"
)

// ddgFixture 是一段模仿 DuckDuckGo HTML 結果頁的片段。
//
// 解析依賴對方固定的 class 名稱，用固定片段測試可以在對方改版時先讓 CI 失敗，
// 而不是讓使用者以為查詢沒有結果。
const ddgFixture = `
<div class="result results_links">
  <a class="result__url" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2Fdocs%3Fv%3D1&amp;rut=deadbeef">example.com/docs</a>
  <a class="result__snippet" href="https://example.com/docs">Don&#x27;t miss <b>this</b> &amp; that</a>
</div>
<div class="result results_links">
  <a class="result__url" href="https://golang.org/ref/spec">golang.org/ref/spec</a>
  <a class="result__snippet" href="https://golang.org/ref/spec">The Go Programming Language Specification</a>
</div>
`

// TestParseSearchResults 驗證結果解析、轉址還原與 HTML entity 解碼。
func TestParseSearchResults(t *testing.T) {
	results := parseSearchResults(ddgFixture)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	if got, want := results[0].URL, "https://example.com/docs?v=1"; got != want {
		t.Errorf("轉址應被還原且 entity 解碼：got %q, want %q", got, want)
	}
	if got, want := results[0].Snippet, "Don't miss this & that"; got != want {
		t.Errorf("snippet 應移除標籤並解碼 entity：got %q, want %q", got, want)
	}

	// 沒有 /l/?uddg= 轉址的連結應原樣保留
	if got, want := results[1].URL, "https://golang.org/ref/spec"; got != want {
		t.Errorf("直接連結不應被改動：got %q, want %q", got, want)
	}
}

// TestParseSearchResults_NoMatches 驗證無法解析時回傳空結果（不 panic）。
func TestParseSearchResults_NoMatches(t *testing.T) {
	for _, body := range []string{"", "<html><body>nothing here</body></html>"} {
		if got := parseSearchResults(body); len(got) != 0 {
			t.Errorf("expected no results for %q, got %d", body, len(got))
		}
	}
}

// TestParseSearchResults_Limit 驗證最多只取 5 筆。
func TestParseSearchResults_Limit(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 8; i++ {
		sb.WriteString(`<a class="result__url" href="https://example.com/` + string(rune('a'+i)) + `">x</a>`)
		sb.WriteString(`<a class="result__snippet">snip</a>`)
	}

	if got := parseSearchResults(sb.String()); len(got) != 5 {
		t.Errorf("expected at most 5 results, got %d", len(got))
	}
}

// TestParseSearchResults_InvalidEscape 驗證無法解碼的轉址不會讓連結消失。
func TestParseSearchResults_InvalidEscape(t *testing.T) {
	body := `<a class="result__url" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2Fbad%zz">x</a><a class="result__snippet">snip</a>`

	results := parseSearchResults(body)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].URL == "" {
		t.Error("解碼失敗時仍應保留原始連結，而不是變成空字串")
	}
}
