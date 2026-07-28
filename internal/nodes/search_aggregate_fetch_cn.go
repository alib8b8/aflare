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
