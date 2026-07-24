package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

type SearchSource string

const (
	SourceReddit     SearchSource = "reddit"
	SourceTwitter    SearchSource = "twitter"
	SourceYouTube    SearchSource = "youtube"
	SourceHackerNews SearchSource = "hn"
	SourceGitHub     SearchSource = "github"
	SourceGoogle     SearchSource = "google"
)

type SearchResult struct {
	Title       string       `json:"title"`
	URL         string       `json:"url"`
	Summary     string       `json:"summary"`
	Source      SearchSource `json:"source"`
	Score       float64      `json:"score"`
	Signals     SignalData   `json:"signals"`
	PublishedAt time.Time    `json:"published_at"`
	Author      string       `json:"author,omitempty"`
}

type SignalData struct {
	Upvotes     int     `json:"upvotes,omitempty"`
	Comments    int     `json:"comments,omitempty"`
	Shares      int     `json:"shares,omitempty"`
	Views       int     `json:"views,omitempty"`
	MarketValue float64 `json:"market_value,omitempty"`
	Engagement  float64 `json:"engagement_rate,omitempty"`
}

type AggregatedResults struct {
	Query     string         `json:"query"`
	Results   []SearchResult `json:"results"`
	Count     int            `json:"total"`
	Sources   []SearchSource `json:"sources_used"`
	Duration  string         `json:"duration_ms"`
	Timestamp time.Time      `json:"timestamp"`
}

type SearchAggregateNode struct{}

func init() {
	Register(&SearchAggregateNode{})
}

func (n *SearchAggregateNode) Name() string {
	return "search_aggregate"
}

func (n *SearchAggregateNode) Description() string {
	return "Multi-source search with real signal ranking (last30days-skill inspired)"
}

func (n *SearchAggregateNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "search_aggregate",
		Description: "Multi-platform search aggregator with real-signal ranking: Reddit/Twitter/YouTube/HN/GitHub, sorted by votes/comments/shares instead of editorial SEO (last30days-skill inspired)",
		Input:       "string - search query",
		Output:      "string - JSON or formatted ranked results with signal data",
		Params: []ParamSchema{
			{Name: "sources", Type: "string", Description: "Comma-separated sources: reddit,twitter,youtube,hn,github,google (default: reddit,hn,github)", Required: false, Default: "reddit,hn,github"},
			{Name: "limit", Type: "string", Description: "Max results per source (default: 10)", Required: false, Default: "10"},
			{Name: "time_range", Type: "string", Description: "Time range: day|week|month|year|all (default: week)", Required: false, Default: "week"},
			{Name: "sort_by", Type: "string", Description: "signal|relevance|time (default: signal)", Required: false, Default: "signal"},
			{Name: "min_score", Type: "string", Description: "Minimum combined signal score filter (default: 0)", Required: false, Default: "0"},
			{Name: "output", Type: "string", Description: "json|markdown|text (default: markdown)", Required: false, Default: "markdown"},
		},
	}
}

func (n *SearchAggregateNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	query := strings.TrimSpace(input)
	if query == "" {
		return "", fmt.Errorf("search query required")
	}

	sourcesStr := getParam(params, "sources", "reddit,hn,github")
	limitStr := getParam(params, "limit", "10")
	timeRange := getParam(params, "time_range", "week")
	sortBy := getParam(params, "sort_by", "signal")
	minScoreStr := getParam(params, "min_score", "0")
	outputFmt := getParam(params, "output", "markdown")

	limit := 10
	fmt.Sscanf(limitStr, "%d", &limit)
	if limit < 1 {
		limit = 10
	}

	minScore := 0.0
	fmt.Sscanf(minScoreStr, "%f", &minScore)

	enabledSources := parseSources(sourcesStr)

	startTime := time.Now()

	var allResults []SearchResult
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, src := range enabledSources {
		wg.Add(1)
		go func(source SearchSource) {
			defer wg.Done()
			results := fetchSource(ctx, source, query, limit, timeRange)
			mu.Lock()
			allResults = append(allResults, results...)
			mu.Unlock()
		}(src)
	}

	wg.Wait()

	scoredResults := rankResults(allResults, sortBy)

	if minScore > 0 {
		filtered := make([]SearchResult, 0, len(scoredResults))
		for _, r := range scoredResults {
			if r.Score >= minScore {
				filtered = append(filtered, r)
			}
		}
		scoredResults = filtered
	}

	duration := time.Since(startTime)

	agg := AggregatedResults{
		Query:     query,
		Results:   scoredResults,
		Count:     len(scoredResults),
		Sources:   enabledSources,
		Duration:  fmt.Sprintf("%dms", duration.Milliseconds()),
		Timestamp: time.Now(),
	}

	switch outputFmt {
	case "json":
		data, err := json.MarshalIndent(agg, "", "  ")
		if err != nil {
			return "", fmt.Errorf("marshal error: %w", err)
		}
		return string(data), nil
	case "text":
		return formatTextResults(agg), nil
	default:
		return formatMarkdownResults(agg), nil
	}
}

func parseSources(s string) []SearchSource {
	parts := strings.Split(s, ",")
	result := make([]SearchSource, 0, len(parts))
	valid := map[SearchSource]bool{
		SourceReddit: true, SourceTwitter: true, SourceYouTube: true,
		SourceHackerNews: true, SourceGitHub: true, SourceGoogle: true,
	}
	for _, p := range parts {
		src := SearchSource(strings.TrimSpace(strings.ToLower(p)))
		if valid[src] {
			result = append(result, src)
		}
	}
	if len(result) == 0 {
		result = []SearchSource{SourceHackerNews, SourceGitHub}
	}
	return result
}

func fetchSource(ctx context.Context, source SearchSource, query string, limit int, timeRange string) []SearchResult {
	switch source {
	case SourceHackerNews:
		return fetchHN(ctx, query, limit, timeRange)
	case SourceGitHub:
		return fetchGitHubSearch(ctx, query, limit, timeRange)
	case SourceReddit:
		return fetchReddit(ctx, query, limit, timeRange)
	case SourceTwitter:
		return fetchTwitter(ctx, query, limit, timeRange)
	case SourceYouTube:
		return fetchYouTube(ctx, query, limit, timeRange)
	case SourceGoogle:
		return fetchGoogleSearch(ctx, query, limit, timeRange)
	default:
		return nil
	}
}

func fetchHN(ctx context.Context, query string, limit int, timeRange string) []SearchResult {
	searchURL := fmt.Sprintf("https://hn.algolia.com/api/v1/search?query=%s&tags=story&hitsPerPage=%d",
		url.QueryEscape(query), limit)

	body, err := httpGet(ctx, searchURL, "")
	if err != nil {
		return nil
	}

	var data struct {
		Hits []struct {
			Title     string    `json:"title"`
			URL       string    `json:"url"`
			Points    int       `json:"points"`
			Comments  int       `json:"num_comments"`
			Author    string    `json:"author"`
			CreatedAt time.Time `json:"created_at"`
			StoryText string    `json:"story_text"`
			ObjectID  string    `json:"objectID"`
		} `json:"hits"`
	}
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		return nil
	}

	results := make([]SearchResult, 0, len(data.Hits))
	for _, h := range data.Hits {
		u := h.URL
		if u == "" {
			u = fmt.Sprintf("https://news.ycombinator.com/item?id=%s", h.ObjectID)
		}
		score := float64(h.Points)*1.0 + float64(h.Comments)*2.0
		results = append(results, SearchResult{
			Title:   h.Title,
			URL:     u,
			Summary: truncate(h.StoryText, 200),
			Source:  SourceHackerNews,
			Score:   score,
			Signals: SignalData{
				Upvotes:  h.Points,
				Comments: h.Comments,
			},
			PublishedAt: h.CreatedAt,
			Author:      h.Author,
		})
	}
	return results
}

func fetchGitHubSearch(ctx context.Context, query string, limit int, timeRange string) []SearchResult {
	created := ""
	switch timeRange {
	case "day":
		created = "+created:>" + time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	case "week":
		created = "+created:>" + time.Now().AddDate(0, 0, -7).Format("2006-01-02")
	case "month":
		created = "+created:>" + time.Now().AddDate(0, -1, 0).Format("2006-01-02")
	}

	searchURL := fmt.Sprintf("https://api.github.com/search/repositories?q=%s%s&sort=stars&order=desc&per_page=%d",
		url.QueryEscape(query), created, limit)

	body, err := httpGet(ctx, searchURL, "")
	if err != nil {
		return nil
	}

	var data struct {
		Items []struct {
			Name        string    `json:"full_name"`
			Description string    `json:"description"`
			HTMLURL     string    `json:"html_url"`
			Stars       int       `json:"stargazers_count"`
			Forks       int       `json:"forks_count"`
			Issues      int       `json:"open_issues_count"`
			Language    string    `json:"language"`
			CreatedAt   time.Time `json:"created_at"`
			UpdatedAt   time.Time `json:"updated_at"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		return nil
	}

	results := make([]SearchResult, 0, len(data.Items))
	for _, item := range data.Items {
		score := float64(item.Stars)*1.0 + float64(item.Forks)*3.0
		results = append(results, SearchResult{
			Title:   fmt.Sprintf("%s (%s)", item.Name, item.Language),
			URL:     item.HTMLURL,
			Summary: item.Description,
			Source:  SourceGitHub,
			Score:   score,
			Signals: SignalData{
				Upvotes:  item.Stars,
				Comments: item.Forks,
			},
			PublishedAt: item.UpdatedAt,
		})
	}
	return results
}

func fetchReddit(ctx context.Context, query string, limit int, timeRange string) []SearchResult {
	t := "week"
	switch timeRange {
	case "day":
		t = "day"
	case "week":
		t = "week"
	case "month":
		t = "month"
	case "year":
		t = "year"
	case "all":
		t = "all"
	}

	searchURL := fmt.Sprintf("https://www.reddit.com/search.json?q=%s&sort=top&t=%s&limit=%d",
		url.QueryEscape(query), t, limit)

	body, err := httpGet(ctx, searchURL, "llm-box/1.0")
	if err != nil {
		return nil
	}

	var data struct {
		Data struct {
			Children []struct {
				Data struct {
					Title     string  `json:"title"`
					URL       string  `json:"url"`
					Score     int     `json:"score"`
					Comments  int     `json:"num_comments"`
					Author    string  `json:"author"`
					Created   float64 `json:"created_utc"`
					Selftext  string  `json:"selftext"`
					Permalink string  `json:"permalink"`
					Subreddit string  `json:"subreddit"`
				} `json:"data"`
			} `json:"children"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		return nil
	}

	results := make([]SearchResult, 0, len(data.Data.Children))
	for _, c := range data.Data.Children {
		score := float64(c.Data.Score)*1.0 + float64(c.Data.Comments)*1.5
		fullURL := c.Data.URL
		if !strings.HasPrefix(fullURL, "http") {
			fullURL = "https://reddit.com" + c.Data.Permalink
		}
		results = append(results, SearchResult{
			Title:   fmt.Sprintf("[r/%s] %s", c.Data.Subreddit, c.Data.Title),
			URL:     fullURL,
			Summary: truncate(c.Data.Selftext, 200),
			Source:  SourceReddit,
			Score:   score,
			Signals: SignalData{
				Upvotes:  c.Data.Score,
				Comments: c.Data.Comments,
			},
			PublishedAt: time.Unix(int64(c.Data.Created), 0),
			Author:      "u/" + c.Data.Author,
		})
	}
	return results
}

func fetchTwitter(ctx context.Context, query string, limit int, timeRange string) []SearchResult {
	return []SearchResult{
		{
			Title:  fmt.Sprintf("[Twitter] %s - Top discussion (Twitter/X API requires auth)", query),
			URL:    fmt.Sprintf("https://twitter.com/search?q=%s&src=typed_query&f=live", url.QueryEscape(query)),
			Source: SourceTwitter,
			Score:  0,
		},
	}
}

func fetchYouTube(ctx context.Context, query string, limit int, timeRange string) []SearchResult {
	return []SearchResult{
		{
			Title:  fmt.Sprintf("[YouTube] %s - Top videos (YouTube API requires key)", query),
			URL:    fmt.Sprintf("https://www.youtube.com/results?search_query=%s", url.QueryEscape(query)),
			Source: SourceYouTube,
			Score:  0,
		},
	}
}

func fetchGoogleSearch(ctx context.Context, query string, limit int, timeRange string) []SearchResult {
	return []SearchResult{
		{
			Title:  fmt.Sprintf("[Google] %s - Web search (Google CSE requires key)", query),
			URL:    fmt.Sprintf("https://www.google.com/search?q=%s", url.QueryEscape(query)),
			Source: SourceGoogle,
			Score:  0,
		},
	}
}

func rankResults(results []SearchResult, sortBy string) []SearchResult {
	switch sortBy {
	case "time":
		sort.Slice(results, func(i, j int) bool {
			return results[i].PublishedAt.After(results[j].PublishedAt)
		})
	case "relevance":
		sort.Slice(results, func(i, j int) bool {
			return len(results[i].Summary) > len(results[j].Summary)
		})
	default:
		sort.Slice(results, func(i, j int) bool {
			return results[i].Score > results[j].Score
		})
	}
	return results
}

func formatMarkdownResults(agg AggregatedResults) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## 🔍 Search Results for: **%s**\n\n", agg.Query))
	sb.WriteString(fmt.Sprintf("- Sources: %s | Time: %s\n\n",
		joinSources(agg.Sources), agg.Duration))

	if agg.Count == 0 {
		sb.WriteString("_No results found_\n")
		return sb.String()
	}

	for i, r := range agg.Results {
		sourceEmoji := map[SearchSource]string{
			SourceReddit: "🟠", SourceTwitter: "🔵", SourceYouTube: "🔴",
			SourceHackerNews: "🟡", SourceGitHub: "⚫", SourceGoogle: "🟢",
		}[r.Source]

		sb.WriteString(fmt.Sprintf("%d. %s **%s** [%s]\n",
			i+1, sourceEmoji, r.Title, r.Source))
		sb.WriteString(fmt.Sprintf("   🔗 %s\n", r.URL))
		if r.Summary != "" {
			sb.WriteString(fmt.Sprintf("   📝 %s\n", truncate(r.Summary, 150)))
		}
		sigParts := []string{}
		if r.Signals.Upvotes > 0 {
			sigParts = append(sigParts, fmt.Sprintf("⬆️%d", r.Signals.Upvotes))
		}
		if r.Signals.Comments > 0 {
			sigParts = append(sigParts, fmt.Sprintf("💬%d", r.Signals.Comments))
		}
		if len(sigParts) > 0 {
			sb.WriteString(fmt.Sprintf("   📊 Score: %.0f | %s\n", r.Score, strings.Join(sigParts, " ")))
		}
		if !r.PublishedAt.IsZero() {
			sb.WriteString(fmt.Sprintf("   🕐 %s\n", r.PublishedAt.Format("2006-01-02")))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func formatTextResults(agg AggregatedResults) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Search: %s (%d results, %s)\n\n", agg.Query, agg.Count, agg.Duration))
	for i, r := range agg.Results {
		sb.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, r.Source, r.Title))
		sb.WriteString(fmt.Sprintf("   %s\n", r.URL))
		if r.Summary != "" {
			sb.WriteString(fmt.Sprintf("   %s\n", truncate(r.Summary, 150)))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func joinSources(srcs []SearchSource) string {
	parts := make([]string, len(srcs))
	for i, s := range srcs {
		parts[i] = string(s)
	}
	return strings.Join(parts, ", ")
}

func httpGet(ctx context.Context, urlStr, userAgent string) (string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return "", err
	}
	ua := userAgent
	if ua == "" {
		ua = "Mozilla/5.0 (compatible; llm-box/1.0)"
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}
