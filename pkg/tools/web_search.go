package tools

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type WebSearchTool struct{}

func (t *WebSearchTool) Name() string { return "web_search" }
func (t *WebSearchTool) Description() string { return "Search the web using DuckDuckGo HTML edition" }
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

	bodyStr := string(body)
	matches := ddgResultRegex.FindAllStringSubmatch(bodyStr, 5) // Get top 5 results

	if len(matches) == 0 {
		return &ToolResult{ForLLM: "No results found or parser failed.", IsError: false}
	}

	var sb strings.Builder
	sb.WriteString("Web Search Results:\n")
	for i, match := range matches {
		if len(match) == 3 {
			link := match[1]
			snippet := tagRemover.ReplaceAllString(match[2], "")
			
			// DDG redirects
			if strings.HasPrefix(link, "//duckduckgo.com/l/?uddg=") {
				link = strings.TrimPrefix(link, "//duckduckgo.com/l/?uddg=")
				link, _ = url.QueryUnescape(link)
				link = strings.Split(link, "&rut=")[0]
			}

			sb.WriteString(fmt.Sprintf("%d. URL: %s\n   Snippet: %s\n\n", i+1, link, strings.TrimSpace(snippet)))
		}
	}

	return &ToolResult{ForLLM: sb.String(), IsError: false}
}
