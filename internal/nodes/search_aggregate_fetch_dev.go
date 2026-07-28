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
	"strings"
	"time"
)

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

func fetchNews(ctx context.Context, query string, limit int, timeRange string) []SearchResult {
	searchURL := fmt.Sprintf("https://hn.algolia.com/api/v1/search_by_date?query=%s&tags=story&hitsPerPage=%d",
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
			Title:       "[News] " + h.Title,
			URL:         u,
			Source:      SourceNews,
			Score:       float64(h.Points) + float64(h.Comments)*2,
			Signals:     SignalData{Upvotes: h.Points, Comments: h.Comments},
			PublishedAt: h.CreatedAt,
			Author:      h.Author,
		})
	}
	return results
}
