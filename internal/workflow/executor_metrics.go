// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​‌‌​​‌‌​​​‌‌​​​​​​‌​​​​‌​​​​​​‌‌‌‌​​‌​​​‌‌‌​‌​​​​‌​‌‌​​‌​‌​​‌‌‌​‌‌‌‌‌‌​​​​​​​​​​​​​​​​​‌​‌​​‌‌​‌‌​​‌‌‌⁠
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

package workflow

import (
	"time"

	"github.com/alib8b8/aflare/internal/metrics"
)

// traceLLMError is the error reconstructed from an LLMStepTrace.ErrText
// when recording per-call metrics. The original error object is not
// preserved in the trace (traces are JSON-serialised, so they carry only
// the error's text), so this type cannot Unwrap the real error — it
// intentionally has no Unwrap method to avoid implying a chain that
// doesn't exist. Carrying StatusCode lets future metrics distinguish
// provider HTTP errors (5xx) from client-side failures (status 0,
// e.g. context cancellation / connection refused) without re-parsing
// the error text.
type traceLLMError struct {
	text       string
	statusCode int
}

func (e *traceLLMError) Error() string { return e.text }

// recordWorkflowMetrics publishes Prometheus metrics for a completed workflow
// run: the overall execution counter/duration and the per-call LLM telemetry
// aggregated in trace.Steps[*].LLM (provider/model/tokens/cost). It is
// lightweight — direct Inc/Observe calls, no goroutine — and safe to call with
// a nil trace (only the workflow counter is updated, with zero duration).
func recordWorkflowMetrics(trace *WorkflowTrace, runErr error) {
	var duration time.Duration
	if trace != nil {
		duration = trace.Duration
		for _, step := range trace.Steps {
			for _, call := range step.LLM {
				var callErr error
				if call.ErrText != "" {
					// Reconstruct a typed error rather than a bare
					// errors.New(string): the typed form carries the
					// HTTP status code and is identifiable as a
					// trace-originated error. metrics.RecordLLMCall
					// currently only checks err != nil, but the typed
					// form means a future metrics evolution (e.g.
					// counting 5xx vs client-side failures separately)
					// can switch on the type without re-stringifying.
					callErr = &traceLLMError{text: call.ErrText, statusCode: call.StatusCode}
				}
				metrics.RecordLLMCall(call.Provider, call.Model, callErr,
					call.PromptTokens, call.CompletionTokens, call.CostUSD)
			}
		}
	}
	metrics.RecordWorkflowExecution(duration, runErr)
}
