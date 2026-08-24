// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​‌‌‌‌‌​​​​‌​‌‌‌​​​‌​​‌‌‌​‌‌​​‌‌​‌‌​​‌‌​‌​​‌​​‌‌​​​​​​​​​​​​​​​​​‌​‌​‌‌​‌​​‌​‌‌‌‌⁠
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
)

// RouterDecision records the outcome of one LLMRouter.Execute call: which
// providers were considered, in what order they were tried, which one
// succeeded, and why each failed one failed. Published via a context-scoped
// RouterDecisionSink so the workflow executor (B-3) can attach it to the
// step trace alongside the per-call LLM telemetry from B-2.
type RouterDecision struct {
	Strategy   string          // routing strategy used (priority/cost/latency/round_robin/random)
	Candidates []string        // provider names in the order the router tried them
	Selected   string          // provider name that produced the final response; "" if all failed
	Attempts   []RouterAttempt // one entry per provider tried, in attempt order
	FinalError string          // error text if all providers failed; "" on success
}

// RouterAttempt records the outcome of calling one provider within a router
// fallback chain.
type RouterAttempt struct {
	Provider string // provider name
	Success  bool   // whether this provider produced the final response
	Error    string // error text on failure; "" on success
	// LatencyMs is the wall-clock duration of this provider attempt. The
	// per-call LLM telemetry (B-2) carries a more precise Latency; this
	// field is kept for quick at-a-glance reading of the decision without
	// cross-referencing the LLM slice.
	LatencyMs int64
}

// RouterDecisionSink receives RouterDecision records. Implementations must
// be safe for concurrent use: a workflow step may invoke the router, and
// parallel DAG steps may invoke it concurrently.
type RouterDecisionSink interface {
	RecordRouterDecision(d RouterDecision)
}

// noopRouterDecisionSink discards all decisions.
type noopRouterDecisionSink struct{}

func (noopRouterDecisionSink) RecordRouterDecision(RouterDecision) {}

type routerSinkCtxKey struct{}

// WithRouterDecisionSink returns a new context carrying sink. Router nodes
// descended from ctx will publish decisions to sink. Passing nil restores
// the no-op default.
func WithRouterDecisionSink(ctx context.Context, sink RouterDecisionSink) context.Context {
	if sink == nil {
		sink = noopRouterDecisionSink{}
	}
	return context.WithValue(ctx, routerSinkCtxKey{}, sink)
}

// RouterDecisionSinkFrom returns the sink attached to ctx, or a no-op sink
// if none is present. Always non-nil.
func RouterDecisionSinkFrom(ctx context.Context) RouterDecisionSink {
	if s, ok := ctx.Value(routerSinkCtxKey{}).(RouterDecisionSink); ok {
		return s
	}
	return noopRouterDecisionSink{}
}
