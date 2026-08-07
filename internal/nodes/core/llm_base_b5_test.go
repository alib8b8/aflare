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

// B-5: benchmark + equivalence tests for the LLM client.
//
// These tests pin down the OpenAI-compatible client behaviour across the
// shape variations real providers (DeepSeek, Qwen, GLM, Kimi, etc.) emit,
// and benchmark the B-2 telemetry pipeline overhead so we can detect
// regressions in the hot path.

package core

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// --- Test scaffolding ---

// recordingSink is a thread-safe LLMCallSink used to capture telemetry
// in equivalence tests.
type recordingSink struct {
	mu    sync.Mutex
	calls []LLMCallTelemetry
}

func (s *recordingSink) RecordLLMCall(t LLMCallTelemetry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, t)
}

func (s *recordingSink) snapshot() []LLMCallTelemetry {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]LLMCallTelemetry(nil), s.calls...)
}

// providerShape is the response shape a mock provider returns. We vary
// it across providers to verify the client tolerates real-world shape
// differences (extra fields, snake_case, missing usage, etc.).
type providerShape struct {
	body    string
	status  int
	headers map[string]string
}

// makeProviderServer returns a mock server returning the given shape for
// every request. The server also echoes back the request body via the
// captured callback so tests can assert what the client sent.
func makeProviderServer(t *testing.T, shape providerShape, captured *[]byte) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if captured != nil {
			*captured = append((*captured)[:0], body...)
		}
		for k, v := range shape.headers {
			w.Header().Set(k, v)
		}
		if shape.status != 0 {
			w.WriteHeader(shape.status)
		} else {
			w.WriteHeader(http.StatusOK)
		}
		_, _ = w.Write([]byte(shape.body))
	}))
}

// --- Equivalence: same content, different provider response shapes ---

// TestLLMClient_MultiProviderShapesEquivalence verifies that the client
// extracts the same content & usage from the varied response shapes that
// real providers emit. All shapes here carry the same logical content
// ("hello") and the same usage (10/5/15), but differ in extraneous fields.
func TestLLMClient_MultiProviderShapesEquivalence(t *testing.T) {
	shapes := []struct {
		name  string
		shape providerShape
	}{
		{
			name: "openai_minimal",
			shape: providerShape{
				body: `{"id":"chatcmpl-1","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`,
			},
		},
		{
			name: "deepseek_with_extra_top_level",
			shape: providerShape{
				body: `{"id":"xxx","object":"chat.completion","created":1700000000,"model":"deepseek-chat","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15},"system_fingerprint":"fp_abc"}`,
			},
		},
		{
			name: "qwen_logprobs_present",
			shape: providerShape{
				body: `{"choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop","logprobs":null}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`,
			},
		},
		{
			name: "glm_no_id_field",
			shape: providerShape{
				body: `{"choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`,
			},
		},
		{
			name: "kimi_with_extra_choices",
			// Some providers echo back extra empty choices; we always
			// read choices[0], so this should still work.
			shape: providerShape{
				body: `{"choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"},{"index":1,"message":{"role":"assistant","content":""},"finish_reason":"length"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`,
			},
		},
	}

	for _, tc := range shapes {
		t.Run(tc.name, func(t *testing.T) {
			var captured []byte
			srv := makeProviderServer(t, tc.shape, &captured)
			defer srv.Close()

			node := NewOpenAICompatibleNode(LLMNodeConfig{
				Name:            "test_" + tc.name,
				DefaultModel:    "test-model",
				DefaultEndpoint: srv.URL,
				EnvAPIKey:       "LLMBOX_B5_NEVER_SET",
				ProviderName:    tc.name,
			})

			sink := &recordingSink{}
			ctx := WithLLMCallSink(context.Background(), sink)

			out, err := node.Execute(ctx, "hi", map[string]string{
				"api_key": "sk-test",
			})
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if out != "hello" {
				t.Errorf("content=%q want hello", out)
			}

			calls := sink.snapshot()
			if len(calls) != 1 {
				t.Fatalf("expected 1 telemetry call, got %d", len(calls))
			}
			c := calls[0]
			if c.StatusCode != http.StatusOK {
				t.Errorf("StatusCode=%d want 200", c.StatusCode)
			}
			if c.ErrText != "" {
				t.Errorf("ErrText=%q want empty", c.ErrText)
			}
			if c.Usage == nil {
				t.Fatal("Usage is nil")
			}
			if c.Usage.PromptTokens != 10 || c.Usage.CompletionTokens != 5 || c.Usage.TotalTokens != 15 {
				t.Errorf("Usage=%+v want 10/5/15", c.Usage)
			}
			if c.Provider != tc.name {
				t.Errorf("Provider=%q want %q", c.Provider, tc.name)
			}
			if c.Model != "test-model" {
				t.Errorf("Model=%q want test-model", c.Model)
			}
			if c.Latency <= 0 {
				t.Error("Latency should be positive")
			}
		})
	}
}

// TestLLMClient_MissingUsageDoesntPanic verifies that a provider response
// without a usage block produces nil Usage in telemetry and a successful
// content extraction (older / cheaper providers omit usage).
func TestLLMClient_MissingUsageDoesntPanic(t *testing.T) {
	shape := providerShape{
		body: `{"choices":[{"message":{"content":"ok"}}]}`,
	}
	var captured []byte
	srv := makeProviderServer(t, shape, &captured)
	defer srv.Close()

	node := NewOpenAICompatibleNode(LLMNodeConfig{
		Name:            "no_usage",
		DefaultModel:    "m",
		DefaultEndpoint: srv.URL,
		EnvAPIKey:       "LLMBOX_B5_NEVER_SET2",
		ProviderName:    "noUsage",
	})
	sink := &recordingSink{}
	ctx := WithLLMCallSink(context.Background(), sink)

	out, err := node.Execute(ctx, "hi", map[string]string{"api_key": "sk-test"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out != "ok" {
		t.Errorf("out=%q want ok", out)
	}
	calls := sink.snapshot()
	if len(calls) != 1 || calls[0].Usage != nil {
		t.Errorf("expected nil Usage, got %+v", calls)
	}
}

// TestLLMClient_TokenCountingPropagates verifies that token counts from
// the provider flow through to telemetry accurately, including the
// case where total_tokens is omitted (must not be inferred).
func TestLLMClient_TokenCountingPropagates(t *testing.T) {
	cases := []struct {
		name        string
		body        string
		wantPrompt  int
		wantCompl   int
		wantTotal   int
		wantTotalOK bool // whether total_tokens is expected to be set
	}{
		{
			name:        "all_three",
			body:        `{"choices":[{"message":{"content":"x"}}],"usage":{"prompt_tokens":7,"completion_tokens":3,"total_tokens":10}}`,
			wantPrompt:  7,
			wantCompl:   3,
			wantTotal:   10,
			wantTotalOK: true,
		},
		{
			name:        "no_total",
			body:        `{"choices":[{"message":{"content":"x"}}],"usage":{"prompt_tokens":7,"completion_tokens":3}}`,
			wantPrompt:  7,
			wantCompl:   3,
			wantTotal:   0,
			wantTotalOK: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var captured []byte
			srv := makeProviderServer(t, providerShape{body: tc.body}, &captured)
			defer srv.Close()

			node := NewOpenAICompatibleNode(LLMNodeConfig{
				Name:            "tok_" + tc.name,
				DefaultModel:    "m",
				DefaultEndpoint: srv.URL,
				EnvAPIKey:       "LLMBOX_B5_NEVER_SET3",
				ProviderName:    "tok",
			})
			sink := &recordingSink{}
			ctx := WithLLMCallSink(context.Background(), sink)

			_, err := node.Execute(ctx, "hi", map[string]string{"api_key": "sk-test"})
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			c := sink.snapshot()[0]
			if c.Usage == nil {
				t.Fatal("nil Usage")
			}
			if c.Usage.PromptTokens != tc.wantPrompt {
				t.Errorf("PromptTokens=%d want %d", c.Usage.PromptTokens, tc.wantPrompt)
			}
			if c.Usage.CompletionTokens != tc.wantCompl {
				t.Errorf("CompletionTokens=%d want %d", c.Usage.CompletionTokens, tc.wantCompl)
			}
			if tc.wantTotalOK {
				if c.Usage.TotalTokens != tc.wantTotal {
					t.Errorf("TotalTokens=%d want %d", c.Usage.TotalTokens, tc.wantTotal)
				}
			} else {
				if c.Usage.TotalTokens != 0 {
					t.Errorf("TotalTokens=%d want 0 when provider omitted it", c.Usage.TotalTokens)
				}
			}
		})
	}
}

// TestLLMClient_RequestShapeSentToProvider verifies the client sends the
// expected JSON request body, including all B-1 fields when supplied.
// This is the "client-side equivalence" check: regardless of provider,
// the request we send must be OpenAI-shaped.
func TestLLMClient_RequestShapeSentToProvider(t *testing.T) {
	var captured []byte
	srv := makeProviderServer(t, providerShape{
		body: `{"choices":[{"message":{"content":"ok"}}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`,
	}, &captured)
	defer srv.Close()

	node := NewOpenAICompatibleNode(LLMNodeConfig{
		Name:            "req_shape",
		DefaultModel:    "default-model",
		DefaultEndpoint: srv.URL,
		EnvAPIKey:       "LLMBOX_B5_NEVER_SET4",
		ProviderName:    "reqshape",
	})

	_, err := node.Execute(context.Background(), "hello", map[string]string{
		"api_key":           "sk-test",
		"model":             "explicit-model",
		"temperature":       "0.4",
		"max_tokens":        "128",
		"top_p":             "0.9",
		"frequency_penalty": "0.1",
		"presence_penalty":  "0.2",
		"stop":              "END,STOP",
		"seed":              "7",
		"response_format":   "json_object",
		"user":              "u-1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var sent LLMRequest
	if err := json.Unmarshal(captured, &sent); err != nil {
		t.Fatalf("captured body is not LLMRequest JSON: %v (body=%s)", err, captured)
	}
	if sent.Model != "explicit-model" {
		t.Errorf("Model=%q want explicit-model", sent.Model)
	}
	if len(sent.Messages) != 1 || sent.Messages[0].Content != "hello" {
		t.Errorf("Messages=%+v", sent.Messages)
	}
	if sent.Temperature != 0.4 {
		t.Errorf("Temperature=%g want 0.4", sent.Temperature)
	}
	if sent.MaxTokens != 128 {
		t.Errorf("MaxTokens=%d want 128", sent.MaxTokens)
	}
	if sent.TopP != 0.9 {
		t.Errorf("TopP=%g", sent.TopP)
	}
	if sent.FrequencyPenalty != 0.1 {
		t.Errorf("FrequencyPenalty=%g", sent.FrequencyPenalty)
	}
	if sent.PresencePenalty != 0.2 {
		t.Errorf("PresencePenalty=%g", sent.PresencePenalty)
	}
	if len(sent.Stop) != 2 || sent.Stop[0] != "END" || sent.Stop[1] != "STOP" {
		t.Errorf("Stop=%v", sent.Stop)
	}
	if sent.Seed == nil || *sent.Seed != 7 {
		t.Errorf("Seed=%v", sent.Seed)
	}
	if sent.ResponseFormat == nil || sent.ResponseFormat.Type != "json_object" {
		t.Errorf("ResponseFormat=%+v", sent.ResponseFormat)
	}
	if sent.User != "u-1" {
		t.Errorf("User=%q", sent.User)
	}
	if sent.Stream != false {
		t.Errorf("Stream=%v want false", sent.Stream)
	}
}

// TestLLMClient_ErrorResponsePropagates verifies that a non-200 response
// produces a non-empty ErrText in telemetry and the call still gets
// recorded (so callers see the failure in the trace).
func TestLLMClient_ErrorResponsePropagates(t *testing.T) {
	shape := providerShape{
		status: http.StatusTooManyRequests,
		body:   `{"error":{"message":"rate limited"}}`,
	}
	srv := makeProviderServer(t, shape, nil)
	defer srv.Close()

	node := NewOpenAICompatibleNode(LLMNodeConfig{
		Name:            "err",
		DefaultModel:    "m",
		DefaultEndpoint: srv.URL,
		EnvAPIKey:       "LLMBOX_B5_NEVER_SET5",
		ProviderName:    "err",
	})
	sink := &recordingSink{}
	ctx := WithLLMCallSink(context.Background(), sink)

	_, err := node.Execute(ctx, "hi", map[string]string{"api_key": "sk-test"})
	if err == nil {
		t.Fatal("expected error from 429")
	}
	calls := sink.snapshot()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call record even on error, got %d", len(calls))
	}
	c := calls[0]
	if c.StatusCode != http.StatusTooManyRequests {
		t.Errorf("StatusCode=%d want 429", c.StatusCode)
	}
	if !strings.Contains(c.ErrText, "rate limited") {
		t.Errorf("ErrText=%q should mention rate limited", c.ErrText)
	}
}

// TestLLMClient_TelemetryRecordsLatency verifies that the recorded
// Latency is positive and reflects the actual server round-trip.
func TestLLMClient_TelemetryRecordsLatency(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(20 * time.Millisecond)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"slow"}}]}`))
	}))
	defer srv.Close()

	node := NewOpenAICompatibleNode(LLMNodeConfig{
		Name:            "slow",
		DefaultModel:    "m",
		DefaultEndpoint: srv.URL,
		EnvAPIKey:       "LLMBOX_B5_NEVER_SET6",
		ProviderName:    "slow",
	})
	sink := &recordingSink{}
	ctx := WithLLMCallSink(context.Background(), sink)

	_, err := node.Execute(ctx, "hi", map[string]string{"api_key": "sk-test"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	c := sink.snapshot()[0]
	if c.Latency < 15*time.Millisecond {
		t.Errorf("Latency=%v want >=15ms (server slept 20ms)", c.Latency)
	}
}

// --- Benchmarks ---

// BenchmarkLLMClient_Execute_NoTelemetry measures the overhead of a
// successful LLM call when no sink is attached (noop default). This is
// the baseline cost of the request/response path.
func BenchmarkLLMClient_Execute_NoTelemetry(b *testing.B) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"hi"}}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
	}))
	defer srv.Close()

	node := NewOpenAICompatibleNode(LLMNodeConfig{
		Name:            "bench",
		DefaultModel:    "m",
		DefaultEndpoint: srv.URL,
		EnvAPIKey:       "LLMBOX_B5_NEVER_SET7",
		ProviderName:    "bench",
	})
	ctx := context.Background()
	params := map[string]string{"api_key": "sk-test"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := node.Execute(ctx, "hello", params)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkLLMClient_Execute_WithTelemetry measures the additional cost
// of telemetry collection vs. the no-telemetry baseline above. The
// recordingSink has a mutex so this also exercises the contention path.
func BenchmarkLLMClient_Execute_WithTelemetry(b *testing.B) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"hi"}}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
	}))
	defer srv.Close()

	node := NewOpenAICompatibleNode(LLMNodeConfig{
		Name:            "bench",
		DefaultModel:    "m",
		DefaultEndpoint: srv.URL,
		EnvAPIKey:       "LLMBOX_B5_NEVER_SET8",
		ProviderName:    "bench",
	})
	sink := &recordingSink{}
	ctx := WithLLMCallSink(context.Background(), sink)
	params := map[string]string{"api_key": "sk-test"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := node.Execute(ctx, "hello", params)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkLLMClient_Parallel_NoTelemetry measures throughput under
// concurrent load (mimicking parallel DAG steps hitting the same provider).
func BenchmarkLLMClient_Parallel_NoTelemetry(b *testing.B) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"hi"}}]}`))
	}))
	defer srv.Close()

	node := NewOpenAICompatibleNode(LLMNodeConfig{
		Name:            "benchpar",
		DefaultModel:    "m",
		DefaultEndpoint: srv.URL,
		EnvAPIKey:       "LLMBOX_B5_NEVER_SET9",
		ProviderName:    "benchpar",
	})
	ctx := context.Background()
	params := map[string]string{"api_key": "sk-test"}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := node.Execute(ctx, "hello", params)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkLLMRequest_Marshal measures the cost of serializing a fully
// populated LLMRequest — useful to detect regressions when fields are
// added to the request shape.
func BenchmarkLLMRequest_Marshal(b *testing.B) {
	seed := 42
	req := LLMRequest{
		Model:            "gpt-test",
		Messages:         []LLMMessage{{Role: "user", Content: "hi"}},
		Temperature:      0.7,
		MaxTokens:        100,
		TopP:             0.9,
		FrequencyPenalty: 0.5,
		PresencePenalty:  -0.3,
		Stop:             []string{"\n", "END"},
		Seed:             &seed,
		ResponseFormat:   &ResponseFormat{Type: "json_object"},
		Tools:            []ToolDefinition{{Type: "function", Function: ToolFunction{Name: "f", Parameters: map[string]interface{}{"type": "object"}}}},
		ToolChoice:       json.RawMessage(`"auto"`),
		User:             "u-1",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := json.Marshal(req)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkLLMResponse_Unmarshal_WithUsage measures the cost of decoding
// a representative provider response.
func BenchmarkLLMResponse_Unmarshal_WithUsage(b *testing.B) {
	raw := []byte(`{"id":"chatcmpl-1","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"hello world"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1234,"completion_tokens":567,"total_tokens":1801}}`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var resp LLMResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			b.Fatal(err)
		}
	}
}

// newSSEBenchServer returns a test server that emits a small SSE stream
// (5 content chunks + [DONE]), suitable for benchmarking the streaming
// read path including the streamBufPool.
func newSSEBenchServer(b *testing.B) *httptest.Server {
	b.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		enc := json.NewEncoder(w)
		for i := 0; i < 5; i++ {
			_, _ = w.Write([]byte("data: "))
			_ = enc.Encode(LLMResponse{Choices: []LLMChoice{{Delta: LLMChoiceDelta{Content: "chunk-"}}}})
			if flusher != nil {
				flusher.Flush()
			}
		}
		_, _ = w.Write([]byte("data: [DONE]\n"))
	}))
}

// BenchmarkLLMClient_ExecuteStream measures the streaming read path. The
// streamBufPool should keep the per-call 256KB scanner buffer allocation
// near zero in steady state (run with -benchmem to verify).
func BenchmarkLLMClient_ExecuteStream(b *testing.B) {
	srv := newSSEBenchServer(b)
	defer srv.Close()

	node := NewOpenAICompatibleNode(LLMNodeConfig{
		Name:            "benchstream",
		DefaultModel:    "m",
		DefaultEndpoint: srv.URL,
		EnvAPIKey:       "LLMBOX_B5_NEVER_SET10",
		ProviderName:    "benchstream",
	})
	ctx := context.Background()
	params := map[string]string{"api_key": "sk-test", "stream": "true"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := node.ExecuteStream(ctx, "hello", params, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkLLMClient_ExecuteStream_Parallel exercises the streamBufPool
// under concurrent fan-out (the map node's workload). The pool must not
// corrupt buffers across goroutines; -race in combination with this
// benchmark is the primary correctness check for the pool.
func BenchmarkLLMClient_ExecuteStream_Parallel(b *testing.B) {
	srv := newSSEBenchServer(b)
	defer srv.Close()

	node := NewOpenAICompatibleNode(LLMNodeConfig{
		Name:            "benchstreampar",
		DefaultModel:    "m",
		DefaultEndpoint: srv.URL,
		EnvAPIKey:       "LLMBOX_B5_NEVER_SET11",
		ProviderName:    "benchstreampar",
	})
	ctx := context.Background()
	params := map[string]string{"api_key": "sk-test", "stream": "true"}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := node.ExecuteStream(ctx, "hello", params, nil)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
