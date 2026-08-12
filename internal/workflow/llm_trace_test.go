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

package workflow

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/alib8b8/aflare/internal/nodes"
	"github.com/alib8b8/aflare/internal/nodes/core"
)

// traceLLMNode wraps a real OpenAICompatibleNode pointed at a mock server so
// we can exercise the full B-2 telemetry pipeline (LLM node → context sink →
// StepTrace.LLM) without depending on a real provider.
type traceLLMNode struct {
	compat *core.OpenAICompatibleNode
}

func newTraceLLMNode(endpoint string) *traceLLMNode {
	return &traceLLMNode{
		compat: core.NewOpenAICompatibleNode(core.LLMNodeConfig{
			Name:            "tracellm",
			DefaultModel:    "test-model",
			DefaultEndpoint: endpoint,
			EnvAPIKey:       "AFLARE_TRACE_TEST_API_KEY_NEVER_SET",
			ProviderName:    "TraceTestProvider",
		}),
	}
}

func (n *traceLLMNode) Name() string        { return "tracellm" }
func (n *traceLLMNode) Description() string { return "trace test LLM node" }
func (n *traceLLMNode) Schema() nodes.NodeSchema {
	return nodes.NodeSchema{Name: "tracellm", Input: "string", Output: "string"}
}
func (n *traceLLMNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	if params == nil {
		params = map[string]string{}
	}
	params["api_key"] = "sk-trace-test"
	return n.compat.Execute(ctx, input, params)
}

// mockLLMWithUsage returns a server that echoes a fixed usage block so we can
// assert token accounting flows through to StepTrace.LLM.
func mockLLMWithUsage(t *testing.T, content string, usage *core.LLMUsage) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		resp := core.LLMResponse{
			Choices: []core.LLMChoice{{Message: core.LLMChoiceMessage{Content: content}}},
			Usage:   usage,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func TestLLMTelemetry_SequentialStepCaptured(t *testing.T) {
	usage := &core.LLMUsage{PromptTokens: 12, CompletionTokens: 8, TotalTokens: 20}
	srv := mockLLMWithUsage(t, "answer", usage)
	defer srv.Close()

	reg := nodes.NewRegistry()
	reg.Register(newTraceLLMNode(srv.URL))

	wf := &Workflow{
		Name: "llm-trace-seq",
		Steps: []WorkflowStep{
			{Node: "tracellm"},
		},
	}

	out, _, trace, err := ExecuteWorkflowWithTrace(context.Background(), wf, reg, nil)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out != "answer" {
		t.Fatalf("output = %q want answer", out)
	}

	if len(trace.Steps) != 1 {
		t.Fatalf("expected 1 step trace, got %d", len(trace.Steps))
	}
	st := trace.Steps[0]
	if len(st.LLM) != 1 {
		t.Fatalf("expected 1 LLM call, got %d", len(st.LLM))
	}
	c := st.LLM[0]
	if c.Model != "test-model" {
		t.Errorf("Model=%q want test-model", c.Model)
	}
	if c.Provider != "TraceTestProvider" {
		t.Errorf("Provider=%q", c.Provider)
	}
	if c.NodeName != "tracellm" {
		t.Errorf("NodeName=%q", c.NodeName)
	}
	if c.Attempt != 1 {
		t.Errorf("Attempt=%d want 1", c.Attempt)
	}
	if c.StatusCode != http.StatusOK {
		t.Errorf("StatusCode=%d want 200", c.StatusCode)
	}
	if c.PromptTokens != 12 || c.CompletionTokens != 8 || c.TotalTokens != 20 {
		t.Errorf("tokens: prompt=%d completion=%d total=%d", c.PromptTokens, c.CompletionTokens, c.TotalTokens)
	}
	if c.ErrText != "" {
		t.Errorf("ErrText=%q want empty", c.ErrText)
	}
	if c.Latency <= 0 {
		t.Error("Latency should be positive")
	}
}

func TestLLMTelemetry_RetriesProduceMultipleCalls(t *testing.T) {
	// First call fails (500), second succeeds. We expect 2 LLM entries
	// in StepTrace.LLM, with the first carrying StatusCode 500 + ErrText.
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		_, _ = io.ReadAll(r.Body)
		if n == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(core.LLMResponse{
				Error: &struct {
					Message string `json:"message"`
				}{Message: "transient"},
			})
			return
		}
		resp := core.LLMResponse{
			Choices: []core.LLMChoice{{Message: core.LLMChoiceMessage{Content: "ok"}}},
			Usage:   &core.LLMUsage{PromptTokens: 5, CompletionTokens: 3, TotalTokens: 8},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	reg := nodes.NewRegistry()
	reg.Register(newTraceLLMNode(srv.URL))

	wf := &Workflow{
		Name: "llm-trace-retry",
		Steps: []WorkflowStep{
			{Node: "tracellm", Retry: 1, Delay: "1ms"},
		},
	}

	_, _, trace, err := ExecuteWorkflowWithTrace(context.Background(), wf, reg, nil)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	st := trace.Steps[0]
	if len(st.LLM) != 2 {
		t.Fatalf("expected 2 LLM calls (1 fail + 1 success), got %d", len(st.LLM))
	}
	if st.LLM[0].StatusCode != http.StatusInternalServerError {
		t.Errorf("first call StatusCode=%d want 500", st.LLM[0].StatusCode)
	}
	if !strings.Contains(st.LLM[0].ErrText, "transient") {
		t.Errorf("first call ErrText=%q should mention transient", st.LLM[0].ErrText)
	}
	if st.LLM[1].StatusCode != http.StatusOK {
		t.Errorf("second call StatusCode=%d want 200", st.LLM[1].StatusCode)
	}
	if st.LLM[1].PromptTokens != 5 {
		t.Errorf("second call PromptTokens=%d want 5", st.LLM[1].PromptTokens)
	}
	// Attempt indices are 1-based in call order.
	if st.LLM[0].Attempt != 1 || st.LLM[1].Attempt != 2 {
		t.Errorf("Attempt indices: %d, %d want 1, 2", st.LLM[0].Attempt, st.LLM[1].Attempt)
	}
}

func TestLLMTelemetry_DAGStepCaptured(t *testing.T) {
	usage := &core.LLMUsage{PromptTokens: 7, CompletionTokens: 4, TotalTokens: 11}
	srv := mockLLMWithUsage(t, "dag-answer", usage)
	defer srv.Close()

	reg := nodes.NewRegistry()
	reg.Register(newTraceLLMNode(srv.URL))

	wf := &Workflow{
		Name: "llm-trace-dag",
		Steps: []WorkflowStep{
			{Node: "tracellm", Name: "a"},
			{Node: "tracellm", Name: "b", DependsOn: []string{"a"}},
		},
	}

	_, _, trace, err := ExecuteWorkflowWithTrace(context.Background(), wf, reg, nil)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(trace.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(trace.Steps))
	}
	for i, st := range trace.Steps {
		if len(st.LLM) != 1 {
			t.Fatalf("step %d: expected 1 LLM call, got %d", i, len(st.LLM))
		}
		if st.LLM[0].TotalTokens != 11 {
			t.Errorf("step %d: TotalTokens=%d want 11", i, st.LLM[0].TotalTokens)
		}
	}
}

func TestLLMTelemetry_NonLLMStepHasNilLLM(t *testing.T) {
	// A plain non-LLM test node should not publish any telemetry; StepTrace.LLM
	// must be nil (not empty slice) so consumers can cheaply detect LLM steps.
	reg := nodes.NewRegistry()
	reg.Register(&testNode{name: "plain"})

	wf := &Workflow{
		Steps: []WorkflowStep{{Node: "plain"}},
	}
	_, _, trace, err := ExecuteWorkflowWithTrace(context.Background(), wf, reg, nil)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if trace.Steps[0].LLM != nil {
		t.Errorf("non-LLM step should have nil LLM, got %+v", trace.Steps[0].LLM)
	}
}

func TestLLMTelemetry_StepResultTraceSharesLLM(t *testing.T) {
	// StepResult.Trace points at the same StepTrace in WorkflowTrace.Steps,
	// so the LLM slice must be readable from either. This guards against a
	// regression where the collector drains into a copy rather than the
	// stored trace entry.
	usage := &core.LLMUsage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2}
	srv := mockLLMWithUsage(t, "shared", usage)
	defer srv.Close()

	reg := nodes.NewRegistry()
	reg.Register(newTraceLLMNode(srv.URL))

	wf := &Workflow{Steps: []WorkflowStep{{Node: "tracellm"}}}
	_, results, trace, err := ExecuteWorkflowWithTrace(context.Background(), wf, reg, nil)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(results) != 1 || results[0].Trace == nil {
		t.Fatalf("expected 1 result with trace, got %+v", results)
	}
	if len(results[0].Trace.LLM) != 1 {
		t.Errorf("StepResult.Trace.LLM has %d entries, want 1", len(results[0].Trace.LLM))
	}
	if len(trace.Steps[0].LLM) != 1 {
		t.Errorf("WorkflowTrace.Steps[0].LLM has %d entries, want 1", len(trace.Steps[0].LLM))
	}
	// Same underlying slice (pointer equality) confirms no copy.
	if &results[0].Trace.LLM[0] != &trace.Steps[0].LLM[0] {
		t.Error("StepResult.Trace.LLM and WorkflowTrace.Steps[0].LLM should share storage")
	}
}
