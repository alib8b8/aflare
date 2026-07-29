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

package metrics

import (
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestRecordNodeExecution_IncrementsCounters(t *testing.T) {
	const node = "test_node_record"
	beforeSuccess := testutil.ToFloat64(nodeExecutions.WithLabelValues(node, "success"))
	beforeError := testutil.ToFloat64(nodeExecutions.WithLabelValues(node, "error"))

	RecordNodeExecution(node, 10*time.Millisecond, nil)
	RecordNodeExecution(node, 5*time.Millisecond, errors.New("boom"))

	afterSuccess := testutil.ToFloat64(nodeExecutions.WithLabelValues(node, "success"))
	afterError := testutil.ToFloat64(nodeExecutions.WithLabelValues(node, "error"))

	if afterSuccess != beforeSuccess+1 {
		t.Errorf("success counter: expected +%d, got %v -> %v", 1, beforeSuccess, afterSuccess)
	}
	if afterError != beforeError+1 {
		t.Errorf("error counter: expected +%d, got %v -> %v", 1, beforeError, afterError)
	}
}

func TestRecordNodeExecution_ObservesDuration(t *testing.T) {
	const node = "test_node_duration"
	// Observing under a fresh label value creates a new histogram series, so
	// the collected sample count (buckets + sum + count) must increase.
	before := testutil.CollectAndCount(nodeExecDuration)
	RecordNodeExecution(node, 42*time.Millisecond, nil)
	after := testutil.CollectAndCount(nodeExecDuration)
	if after <= before {
		t.Errorf("expected histogram sample count to increase, got %d -> %d", before, after)
	}
}

func TestRecordWorkflowExecution(t *testing.T) {
	beforeSuccess := testutil.ToFloat64(workflowExecutions.WithLabelValues("success"))
	beforeError := testutil.ToFloat64(workflowExecutions.WithLabelValues("error"))

	RecordWorkflowExecution(100*time.Millisecond, nil)
	RecordWorkflowExecution(50*time.Millisecond, errors.New("fail"))

	if got := testutil.ToFloat64(workflowExecutions.WithLabelValues("success")); got != beforeSuccess+1 {
		t.Errorf("workflow success: expected %v, got %v", beforeSuccess+1, got)
	}
	if got := testutil.ToFloat64(workflowExecutions.WithLabelValues("error")); got != beforeError+1 {
		t.Errorf("workflow error: expected %v, got %v", beforeError+1, got)
	}
}

func TestIncSecurityBlock(t *testing.T) {
	const bt = "ssrf"
	before := testutil.ToFloat64(securityBlocks.WithLabelValues(bt))
	IncSecurityBlock(bt)
	IncSecurityBlock(bt)
	if got := testutil.ToFloat64(securityBlocks.WithLabelValues(bt)); got != before+2 {
		t.Errorf("expected %v, got %v", before+2, got)
	}
}

func TestRecordLLMCall(t *testing.T) {
	const provider, model = "openai", "gpt-4"
	beforeCalls := testutil.ToFloat64(llmCalls.WithLabelValues(provider, model, "success"))
	beforePrompt := testutil.ToFloat64(llmTokens.WithLabelValues(provider, model, "prompt"))
	beforeCompletion := testutil.ToFloat64(llmTokens.WithLabelValues(provider, model, "completion"))
	beforeCost := testutil.ToFloat64(llmCost.WithLabelValues(provider, model))

	RecordLLMCall(provider, model, nil, 100, 50, 0.012)

	if got := testutil.ToFloat64(llmCalls.WithLabelValues(provider, model, "success")); got != beforeCalls+1 {
		t.Errorf("llm calls: expected %v, got %v", beforeCalls+1, got)
	}
	if got := testutil.ToFloat64(llmTokens.WithLabelValues(provider, model, "prompt")); got != beforePrompt+100 {
		t.Errorf("prompt tokens: expected %v, got %v", beforePrompt+100, got)
	}
	if got := testutil.ToFloat64(llmTokens.WithLabelValues(provider, model, "completion")); got != beforeCompletion+50 {
		t.Errorf("completion tokens: expected %v, got %v", beforeCompletion+50, got)
	}
	if got := testutil.ToFloat64(llmCost.WithLabelValues(provider, model)); got != beforeCost+0.012 {
		t.Errorf("cost: expected %v, got %v", beforeCost+0.012, got)
	}
}

func TestRecordLLMCall_ErrorStatus(t *testing.T) {
	const provider, model = "failprov", "m"
	before := testutil.ToFloat64(llmCalls.WithLabelValues(provider, model, "error"))
	RecordLLMCall(provider, model, errors.New("timeout"), 0, 0, 0)
	if got := testutil.ToFloat64(llmCalls.WithLabelValues(provider, model, "error")); got != before+1 {
		t.Errorf("error calls: expected %v, got %v", before+1, got)
	}
}

func TestRegister_Idempotent(t *testing.T) {
	// Register must be safe to call multiple times (sync.Once); calling it
	// again should not panic.
	Register()
	Register()
}

func TestCollectSnapshot_CacheDelta(t *testing.T) {
	SetCacheStatsProvider(func() (int64, int64) { return 7, 4 })
	beforeHits := testutil.ToFloat64(cacheHits)
	beforeMisses := testutil.ToFloat64(cacheMisses)

	CollectSnapshot()

	if got := testutil.ToFloat64(cacheHits); got != beforeHits+7 {
		t.Errorf("cache hits delta: expected %v, got %v", beforeHits+7, got)
	}
	if got := testutil.ToFloat64(cacheMisses); got != beforeMisses+4 {
		t.Errorf("cache misses delta: expected %v, got %v", beforeMisses+4, got)
	}

	// Second scrape with same cumulative values: no additional delta.
	CollectSnapshot()
	if got := testutil.ToFloat64(cacheHits); got != beforeHits+7 {
		t.Errorf("cache hits should not increase on second scrape: got %v", got)
	}
}

func TestCollectSnapshot_CacheClampOnReset(t *testing.T) {
	// Simulate a cache Clear(): cumulative drops back to 0. The delta must be
	// clamped to 0 so the Prometheus counter never decreases.
	SetCacheStatsProvider(func() (int64, int64) { return 0, 0 })
	beforeHits := testutil.ToFloat64(cacheHits)
	CollectSnapshot()
	if got := testutil.ToFloat64(cacheHits); got != beforeHits {
		t.Errorf("cache hits should not decrease on reset: got %v (before %v)", got, beforeHits)
	}
}

func TestCollectSnapshot_RegistryAndSecurityGauges(t *testing.T) {
	SetRegistryStatsProvider(func() []NodeStat {
		return []NodeStat{{Name: "gauge_node", Calls: 30, Errors: 5}}
	})
	SetSecurityStatsProvider(func() map[string]int64 {
		return map[string]int64{"path_traversal": 9}
	})

	CollectSnapshot()

	if got := testutil.ToFloat64(nodeCallsGauge.WithLabelValues("gauge_node")); got != 30 {
		t.Errorf("node calls gauge: expected 30, got %v", got)
	}
	if got := testutil.ToFloat64(nodeErrorsGauge.WithLabelValues("gauge_node")); got != 5 {
		t.Errorf("node errors gauge: expected 5, got %v", got)
	}
	if got := testutil.ToFloat64(securityBlocksGauge.WithLabelValues("path_traversal")); got != 9 {
		t.Errorf("security blocks gauge: expected 9, got %v", got)
	}
}

func TestCollectSnapshot_NoProvidersIsNoOp(t *testing.T) {
	// With no providers registered, CollectSnapshot must be a safe no-op.
	SetRegistryStatsProvider(nil)
	SetSecurityStatsProvider(nil)
	SetCacheStatsProvider(nil)
	CollectSnapshot() // should not panic
}
