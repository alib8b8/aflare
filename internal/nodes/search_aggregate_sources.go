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
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

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
