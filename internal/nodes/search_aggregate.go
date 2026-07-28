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
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/alib8b8/llm-box/internal/logger"
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
