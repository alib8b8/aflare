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
	"fmt"
	"sort"
	"strings"
)

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
