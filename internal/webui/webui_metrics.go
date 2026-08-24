// Copyright (c) 2026 aflare Contributors
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

package webui

import (
	"sync"
	"time"

	"github.com/alib8b8/aflare/internal/metrics"
	"github.com/alib8b8/aflare/internal/nodes/core"
)

// registerMetricsProviders wires the snapshot stats providers used by
// metrics.CollectSnapshot to the authoritative global singletons (the node
// Registry stats and the SecurityStats). Cache stats are only registered when
// the application holds a Cache instance; there is no global cache, so cache
// counters stay at zero until one is wired via metrics.SetCacheStatsProvider.
func registerMetricsProviders() {
	metrics.SetRegistryStatsProvider(func() []metrics.NodeStat {
		stats := core.GetGlobalRegistry().GetAllStats()
		out := make([]metrics.NodeStat, 0, len(stats))
		for name, s := range stats {
			out = append(out, metrics.NodeStat{
				Name:   name,
				Calls:  s.Calls,
				Errors: s.Errors,
			})
		}
		return out
	})
	metrics.SetSecurityStatsProvider(func() map[string]int64 {
		snap := core.GetSecurityStats().Snapshot()
		out := make(map[string]int64, len(snap.ByType))
		for t, c := range snap.ByType {
			out[string(t)] = c
		}
		return out
	})
}

// metricsRateLimiter is a mutex-guarded token bucket rate limiter for the
// /metrics endpoint. It refills on demand (no background goroutine) at rps
// tokens/second up to rps burst. allow() consumes one token and reports
// whether the request may proceed.
type metricsRateLimiter struct {
	mu       sync.Mutex
	tokens   float64
	max      float64
	rps      float64
	lastTime time.Time
	now      func() time.Time
}

// newMetricsRateLimiter returns a token bucket allowing up to rps requests per
// second with a burst of rps.
func newMetricsRateLimiter(rps int) *metricsRateLimiter {
	if rps <= 0 {
		rps = 1
	}
	return &metricsRateLimiter{
		tokens:   float64(rps),
		max:      float64(rps),
		rps:      float64(rps),
		now:      time.Now,
		lastTime: time.Now(),
	}
}

// allow reports whether a request may proceed, consuming one token if so.
func (rl *metricsRateLimiter) allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := rl.now()
	elapsed := now.Sub(rl.lastTime).Seconds()
	rl.lastTime = now
	rl.tokens += elapsed * rl.rps
	if rl.tokens > rl.max {
		rl.tokens = rl.max
	}
	if rl.tokens < 1 {
		return false
	}
	rl.tokens--
	return true
}
