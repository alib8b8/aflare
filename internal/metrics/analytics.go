// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌‌‌​​​​‌‌‌‌‌​​‌​​​​​‌‌‌‌‌​​​‌​‌​‌‌‌​​‌‌​‌​​​​‌​‌​​​​​​​​​​​​​​​​‌‌‌‌​​​‌​​​​‌‌​‌⁠
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

// analytics.go defines the user-facing product analytics metrics:
//
//  1. aflare_first_session_success_total — first-time user experience rate
//     (>70% target: new users get a valid response within 2 minutes)
//
//  2. aflare_session_turns — session turn count histogram
//     (>5 turns average target: users are engaged, not just trying once)
//
//  3. aflare_template_usage_total — template call counter
//     (top 50 templates coverage >80% target: how many templates are actually used)
//
//  4. aflare_capability_inits_total — capability init counter
//     (tracks how often reflection/memory/etc. are loaded)
//
// These are Prometheus counters/histograms. The hot-path recording is a single
// atomic Inc/Observe call — no blocking, no allocation on the fast path.
package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

// Metric name constants for analytics.
const (
	FirstSessionSuccessName = "aflare_first_session_success_total"
	SessionTurnsName        = "aflare_session_turns"
	TemplateUsageName       = "aflare_template_usage_total"
	CapabilityInitsName     = "aflare_capability_inits_total"
)

var (
	// firstSessionSuccess tracks whether a new user's first session was successful.
	// Labels: provider (ollama/openai/deepseek/...), outcome (success/timeout/error)
	firstSessionSuccess = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: FirstSessionSuccessName,
		Help: "Count of first-time user sessions by provider and outcome (success/timeout/error). " +
			"Used to track first-successful-experience rate (target: >70% within 2 minutes).",
	}, []string{"provider", "outcome"})

	// sessionTurns is a histogram of turns per session.
	// Buckets: 1, 2, 3, 5, 10, 20, 50, 100
	sessionTurns = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: SessionTurnsName,
		Help: "Distribution of turn counts per chat session. " +
			"Used to track engagement (target: average >5 turns).",
		Buckets: []float64{1, 2, 3, 5, 10, 20, 50, 100},
	})

	// templateUsage tracks which templates are called by the agent.
	// Labels: template_name, source (builtin/external)
	templateUsage = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: TemplateUsageName,
		Help: "Count of template executions by template name and source (builtin/external). " +
			"Used to track template usage rate (target: top 50 templates cover >80%).",
	}, []string{"template_name", "source"})

	// capabilityInits tracks how often each capability is initialized.
	// Labels: capability_name
	capabilityInits = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: CapabilityInitsName,
		Help: "Count of capability initializations by capability name. " +
			"Used to track capability adoption (e.g. reflection, memory, bdi).",
	}, []string{"capability_name"})

	analyticsRegisterOnce sync.Once
)

// RegisterAnalytics registers all analytics metrics with the default Prometheus
// registry. Safe to call multiple times; only the first call performs registration.
// This is separate from Register() so consumers can opt in to analytics.
func RegisterAnalytics() {
	analyticsRegisterOnce.Do(func() {
		prometheus.MustRegister(
			firstSessionSuccess,
			sessionTurns,
			templateUsage,
			capabilityInits,
		)
	})
}

// RecordFirstSession records the outcome of a first-time user session.
// outcome should be one of: "success", "timeout", "error"
func RecordFirstSession(provider, outcome string) {
	firstSessionSuccess.WithLabelValues(provider, outcome).Inc()
}

// RecordSessionTurns records the total number of turns in a completed session.
func RecordSessionTurns(turns int) {
	if turns > 0 {
		sessionTurns.Observe(float64(turns))
	}
}

// RecordTemplateUsage records a template execution.
// templateName is the template identifier (e.g. "stock-screener").
// source is "builtin" or "external".
func RecordTemplateUsage(templateName, source string) {
	templateUsage.WithLabelValues(templateName, source).Inc()
}

// RecordCapabilityInit records a capability initialization.
func RecordCapabilityInit(capabilityName string) {
	capabilityInits.WithLabelValues(capabilityName).Inc()
}
