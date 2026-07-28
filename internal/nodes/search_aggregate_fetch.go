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

func fetchWeibo(ctx context.Context, query string, limit int, timeRange string) []SearchResult {
	searchURL := fmt.Sprintf("https://m.weibo.cn/api/container/getIndex?containerid=100103type%%3D1%%26q%%3D%s&page_type=searchall",
		url.QueryEscape(query))
	body, err := httpGet(ctx, searchURL, "Mozilla/5.0")
	if err != nil {
		return nil
	}
	var data struct {
		Data struct {
			Cards []struct {
				MBlog struct {
					Text         string `json:"text"`
					ID           string `json:"id"`
					CreatedAt    string `json:"created_at"`
					RepostsCnt   int    `json:"reposts_count"`
					CommentsCnt  int    `json:"comments_count"`
					AttitudesCnt int    `json:"attitudes_count"`
					User         struct {
						ScreenName string `json:"screen_name"`
					} `json:"user"`
				} `json:"mblog"`
			} `json:"cards"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		return nil
	}
	results := make([]SearchResult, 0, len(data.Data.Cards))
	for i, c := range data.Data.Cards {
		if i >= limit {
			break
		}
		mb := c.MBlog
		if mb.ID == "" {
			continue
		}
		cleanText := stripHTML(mb.Text)
		score := float64(mb.AttitudesCnt)*1.0 + float64(mb.CommentsCnt)*2.0 + float64(mb.RepostsCnt)*3.0
		results = append(results, SearchResult{
			Title:   truncate(cleanText, 80),
			URL:     fmt.Sprintf("https://m.weibo.cn/status/%s", mb.ID),
			Summary: truncate(cleanText, 200),
			Source:  SourceWeibo,
			Score:   score,
			Signals: SignalData{
				Upvotes:  mb.AttitudesCnt,
				Comments: mb.CommentsCnt,
				Shares:   mb.RepostsCnt,
			},
			PublishedAt: time.Now(),
			Author:      mb.User.ScreenName,
		})
	}
	return results
}

func stripHTML(s string) string {
	result := s
	for {
		start := strings.Index(result, "<")
		if start == -1 {
			break
		}
		end := strings.Index(result[start:], ">")
		if end == -1 {
			break
		}
		result = result[:start] + result[start+end+1:]
	}
	return strings.TrimSpace(result)
}

func fetchZhihu(ctx context.Context, query string, limit int, timeRange string) []SearchResult {
	searchURL := fmt.Sprintf("https://www.zhihu.com/api/v4/search_v3?t=general&q=%s&limit=%d",
		url.QueryEscape(query), limit)
	body, err := httpGet(ctx, searchURL, "Mozilla/5.0")
	if err != nil {
		return nil
	}
	var data struct {
		Data []struct {
			Object struct {
				Type       string `json:"type"`
				Title      string `json:"title"`
				Excerpt    string `json:"excerpt"`
				URL        string `json:"url"`
				VoteupCnt  int    `json:"voteup_count"`
				CommentCnt int    `json:"comment_count"`
				Author     struct {
					Name string `json:"name"`
				} `json:"author"`
				Created int64 `json:"created"`
			} `json:"object"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		return nil
	}
	results := make([]SearchResult, 0, len(data.Data))
	for _, d := range data.Data {
		o := d.Object
		if o.Title == "" {
			continue
		}
		score := float64(o.VoteupCnt)*1.0 + float64(o.CommentCnt)*2.0
		pubTime := time.Now()
		if o.Created > 0 {
			pubTime = time.Unix(o.Created, 0)
		}
		results = append(results, SearchResult{
			Title:       o.Title,
			URL:         o.URL,
			Summary:     truncate(o.Excerpt, 200),
			Source:      SourceZhihu,
			Score:       score,
			Signals:     SignalData{Upvotes: o.VoteupCnt, Comments: o.CommentCnt},
			PublishedAt: pubTime,
			Author:      o.Author.Name,
		})
	}
	return results
}

func fetchBilibili(ctx context.Context, query string, limit int, timeRange string) []SearchResult {
	searchURL := fmt.Sprintf("https://api.bilibili.com/x/web-interface/search/type?search_type=video&keyword=%s&page_size=%d",
		url.QueryEscape(query), limit)
	body, err := httpGet(ctx, searchURL, "Mozilla/5.0")
	if err != nil {
		return nil
	}
	var data struct {
		Data struct {
			Result []struct {
				Title       string `json:"title"`
				Bvid        string `json:"bvid"`
				Play        int    `json:"play"`
				VideoReview int    `json:"video_review"`
				Favorites   int    `json:"favorites"`
				Duration    string `json:"duration"`
				Author      string `json:"author"`
				Pubdate     int64  `json:"pubdate"`
				Description string `json:"description"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		return nil
	}
	results := make([]SearchResult, 0, len(data.Data.Result))
	for _, v := range data.Data.Result {
		score := float64(v.Play)*0.01 + float64(v.VideoReview)*1.0 + float64(v.Favorites)*2.0
		pubTime := time.Now()
		if v.Pubdate > 0 {
			pubTime = time.Unix(v.Pubdate, 0)
		}
		results = append(results, SearchResult{
			Title:   stripHTML(v.Title),
			URL:     fmt.Sprintf("https://www.bilibili.com/video/%s", v.Bvid),
			Summary: truncate(v.Description, 200),
			Source:  SourceBilibili,
			Score:   score,
			Signals: SignalData{
				Views:    v.Play,
				Comments: v.VideoReview,
				Shares:   v.Favorites,
			},
			PublishedAt: pubTime,
			Author:      v.Author,
		})
	}
	return results
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
