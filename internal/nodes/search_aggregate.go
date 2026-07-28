// Copyright (c) 2026 llm-box Contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package nodes

import (
"context"
"encoding/json"
"fmt"
"github.com/alib8b8/llm-box/internal/logger"
"io"
"net/http"
"runtime/debug"
"sort"
"strings"
"sync"
"time"
)

type SearchSource string

const (
	SourceReddit         SearchSource = "reddit"
	SourceTwitter        SearchSource = "twitter"
	SourceYouTube        SearchSource = "youtube"
	SourceHackerNews     SearchSource = "hn"
	SourceGitHub         SearchSource = "github"
	SourceGoogle         SearchSource = "google"
	SourceWeibo          SearchSource = "weibo"
	SourceZhihu          SearchSource = "zhihu"
	SourceBilibili       SearchSource = "bilibili"
	SourceLinkedIn       SearchSource = "linkedin"
	SourceNews           SearchSource = "news"
	SourceFinance        SearchSource = "finance"
	SourceAcademic       SearchSource = "academic"
	SourceShopping       SearchSource = "shopping"
	SourceGeopolitical   SearchSource = "geopolitical"
	SourceInfrastructure SearchSource = "infrastructure"
	SourceGlobalEvents   SearchSource = "globalevents"
	SourceEnergy         SearchSource = "energy"
	SourceSupplyChain    SearchSource = "supplychain"
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
			{Name: "sources", Type: "string", Description: "Comma-separated sources: reddit,twitter,youtube,hn,github,google,weibo,zhihu,bilibili,linkedin,news,finance,academic,shopping,geopolitical,infrastructure,globalevents,energy,supplychain (default: reddit,hn,github,news)", Required: false, Default: "reddit,hn,github,news"},
			{Name: "region", Type: "string", Description: "Region filter: global,us,eu,asia,cn,mena (default: global)", Required: false, Default: "global"},
			{Name: "category", Type: "string", Description: "Category filter: politics,economy,technology,military,energy,health,all (default: all)", Required: false, Default: "all"},
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
	if _, err := fmt.Sscanf(limitStr, "%d", &limit); err != nil {
		// keep default value on parse failure
	}
	if limit < 1 {
		limit = 10
	}

	minScore := 0.0
	if _, err := fmt.Sscanf(minScoreStr, "%f", &minScore); err != nil {
		// keep default value on parse failure
	}

	enabledSources := parseSources(sourcesStr)

	startTime := time.Now()

	var allResults []SearchResult
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, src := range enabledSources {
		wg.Add(1)
		go func(source SearchSource) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					logger.Error("search source fetch panicked",
						"source", string(source),
						"panic", r,
						"stack", string(debug.Stack()),
					)
				}
			}()
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



func parseSources(s string) []SearchSource {
	parts := strings.Split(s, ",")
	result := make([]SearchSource, 0, len(parts))
	valid := map[SearchSource]bool{
		SourceReddit: true, SourceTwitter: true, SourceYouTube: true,
		SourceHackerNews: true, SourceGitHub: true, SourceGoogle: true,
		SourceWeibo: true, SourceZhihu: true, SourceBilibili: true,
		SourceLinkedIn: true, SourceNews: true, SourceFinance: true,
		SourceAcademic: true, SourceShopping: true,
		SourceGeopolitical: true, SourceInfrastructure: true,
		SourceGlobalEvents: true, SourceEnergy: true, SourceSupplyChain: true,
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
	case SourceWeibo:
		return fetchWeibo(ctx, query, limit, timeRange)
	case SourceZhihu:
		return fetchZhihu(ctx, query, limit, timeRange)
	case SourceBilibili:
		return fetchBilibili(ctx, query, limit, timeRange)
	case SourceLinkedIn:
		return fetchLinkedIn(ctx, query, limit, timeRange)
	case SourceNews:
		return fetchNews(ctx, query, limit, timeRange)
	case SourceFinance:
		return fetchFinance(ctx, query, limit, timeRange)
	case SourceAcademic:
		return fetchAcademic(ctx, query, limit, timeRange)
	case SourceShopping:
		return fetchShopping(ctx, query, limit, timeRange)
	case SourceGeopolitical:
		return fetchGeopolitical(ctx, query, limit, timeRange)
	case SourceInfrastructure:
		return fetchInfrastructure(ctx, query, limit, timeRange)
	case SourceGlobalEvents:
		return fetchGlobalEvents(ctx, query, limit, timeRange)
	case SourceEnergy:
		return fetchEnergy(ctx, query, limit, timeRange)
	case SourceSupplyChain:
		return fetchSupplyChain(ctx, query, limit, timeRange)
	default:
		return nil
	}
}

var globalNewsSources = map[string][]string{
	"global": {"reuters.com", "apnews.com", "bbc.com", "cnn.com", "aljazeera.com"},
	"us":     {"nytimes.com", "washingtonpost.com", "cnn.com", "foxnews.com"},
	"eu":     {"dw.com", "bbc.com", "lemonde.fr", "spiegel.de"},
	"asia":   {"scmp.com", "nikkei.com", "straitstimes.com", "thehindu.com"},
	"cn":     {"xinhuanet.com", "people.com.cn", "caixin.com"},
	"mena":   {"aljazeera.com", "arabnews.com", "thenationalnews.com"},
}

func httpGet(ctx context.Context, urlStr, userAgent string) (string, error) {
	if err := validateURL(urlStr); err != nil {
		return "", fmt.Errorf("URL validation failed: %w", err)
	}

	client := &http.Client{
		Timeout:       15 * time.Second,
		Transport:     safeHTTPClient.Transport,
		CheckRedirect: httpRedirectValidator(validateURL),
	}
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

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxHTTPResponseSize))
	if err != nil {
		return "", err
	}
	return string(body), nil
}
