package tools

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/chiisen/mini_bot/pkg/i18n"
)

type WebSearchTool struct{}

func (t *WebSearchTool) Name() string        { return "web_search" }
func (t *WebSearchTool) Description() string { return i18n.GetInstance().T("tools.web_search") }
func (t *WebSearchTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{"type": "string", "description": "The search query"},
		},
		"required": []string{"query"},
	}
}

// Very basic HTML strip regex for DuckDuckGo
var ddgResultRegex = regexp.MustCompile(`(?s)<a class="result__url" href="([^"]+)".*?<a class="result__snippet[^>]+>(.*?)</a>`)
var tagRemover = regexp.MustCompile(`<[^>]*>`)

// searchResult 是單筆搜尋結果
type searchResult struct {
	URL     string
	Snippet string
}

// parseSearchResults 從 DuckDuckGo 的 HTML 結果頁取出前 5 筆結果。
//
// 解析依賴 DuckDuckGo 的 class 名稱，本來就容易因對方改版而失效，因此獨立成函式
// 以便用固定的 HTML 片段測試；否則一旦失效，使用者只會看到「沒有結果」。
func parseSearchResults(body string) []searchResult {
	matches := ddgResultRegex.FindAllStringSubmatch(body, 5)
	results := make([]searchResult, 0, len(matches))

	for _, match := range matches {
		if len(match) != 3 {
			continue
		}

		// 先解 HTML entity 再處理轉址：href 是 HTML 轉義過的，`&amp;` 要先還原成
		// `&` 才切得掉後面的 rut 參數
		link := html.UnescapeString(match[1])

		// DDG redirects
		if strings.HasPrefix(link, "//duckduckgo.com/l/?uddg=") {
			link = strings.TrimPrefix(link, "//duckduckgo.com/l/?uddg=")
			if unescaped, err := url.QueryUnescape(link); err == nil {
				link = unescaped
			}
			link = strings.Split(link, "&rut=")[0]
		}

		// snippet 同樣先移除標籤再解 entity：反過來做的話，被轉義的標記會先變成真的
		// 標籤而被整段移除，內容反而遺失
		snippet := html.UnescapeString(tagRemover.ReplaceAllString(match[2], ""))

		results = append(results, searchResult{
			URL:     link,
			Snippet: strings.TrimSpace(snippet),
		})
	}

	return results
}

func (t *WebSearchTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return &ToolResult{ForLLM: "Error: query is required", IsError: true}
	}

	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(query))

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return &ToolResult{ForLLM: fmt.Sprintf("Error creating request: %v", err), IsError: true}
	}

	// DDG HTML requires a user-agent to not block requests immediately
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/100.0.0.0 Safari/537.36")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return &ToolResult{ForLLM: fmt.Sprintf("Error fetching search results: %v", err), IsError: true}
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return &ToolResult{ForLLM: fmt.Sprintf("Failed to fetch results, HTTP status: %d", resp.StatusCode), IsError: true}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &ToolResult{ForLLM: fmt.Sprintf("Error reading response: %v", err), IsError: true}
	}

	results := parseSearchResults(string(body))
	if len(results) == 0 {
		// 無法區分「真的沒有結果」與「DuckDuckGo 改了 HTML 結構導致解析失敗」，
		// 因此訊息要同時涵蓋兩種可能，否則使用者會以為查詢真的沒有結果
		return &ToolResult{
			ForLLM:  "No results were parsed from DuckDuckGo: either the query has no matches, or the HTML layout changed and the parser needs updating.",
			IsError: false,
		}
	}

	var sb strings.Builder
	sb.WriteString("Web Search Results:\n")
	for i, result := range results {
		sb.WriteString(fmt.Sprintf("%d. URL: %s\n   Snippet: %s\n\n", i+1, result.URL, result.Snippet))
	}

	return &ToolResult{ForLLM: sb.String(), IsError: false}
}
