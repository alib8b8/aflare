// Copyright (c) 2026 aflare Contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
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
	"strings"
	"testing"
)

// TestSearchAggregateNode_Registration verifies the search_aggregate node is
// registered in the global registry.
func TestSearchAggregateNode_Registration(t *testing.T) {
	node, ok := Get("search_aggregate")
	if !ok {
		t.Fatal("search_aggregate node not found in registry")
	}
	if node.Name() != "search_aggregate" {
		t.Errorf("expected node name 'search_aggregate', got '%s'", node.Name())
	}
}

// TestSearchAggregateNode_Description ensures Description returns non-empty string.
func TestSearchAggregateNode_Description(t *testing.T) {
	node := &SearchAggregateNode{}
	if desc := node.Description(); desc == "" {
		t.Error("Description() returned empty string")
	}
}

// TestSearchAggregateNode_Schema verifies the schema name, description, and params.
func TestSearchAggregateNode_Schema(t *testing.T) {
	node := &SearchAggregateNode{}
	schema := node.Schema()

	if schema.Name != "search_aggregate" {
		t.Errorf("Schema().Name = %q, want %q", schema.Name, "search_aggregate")
	}
	if schema.Description == "" {
		t.Error("Schema().Description is empty")
	}
	if schema.Input == "" {
		t.Error("Schema().Input is empty")
	}
	if schema.Output == "" {
		t.Error("Schema().Output is empty")
	}

	expectedParams := []string{
		"sources", "region", "category", "limit",
		"time_range", "sort_by", "min_score", "output",
	}
	for _, name := range expectedParams {
		found := false
		for _, p := range schema.Params {
			if p.Name == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected param %q in schema", name)
		}
	}
}

// TestSearchAggregateNode_SchemaDefaults verifies documented default values.
func TestSearchAggregateNode_SchemaDefaults(t *testing.T) {
	node := &SearchAggregateNode{}
	schema := node.Schema()
	defaults := map[string]string{
		"sources":    "reddit,hn,github,news",
		"region":     "global",
		"category":   "all",
		"limit":      "10",
		"time_range": "week",
		"sort_by":    "signal",
		"min_score":  "0",
		"output":     "markdown",
	}
	for _, p := range schema.Params {
		if want, ok := defaults[p.Name]; ok && p.Default != want {
			t.Errorf("param %q default = %q, want %q", p.Name, p.Default, want)
		}
	}
}

// TestSearchSources_AllDefined verifies that exactly 19 source types are
// defined and each has a unique, non-empty string value.
func TestSearchSources_AllDefined(t *testing.T) {
	allSources := []SearchSource{
		SourceReddit, SourceTwitter, SourceYouTube, SourceHackerNews,
		SourceGitHub, SourceGoogle, SourceWeibo, SourceZhihu,
		SourceBilibili, SourceLinkedIn, SourceNews, SourceFinance,
		SourceAcademic, SourceShopping, SourceGeopolitical,
		SourceInfrastructure, SourceGlobalEvents, SourceEnergy,
		SourceSupplyChain,
	}
	if len(allSources) != 19 {
		t.Fatalf("expected 19 source constants, got %d", len(allSources))
	}

	seen := make(map[SearchSource]bool)
	for _, s := range allSources {
		if string(s) == "" {
			t.Errorf("source constant has empty string value")
		}
		if seen[s] {
			t.Errorf("duplicate source constant: %s", s)
		}
		seen[s] = true
	}
}

// TestSearchSources_KnownValues verifies a few known source string values
// to guard against accidental re-ordering or renaming.
func TestSearchSources_KnownValues(t *testing.T) {
	tests := []struct {
		source SearchSource
		want   string
	}{
		{SourceReddit, "reddit"},
		{SourceTwitter, "twitter"},
		{SourceHackerNews, "hn"},
		{SourceGitHub, "github"},
		{SourceGoogle, "google"},
		{SourceWeibo, "weibo"},
		{SourceZhihu, "zhihu"},
		{SourceBilibili, "bilibili"},
		{SourceGeopolitical, "geopolitical"},
		{SourceInfrastructure, "infrastructure"},
		{SourceGlobalEvents, "globalevents"},
		{SourceEnergy, "energy"},
		{SourceSupplyChain, "supplychain"},
	}
	for _, tt := range tests {
		t.Run(string(tt.source), func(t *testing.T) {
			if string(tt.source) != tt.want {
				t.Errorf("string(%s) = %q, want %q", tt.source, string(tt.source), tt.want)
			}
		})
	}
}

// TestParseSources verifies source string parsing and validation.
func TestParseSources(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantLen      int
		wantContains []SearchSource
	}{
		{"single", "reddit", 1, []SearchSource{SourceReddit}},
		{"multiple", "reddit,twitter,youtube", 3, nil},
		{"with_spaces", "reddit, twitter , hn", 3, nil},
		{"uppercase", "REDDIT,TWITTER", 2, nil},
		{"mixed_case", "Reddit,Twitter", 2, nil},
		{"invalid_only_falls_back", "invalidsource,anothersource", 2, nil},
		{"empty_falls_back", "", 2, nil},
		{"mixed_valid_invalid", "reddit,invalid,twitter", 2, []SearchSource{SourceReddit, SourceTwitter}},
		{
			"all_19",
			"reddit,twitter,youtube,hn,github,google,weibo,zhihu,bilibili,linkedin,news,finance,academic,shopping,geopolitical,infrastructure,globalevents,energy,supplychain",
			19,
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseSources(tt.input)
			if len(got) != tt.wantLen {
				t.Errorf("parseSources(%q) returned %d sources, want %d: %v",
					tt.input, len(got), tt.wantLen, got)
			}
			for _, want := range tt.wantContains {
				found := false
				for _, g := range got {
					if g == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("parseSources(%q) missing expected source %s", tt.input, want)
				}
			}
		})
	}
}

// TestSearchAggregate_Execute_EmptyQuery verifies Execute errors on empty query
// before any source fetching is attempted.
func TestSearchAggregate_Execute_EmptyQuery(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"whitespace", "   "},
		{"tabs_newlines", "\n\t  \n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := (&SearchAggregateNode{}).Execute(ctx, tt.input, nil)
			if err == nil {
				t.Fatal("expected error for empty query, got nil")
			}
			if !strings.Contains(err.Error(), "search query required") {
				t.Errorf("expected 'search query required' error, got: %v", err)
			}
		})
	}
}

// TestSearchAggregate_Execute_NonNetworkSources tests Execute end-to-end using
// sources that return static results without making any HTTP calls (Twitter,
// YouTube, etc.). This keeps the test fully deterministic and offline.
func TestSearchAggregate_Execute_NonNetworkSources(t *testing.T) {
	ctx := context.Background()
	params := map[string]string{
		"sources": "twitter",
		"limit":   "5",
		"output":  "json",
	}
	output, err := (&SearchAggregateNode{}).Execute(ctx, "test query", params)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if output == "" {
		t.Fatal("expected non-empty output")
	}

	var agg AggregatedResults
	if err := json.Unmarshal([]byte(output), &agg); err != nil {
		t.Fatalf("failed to parse JSON output: %v\noutput: %s", err, output)
	}
	if agg.Query != "test query" {
		t.Errorf("expected query 'test query', got %q", agg.Query)
	}
	if agg.Count != 1 {
		t.Errorf("expected 1 result from twitter, got %d", agg.Count)
	}
	if len(agg.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(agg.Results))
	}
	if agg.Results[0].Source != SourceTwitter {
		t.Errorf("expected source twitter, got %s", agg.Results[0].Source)
	}
	if len(agg.Sources) != 1 || agg.Sources[0] != SourceTwitter {
		t.Errorf("expected sources list [twitter], got %v", agg.Sources)
	}
}

// TestSearchAggregate_Execute_MinScoreFilter verifies the min_score param
// filters out results below the threshold. Twitter returns a result with
// Score=0, so min_score > 0 should remove it.
func TestSearchAggregate_Execute_MinScoreFilter(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name      string
		minScore  string
		wantCount int
	}{
		{"zero_keeps_all", "0", 1},
		{"one_filters_all", "1", 0},
		{"float_threshold", "0.5", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := map[string]string{
				"sources":   "twitter",
				"min_score": tt.minScore,
				"output":    "json",
			}
			output, err := (&SearchAggregateNode{}).Execute(ctx, "test", params)
			if err != nil {
				t.Fatalf("Execute failed: %v", err)
			}
			var agg AggregatedResults
			if err := json.Unmarshal([]byte(output), &agg); err != nil {
				t.Fatalf("failed to parse JSON: %v", err)
			}
			if agg.Count != tt.wantCount {
				t.Errorf("min_score=%q: expected %d results, got %d",
					tt.minScore, tt.wantCount, agg.Count)
			}
		})
	}
}

// TestSearchAggregate_Execute_LimitParam verifies that the limit param is
// parsed without error for valid and invalid values.
func TestSearchAggregate_Execute_LimitParam(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name    string
		limit   string
		wantErr bool
	}{
		{"valid_small", "5", false},
		{"valid_large", "100", false},
		{"zero_defaults_to_10", "0", false},
		{"negative_defaults_to_10", "-5", false},
		{"invalid_string_defaults", "abc", false},
		{"empty_string_defaults", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := map[string]string{
				"sources": "twitter",
				"limit":   tt.limit,
				"output":  "json",
			}
			_, err := (&SearchAggregateNode{}).Execute(ctx, "test", params)
			if (err != nil) != tt.wantErr {
				t.Errorf("limit=%q: error = %v, wantErr %v", tt.limit, err, tt.wantErr)
			}
		})
	}
}

// TestSearchAggregate_Execute_OutputFormats verifies all output format options
// produce non-empty results without error.
func TestSearchAggregate_Execute_OutputFormats(t *testing.T) {
	ctx := context.Background()
	formats := []struct {
		name   string
		output string
	}{
		{"json", "json"},
		{"markdown", "markdown"},
		{"text", "text"},
		{"default_unrecognized", "unknown"},
	}
	for _, tt := range formats {
		t.Run(tt.name, func(t *testing.T) {
			params := map[string]string{
				"sources": "twitter",
				"output":  tt.output,
			}
			output, err := (&SearchAggregateNode{}).Execute(ctx, "test", params)
			if err != nil {
				t.Fatalf("Execute with output=%s failed: %v", tt.output, err)
			}
			if output == "" {
				t.Errorf("expected non-empty output for format %s", tt.output)
			}
		})
	}
}

// TestSearchAggregate_Execute_RegionCategoryParams verifies that the region
// and category params are accepted without error (they are declared in the
// schema for documentation; Execute does not currently filter on them).
func TestSearchAggregate_Execute_RegionCategoryParams(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name     string
		region   string
		category string
	}{
		{"us_technology", "us", "technology"},
		{"eu_politics", "eu", "politics"},
		{"asia_economy", "asia", "economy"},
		{"cn_energy", "cn", "energy"},
		{"mena_military", "mena", "military"},
		{"global_all", "global", "all"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := map[string]string{
				"sources":  "twitter",
				"region":   tt.region,
				"category": tt.category,
				"output":   "json",
			}
			output, err := (&SearchAggregateNode{}).Execute(ctx, "test", params)
			if err != nil {
				t.Fatalf("Execute with region=%s category=%s failed: %v",
					tt.region, tt.category, err)
			}
			if output == "" {
				t.Error("expected non-empty output")
			}
		})
	}
}

// TestSearchAggregate_Execute_SortByParam verifies the sort_by param is
// accepted for all documented values without error.
func TestSearchAggregate_Execute_SortByParam(t *testing.T) {
	ctx := context.Background()
	sorts := []string{"signal", "relevance", "time"}
	for _, sortBy := range sorts {
		t.Run(sortBy, func(t *testing.T) {
			params := map[string]string{
				"sources": "twitter",
				"sort_by": sortBy,
				"output":  "json",
			}
			_, err := (&SearchAggregateNode{}).Execute(ctx, "test", params)
			if err != nil {
				t.Fatalf("Execute with sort_by=%s failed: %v", sortBy, err)
			}
		})
	}
}

// TestSearchAggregate_Execute_TimeRangeParam verifies the time_range param is
// accepted for all documented values without error.
func TestSearchAggregate_Execute_TimeRangeParam(t *testing.T) {
	ctx := context.Background()
	ranges := []string{"day", "week", "month", "year", "all"}
	for _, tr := range ranges {
		t.Run(tr, func(t *testing.T) {
			params := map[string]string{
				"sources":    "twitter",
				"time_range": tr,
				"output":     "json",
			}
			_, err := (&SearchAggregateNode{}).Execute(ctx, "test", params)
			if err != nil {
				t.Fatalf("Execute with time_range=%s failed: %v", tr, err)
			}
		})
	}
}

// TestRankResults verifies the rankResults sorting behavior for each sort mode.
func TestRankResults(t *testing.T) {
	results := []SearchResult{
		{Title: "low", Score: 1.0},
		{Title: "high", Score: 10.0},
		{Title: "mid", Score: 5.0},
	}

	t.Run("signal_descending", func(t *testing.T) {
		input := make([]SearchResult, len(results))
		copy(input, results)
		got := rankResults(input, "signal")
		if len(got) != 3 {
			t.Fatalf("expected 3 results, got %d", len(got))
		}
		if got[0].Title != "high" {
			t.Errorf("signal: expected 'high' first, got %q", got[0].Title)
		}
		if got[2].Title != "low" {
			t.Errorf("signal: expected 'low' last, got %q", got[2].Title)
		}
	})

	t.Run("unknown_defaults_to_signal", func(t *testing.T) {
		input := make([]SearchResult, len(results))
		copy(input, results)
		got := rankResults(input, "unknown_sort")
		if len(got) != 3 {
			t.Fatalf("expected 3 results, got %d", len(got))
		}
		if got[0].Title != "high" {
			t.Errorf("default sort: expected 'high' first, got %q", got[0].Title)
		}
	})

	t.Run("empty_results", func(t *testing.T) {
		got := rankResults(nil, "signal")
		if len(got) != 0 {
			t.Errorf("expected 0 results for empty input, got %d", len(got))
		}
	})
}

// TestJoinSources verifies the joinSources helper.
func TestJoinSources(t *testing.T) {
	tests := []struct {
		name string
		srcs []SearchSource
		want string
	}{
		{"empty", nil, ""},
		{"single", []SearchSource{SourceReddit}, "reddit"},
		{"multiple", []SearchSource{SourceReddit, SourceTwitter, SourceHackerNews}, "reddit, twitter, hn"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := joinSources(tt.srcs); got != tt.want {
				t.Errorf("joinSources() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestStripHTML verifies the stripHTML helper removes HTML tags.
func TestStripHTML(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"no_html", "hello world", "hello world"},
		{"single_tag", "<b>bold</b>", "bold"},
		{"nested", "<p><a href=x>link</a></p>", "link"},
		{"multiple", "<b>a</b> <i>b</i>", "a b"},
		{"empty", "", ""},
		{"unclosed_tag", "<b>text", "text"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stripHTML(tt.in); got != tt.want {
				t.Errorf("stripHTML(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestFormatMarkdownResults_NoResults verifies the markdown formatter handles
// empty result sets.
func TestFormatMarkdownResults_NoResults(t *testing.T) {
	agg := AggregatedResults{
		Query:   "nothing",
		Results: nil,
		Count:   0,
		Sources: []SearchSource{SourceTwitter},
	}
	out := formatMarkdownResults(agg)
	if !strings.Contains(out, "nothing") {
		t.Errorf("expected query in markdown output")
	}
	if !strings.Contains(out, "No results found") {
		t.Errorf("expected 'No results found' message for empty results")
	}
}

// TestFormatTextResults verifies the text formatter produces output containing
// the query and result count.
func TestFormatTextResults(t *testing.T) {
	agg := AggregatedResults{
		Query:   "my query",
		Results: []SearchResult{{Title: "r1", URL: "http://example.com", Source: SourceTwitter}},
		Count:   1,
		Sources: []SearchSource{SourceTwitter},
	}
	out := formatTextResults(agg)
	if !strings.Contains(out, "my query") {
		t.Error("expected query in text output")
	}
	if !strings.Contains(out, "r1") {
		t.Error("expected result title in text output")
	}
	if !strings.Contains(out, "http://example.com") {
		t.Error("expected result URL in text output")
	}
}

// TestSearchAggregate_Execute_NetworkSources documents that sources like
// reddit, hn, and github make real HTTP calls. The test is skipped to keep the
// suite deterministic and offline.
func TestSearchAggregate_Execute_NetworkSources(t *testing.T) {
	t.Skip("requires network - reddit/hn/github sources make HTTP calls")

	ctx := context.Background()
	params := map[string]string{
		"sources": "reddit,hn,github",
		"output":  "json",
	}
	_, err := (&SearchAggregateNode{}).Execute(ctx, "test", params)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
}

// TestGlobalNewsSources verifies the region-to-source mapping is populated.
func TestGlobalNewsSources(t *testing.T) {
	expectedRegions := []string{"global", "us", "eu", "asia", "cn", "mena"}
	for _, region := range expectedRegions {
		t.Run(region, func(t *testing.T) {
			sources, ok := globalNewsSources[region]
			if !ok {
				t.Errorf("expected region %q in globalNewsSources", region)
				return
			}
			if len(sources) == 0 {
				t.Errorf("expected non-empty source list for region %q", region)
			}
		})
	}
}
