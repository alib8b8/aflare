// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌‌‌‌​​‌‌‌​‌​​‌​​‌‌‌‌​‌​‌​​​​‌​‌‌‌​​​​​​​‌​​‌‌​​​​​​​​​​​​​​​​​​​​​‌​​‌​​‌​‌​​​‌‌⁠
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

// Package metrics defines the Prometheus metrics for aflare and provides
// lightweight recording helpers.
//
// # Design notes
//
// Hot-path metrics (node/workflow executions, security blocks, LLM calls) are
// recorded inline via direct Inc/Observe calls at the execution sites
// (core.ExecuteWithStats, core.RecordBlock, the workflow executor). These are
// monotonic counters / histograms and are updated synchronously — no extra
// goroutines — so they do not affect the main path.
//
// Snapshot metrics (cache hits/misses, node/security snapshot gauges) are
// pulled on demand from the existing internal stats accumulators
// (Registry stats / SecurityStats / CacheStats) by CollectSnapshot, which the
// /metrics handler calls immediately before scraping. To avoid import cycles
// (core and workflow import this package), the snapshot sources are wired via
// provider callbacks registered by the caller (e.g. the WebUI server).
//
// The package only depends on prometheus/client_golang; it does NOT import any
// other internal package, so it can be imported freely from core/workflow/cache.
package metrics

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Metric name constants. Centralised so collectors and tests share one source
// of truth.
const (
	NodeExecutionsName       = "aflare_node_executions_total"
	NodeExecDurationName     = "aflare_node_execution_duration_seconds"
	WorkflowExecutionsName   = "aflare_workflow_executions_total"
	WorkflowExecDurationName = "aflare_workflow_execution_duration_seconds"
	SecurityBlocksName       = "aflare_security_blocks_total"
	CacheHitsName            = "aflare_cache_hits_total"
	CacheMissesName          = "aflare_cache_misses_total"
	LLMCallsName             = "aflare_llm_calls_total"
	LLMTokensName            = "aflare_llm_tokens_total"
	LLMCostName              = "aflare_llm_cost_usd_total"
	NodeCallsGaugeName       = "aflare_node_calls"
	NodeErrorsGaugeName      = "aflare_node_errors"
	SecurityBlocksGaugeName  = "aflare_security_blocks"

	// P2-8: WAL durability / volume observability.
	WALAppendsName        = "aflare_wal_appends_total" // {record_type: delta|snapshot}
	WALAppendDurationName = "aflare_wal_append_duration_seconds"
	WALCompactionsName    = "aflare_wal_compactions_total"
	WALSyncsName          = "aflare_wal_syncs_total"

	// P2-8: DAG dynamic scheduling observability — time a dispatched step
	// waits in the reorder buffer before its in-order processing turn.
	DAGStepReorderWaitName = "aflare_dag_step_reorder_wait_seconds"

	// P2-8: memory hybrid retrieval observability.
	MemorySearchesName       = "aflare_memory_searches_total" // {mode: keyword|hybrid}
	MemorySearchDurationName = "aflare_memory_search_duration_seconds"
	MemorySearchResultsName  = "aflare_memory_search_results"

	// P2-8: LLM request dedup (singleflight) savings.
	LLMSingleflightSharedName = "aflare_llm_singleflight_shared_total"

	// P2-8: ReAct parallel tool-call batches.
	AgentToolBatchSizeName     = "aflare_agent_tool_batch_size"
	AgentToolBatchDurationName = "aflare_agent_tool_batch_duration_seconds"

	// ReAct agent conversation compaction (context budget exceeded).
	AgentContextCompactionsName = "aflare_agent_context_compactions_total"
)

var (
	nodeExecutions = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: NodeExecutionsName,
		Help: "Total number of node executions, by node name and status (success/error).",
	}, []string{"node_name", "status"})

	nodeExecDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    NodeExecDurationName,
		Help:    "Wall-clock duration of a single node execution, in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"node_name"})

	workflowExecutions = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: WorkflowExecutionsName,
		Help: "Total number of workflow executions, by status (success/error).",
	}, []string{"status"})

	workflowExecDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    WorkflowExecDurationName,
		Help:    "Wall-clock duration of a full workflow execution, in seconds.",
		Buckets: prometheus.DefBuckets,
	})

	securityBlocks = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: SecurityBlocksName,
		Help: "Total number of security blocks, by block type (ssrf/path_traversal/command_injection/...).",
	}, []string{"block_type"})

	cacheHits = prometheus.NewCounter(prometheus.CounterOpts{
		Name: CacheHitsName,
		Help: "Total cache hits. Pulled from CacheStats via CollectSnapshot.",
	})

	cacheMisses = prometheus.NewCounter(prometheus.CounterOpts{
		Name: CacheMissesName,
		Help: "Total cache misses. Pulled from CacheStats via CollectSnapshot.",
	})

	llmCalls = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: LLMCallsName,
		Help: "Total number of LLM calls, by provider, model and status (success/error).",
	}, []string{"provider", "model", "status"})

	llmTokens = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: LLMTokensName,
		Help: "Total LLM tokens consumed, by provider, model and type (prompt/completion).",
	}, []string{"provider", "model", "type"})

	llmCost = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: LLMCostName,
		Help: "Total estimated LLM cost in USD, by provider and model.",
	}, []string{"provider", "model"})

	// Snapshot gauges are set by CollectSnapshot from the authoritative
	// internal stats accumulators. They cross-check the direct-Inc counters
	// and give a point-in-time view for pull-based consumers.
	nodeCallsGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: NodeCallsGaugeName,
		Help: "Snapshot of per-node call count from Registry stats (pull-based).",
	}, []string{"node_name"})

	nodeErrorsGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: NodeErrorsGaugeName,
		Help: "Snapshot of per-node error count from Registry stats (pull-based).",
	}, []string{"node_name"})

	securityBlocksGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: SecurityBlocksGaugeName,
		Help: "Snapshot of security blocks by type from SecurityStats (pull-based).",
	}, []string{"block_type"})

	// P2-8: WAL durability / volume metrics.
	walAppends = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: WALAppendsName,
		Help: "Total WAL records appended, by record type (delta | snapshot).",
	}, []string{"record_type"})

	walAppendDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    WALAppendDurationName,
		Help:    "Duration of a WAL append, including fsync when SyncEveryWrite is on, in seconds.",
		Buckets: prometheus.DefBuckets,
	})

	walCompactions = prometheus.NewCounter(prometheus.CounterOpts{
		Name: WALCompactionsName,
		Help: "Total WAL compactions performed.",
	})

	walSyncs = prometheus.NewCounter(prometheus.CounterOpts{
		Name: WALSyncsName,
		Help: "Total WAL fsync operations (explicit Sync, SyncEveryWrite appends, periodic syncer ticks).",
	})

	// P2-8: DAG dynamic scheduling metrics.
	dagStepReorderWait = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    DAGStepReorderWaitName,
		Help:    "Time a dispatched DAG step waits in the reorder buffer before its in-order processing turn, in seconds.",
		Buckets: prometheus.DefBuckets,
	})

	// P2-8: memory hybrid retrieval metrics.
	memorySearches = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: MemorySearchesName,
		Help: "Total memory searches, by mode (keyword = scoring only, hybrid = keyword + vector).",
	}, []string{"mode"})

	memorySearchDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    MemorySearchDurationName,
		Help:    "Duration of a memory search, in seconds.",
		Buckets: prometheus.DefBuckets,
	})

	memorySearchResults = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    MemorySearchResultsName,
		Help:    "Number of entries returned by a memory search.",
		Buckets: prometheus.LinearBuckets(0, 1, 11), // 0..10
	})

	// P2-8: LLM request dedup savings.
	llmSingleflightShared = prometheus.NewCounter(prometheus.CounterOpts{
		Name: LLMSingleflightSharedName,
		Help: "Total LLM requests satisfied by an in-flight identical request (singleflight followers).",
	})

	// P2-8: ReAct parallel tool-call batches.
	agentToolBatchSize = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    AgentToolBatchSizeName,
		Help:    "Number of tool calls in one ReAct tool-call batch (recorded only for batches > 1).",
		Buckets: prometheus.LinearBuckets(2, 1, 9), // 2..10
	})

	agentToolBatchDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    AgentToolBatchDurationName,
		Help:    "Wall-clock duration of one parallel ReAct tool-call batch, in seconds.",
		Buckets: prometheus.DefBuckets,
	})

	// ReAct agent conversation compaction.
	agentContextCompactions = prometheus.NewCounter(prometheus.CounterOpts{
		Name: AgentContextCompactionsName,
		Help: "Total number of times the ReAct agent conversation was compacted due to the context budget being exceeded.",
	})

	registerOnce sync.Once
)

// Register registers all metrics with the default Prometheus registry. It is
// safe to call multiple times; only the first call performs the registration.
func Register() {
	registerOnce.Do(func() {
		prometheus.MustRegister(
			nodeExecutions,
			nodeExecDuration,
			workflowExecutions,
			workflowExecDuration,
			securityBlocks,
			cacheHits,
			cacheMisses,
			llmCalls,
			llmTokens,
			llmCost,
			nodeCallsGauge,
			nodeErrorsGauge,
			securityBlocksGauge,
			walAppends,
			walAppendDuration,
			walCompactions,
			walSyncs,
			dagStepReorderWait,
			memorySearches,
			memorySearchDuration,
			memorySearchResults,
			llmSingleflightShared,
			agentToolBatchSize,
			agentToolBatchDuration,
			agentContextCompactions,
		)
	})
}

// RecordNodeExecution increments the node execution counter and observes the
// execution duration. err is non-nil when the node execution failed. Call this
// inline from the node execution path; it is a couple of atomic ops and does
// not block.
func RecordNodeExecution(name string, duration time.Duration, err error) {
	status := "success"
	if err != nil {
		status = "error"
	}
	nodeExecutions.WithLabelValues(name, status).Inc()
	nodeExecDuration.WithLabelValues(name).Observe(duration.Seconds())
}

// RecordWorkflowExecution increments the workflow execution counter and
// observes the overall workflow duration. err is non-nil when the workflow
// failed.
func RecordWorkflowExecution(duration time.Duration, err error) {
	status := "success"
	if err != nil {
		status = "error"
	}
	workflowExecutions.WithLabelValues(status).Inc()
	workflowExecDuration.Observe(duration.Seconds())
}

// IncSecurityBlock increments the security blocks counter for the given block
// type (e.g. "ssrf", "path_traversal").
func IncSecurityBlock(blockType string) {
	securityBlocks.WithLabelValues(blockType).Inc()
}

// RecordLLMCall records a single LLM call: increments the call counter (by
// provider/model/status) and the token counters (prompt/completion), and adds
// the cost. err is non-nil when the call failed. Zero token counts and cost
// are allowed (providers that omit usage/cost).
func RecordLLMCall(provider, model string, err error, promptTokens, completionTokens int, costUSD float64) {
	status := "success"
	if err != nil {
		status = "error"
	}
	llmCalls.WithLabelValues(provider, model, status).Inc()
	if promptTokens > 0 {
		llmTokens.WithLabelValues(provider, model, "prompt").Add(float64(promptTokens))
	}
	if completionTokens > 0 {
		llmTokens.WithLabelValues(provider, model, "completion").Add(float64(completionTokens))
	}
	if costUSD > 0 {
		llmCost.WithLabelValues(provider, model).Add(costUSD)
	}
}

// --- P2-8: WAL / DAG / memory / dedup / tool-batch metrics ---------------

// RecordWALAppend records one WAL append: the record type (delta/snapshot)
// and the append duration, which includes the fsync when SyncEveryWrite is
// enabled.
func RecordWALAppend(isDelta bool, duration time.Duration) {
	recordType := "snapshot"
	if isDelta {
		recordType = "delta"
	}
	walAppends.WithLabelValues(recordType).Inc()
	walAppendDuration.Observe(duration.Seconds())
}

// IncWALCompaction increments the WAL compaction counter.
func IncWALCompaction() {
	walCompactions.Inc()
}

// IncWALSync increments the WAL fsync counter.
func IncWALSync() {
	walSyncs.Inc()
}

// RecordDAGStepReorderWait observes how long a dispatched DAG step waited
// in the reorder buffer before its in-order processing turn. Sustained high
// values indicate the processing order (not the dependency graph) is the
// scheduling bottleneck.
func RecordDAGStepReorderWait(duration time.Duration) {
	dagStepReorderWait.Observe(duration.Seconds())
}

// RecordMemorySearch records one memory search: mode (keyword = scoring
// only, hybrid = keyword + vector), duration and number of returned
// entries.
func RecordMemorySearch(mode string, duration time.Duration, results int) {
	memorySearches.WithLabelValues(mode).Inc()
	memorySearchDuration.Observe(duration.Seconds())
	memorySearchResults.Observe(float64(results))
}

// IncLLMSingleflightShared increments the counter of LLM requests satisfied
// by an in-flight identical request (singleflight follower).
func IncLLMSingleflightShared() {
	llmSingleflightShared.Inc()
}

// RecordAgentToolBatch records one parallel ReAct tool-call batch (only
// batches with more than one call): the number of calls and the wall-clock
// duration of the whole batch.
func RecordAgentToolBatch(count int, duration time.Duration) {
	agentToolBatchSize.Observe(float64(count))
	agentToolBatchDuration.Observe(duration.Seconds())
}

// IncAgentContextCompactions increments the counter of ReAct agent
// conversation compactions (context budget exceeded).
func IncAgentContextCompactions() {
	agentContextCompactions.Inc()
}

// --- Snapshot collection -------------------------------------------------

// NodeStat is the metrics-package-local projection of a node's execution
// stats, decoupled from core.NodeExecStats to avoid an import cycle.
type NodeStat struct {
	Name   string
	Calls  int64
	Errors int64
}

// A set of optional provider callbacks that feed CollectSnapshot from the
// authoritative internal stats accumulators. They are wired by the caller
// (e.g. the WebUI server) to avoid this package importing core/cache.
var (
	providersMu      sync.RWMutex
	registryProvider func() []NodeStat
	securityProvider func() map[string]int64 // block_type -> count
	cacheProvider    func() (hits, misses int64)
)

// SetRegistryStatsProvider registers a callback returning per-node call/error
// counts. The callback is invoked by CollectSnapshot on each scrape.
func SetRegistryStatsProvider(fn func() []NodeStat) {
	providersMu.Lock()
	registryProvider = fn
	providersMu.Unlock()
}

// SetSecurityStatsProvider registers a callback returning security block
// counts keyed by block type.
func SetSecurityStatsProvider(fn func() map[string]int64) {
	providersMu.Lock()
	securityProvider = fn
	providersMu.Unlock()
}

// SetCacheStatsProvider registers a callback returning cumulative cache
// (hits, misses). Cache counters are delta-updated from these cumulative
// values so they remain monotonic across scrapes.
func SetCacheStatsProvider(fn func() (hits, misses int64)) {
	providersMu.Lock()
	cacheProvider = fn
	lastCacheHits = 0
	lastCacheMisses = 0
	providersMu.Unlock()
}

// lastCache* track the cumulative source values seen on the previous
// CollectSnapshot so the counter only advances by the delta. Guarded by
// collectMu below.
var (
	collectMu       sync.Mutex
	lastCacheHits   int64
	lastCacheMisses int64
)

// CollectSnapshot reads the registered stats providers and updates the
// snapshot gauges (node/security) and the cache counters (delta). It is
// intended to be called by the /metrics handler immediately before scraping so
// the pull-based metrics reflect the latest internal state.
//
// Sources without a registered provider are skipped, so calling this before
// any provider is wired is a safe no-op. The hot-path counters
// (node/workflow/security/llm) are NOT touched here — they are maintained
// inline via the Record*/Inc* helpers — to avoid double counting.
func CollectSnapshot() {
	providersMu.RLock()
	regFn := registryProvider
	secFn := securityProvider
	cacheFn := cacheProvider
	providersMu.RUnlock()

	if regFn != nil {
		for _, s := range regFn() {
			nodeCallsGauge.WithLabelValues(s.Name).Set(float64(s.Calls))
			nodeErrorsGauge.WithLabelValues(s.Name).Set(float64(s.Errors))
		}
	}

	if secFn != nil {
		for blockType, count := range secFn() {
			securityBlocksGauge.WithLabelValues(blockType).Set(float64(count))
		}
	}

	if cacheFn != nil {
		hits, misses := cacheFn()
		collectMu.Lock()
		dHits := hits - lastCacheHits
		dMisses := misses - lastCacheMisses
		// A cache Clear() resets the cumulative counters to 0; clamp the
		// delta so the Prometheus counter never decreases.
		if dHits < 0 {
			dHits = 0
		}
		if dMisses < 0 {
			dMisses = 0
		}
		lastCacheHits = hits
		lastCacheMisses = misses
		collectMu.Unlock()
		if dHits > 0 {
			cacheHits.Add(float64(dHits))
		}
		if dMisses > 0 {
			cacheMisses.Add(float64(dMisses))
		}
	}
}
