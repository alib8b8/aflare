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
	"net/url"
	"time"
)

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

func fetchLinkedIn(ctx context.Context, query string, limit int, timeRange string) []SearchResult {
	return []SearchResult{
		{
			Title:       fmt.Sprintf("LinkedIn: %s (Professional results)", query),
			URL:         fmt.Sprintf("https://www.linkedin.com/search/results/all/?keywords=%s", url.QueryEscape(query)),
			Summary:     "LinkedIn professional network search results - visit link for full details",
			Source:      SourceLinkedIn,
			Score:       1.0,
			PublishedAt: time.Now(),
		},
	}
}

func fetchFinance(ctx context.Context, query string, limit int, timeRange string) []SearchResult {
	return []SearchResult{
		{
			Title:       fmt.Sprintf("Finance: %s - Market & Finance News", query),
			URL:         fmt.Sprintf("https://www.google.com/finance?q=%s", url.QueryEscape(query)),
			Summary:     "Financial markets, stocks, and economic news search results",
			Source:      SourceFinance,
			Score:       1.0,
			PublishedAt: time.Now(),
		},
	}
}

func fetchShopping(ctx context.Context, query string, limit int, timeRange string) []SearchResult {
	return []SearchResult{
		{
			Title:       fmt.Sprintf("Shopping: %s - Product Search", query),
			URL:         fmt.Sprintf("https://www.google.com/search?tbm=shop&q=%s", url.QueryEscape(query)),
			Summary:     "Shopping and product comparison results",
			Source:      SourceShopping,
			Score:       1.0,
			PublishedAt: time.Now(),
		},
	}
}

func fetchEnergy(ctx context.Context, query string, limit int, timeRange string) []SearchResult {
	return []SearchResult{
		{
			Title:       fmt.Sprintf("[Energy] %s - Oil, Gas, Renewable & Nuclear News", query),
			URL:         fmt.Sprintf("https://www.google.com/search?q=%s+energy+oil+gas+renewable+nuclear", url.QueryEscape(query)),
			Summary:     "Energy markets, commodity prices, and infrastructure developments",
			Source:      SourceEnergy,
			Score:       2.0,
			PublishedAt: time.Now(),
		},
	}
}

func fetchSupplyChain(ctx context.Context, query string, limit int, timeRange string) []SearchResult {
	return []SearchResult{
		{
			Title:       fmt.Sprintf("[SupplyChain] %s - Logistics, Shipping & Semiconductor Supply", query),
			URL:         fmt.Sprintf("https://www.google.com/search?q=%s+supply+chain+shipping+logistics+semiconductor", url.QueryEscape(query)),
			Summary:     "Global supply chain status: shipping rates, port congestion, semiconductor fab updates",
			Source:      SourceSupplyChain,
			Score:       2.0,
			PublishedAt: time.Now(),
		},
	}
}

func fetchGeopolitical(ctx context.Context, query string, limit int, timeRange string) []SearchResult {
	searchURL := fmt.Sprintf("https://hn.algolia.com/api/v1/search?query=%s+geopolitics+war+sanctions&tags=story&hitsPerPage=%d",
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
		results = append(results, SearchResult{
			Title:       "[Geopolitical] " + h.Title,
			URL:         u,
			Source:      SourceGeopolitical,
			Score:       float64(h.Points) + float64(h.Comments)*2,
			Signals:     SignalData{Upvotes: h.Points, Comments: h.Comments},
			PublishedAt: h.CreatedAt,
			Author:      h.Author,
		})
	}
	return results
}

func fetchInfrastructure(ctx context.Context, query string, limit int, timeRange string) []SearchResult {
	searchURL := fmt.Sprintf("https://hn.algolia.com/api/v1/search?query=%s+internet+outage+cloud+datacenter+subsea+cable&tags=story&hitsPerPage=%d",
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
		results = append(results, SearchResult{
			Title:       "[Infra] " + h.Title,
			URL:         u,
			Source:      SourceInfrastructure,
			Score:       float64(h.Points) + float64(h.Comments)*2,
			Signals:     SignalData{Upvotes: h.Points, Comments: h.Comments},
			PublishedAt: h.CreatedAt,
			Author:      h.Author,
		})
	}
	return results
}

func fetchGlobalEvents(ctx context.Context, query string, limit int, timeRange string) []SearchResult {
	searchURL := fmt.Sprintf("https://hn.algolia.com/api/v1/search_by_date?query=%s+breaking+alert+emergency&tags=story&hitsPerPage=%d",
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
		results = append(results, SearchResult{
			Title:       "[Global] " + h.Title,
			URL:         u,
			Source:      SourceGlobalEvents,
			Score:       float64(h.Points) + float64(h.Comments)*3,
			Signals:     SignalData{Upvotes: h.Points, Comments: h.Comments},
			PublishedAt: h.CreatedAt,
			Author:      h.Author,
		})
	}
	return results
}

func fetchAcademic(ctx context.Context, query string, limit int, timeRange string) []SearchResult {
	searchURL := fmt.Sprintf("https://api.semanticscholar.org/graph/v1/paper/search?query=%s&limit=%d&fields=title,authors,year,abstract,citationCount,url",
		url.QueryEscape(query), limit)
	body, err := httpGet(ctx, searchURL, "")
	if err != nil {
		return nil
	}
	var data struct {
		Data []struct {
			Title         string `json:"title"`
			URL           string `json:"url"`
			Abstract      string `json:"abstract"`
			Year          int    `json:"year"`
			CitationCount int    `json:"citationCount"`
			Authors       []struct {
				Name string `json:"name"`
			} `json:"authors"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		return nil
	}
	results := make([]SearchResult, 0, len(data.Data))
	for _, p := range data.Data {
		authorStr := ""
		if len(p.Authors) > 0 {
			authorStr = p.Authors[0].Name
			if len(p.Authors) > 1 {
				authorStr += " et al."
			}
		}
		results = append(results, SearchResult{
			Title:       fmt.Sprintf("[%d] %s", p.Year, p.Title),
			URL:         p.URL,
			Summary:     truncate(p.Abstract, 200),
			Source:      SourceAcademic,
			Score:       float64(p.CitationCount),
			Signals:     SignalData{Upvotes: p.CitationCount},
			PublishedAt: time.Date(p.Year, 1, 1, 0, 0, 0, 0, time.UTC),
			Author:      authorStr,
		})
	}
	return results
}
