// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​‌‌​​‌​‌​​‌‌​​​‌​​‌​​​​​‌​‌​‌‌​​‌​​‌‌​‌‌​​​​​​​‌​‌​‌‌‌‌​‌‌​​​​​​‌‌​‌​‌​​​​​​​​​​​​​​​​‌‌​​​​​​‌‌​​‌‌​​⁠
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

package agentx

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// breakerTestPolicy installs a fast breaker policy and restores defaults
// (plus clean state) when the test ends.
func breakerTestPolicy(t *testing.T, threshold int, openFor time.Duration) {
	t.Helper()
	setAgentBreakerPolicyForTest(threshold, openFor)
	resetAgentBreakers()
	t.Cleanup(func() {
		setAgentBreakerPolicyForTest(defaultBreakerThreshold, defaultBreakerOpenTime)
		resetAgentBreakers()
	})
}

func TestBreaker_StateTransitions(t *testing.T) {
	b := newAgentBreakerSet(2, 20*time.Millisecond)
	def := AgentDef{Name: "cli-x", Driver: DriverCLI, Binary: "x"}

	// Closed below the threshold.
	b.record(def.Name, errors.New("boom"))
	if err := b.allow(context.Background(), def); err != nil {
		t.Fatalf("allow after 1 failure: %v", err)
	}

	// Threshold reached: open, delegations fast-fail.
	b.record(def.Name, errors.New("boom again"))
	var open *CircuitOpenError
	if err := b.allow(context.Background(), def); !errors.As(err, &open) {
		t.Fatalf("allow after 2 failures: got %v, want CircuitOpenError", err)
	}
	if !strings.Contains(open.Error(), "circuit-open") {
		t.Errorf("error message %q must contain circuit-open", open.Error())
	}

	// Cool-down elapsed: half-open lets a CLI delegation through...
	time.Sleep(30 * time.Millisecond)
	if err := b.allow(context.Background(), def); err != nil {
		t.Fatalf("allow half-open (cli): %v", err)
	}
	// ...and its failure re-opens immediately.
	b.record(def.Name, errors.New("still broken"))
	if err := b.allow(context.Background(), def); !errors.As(err, &open) {
		t.Fatalf("allow after half-open failure: got %v, want CircuitOpenError", err)
	}

	// Next cool-down + success closes the circuit for good.
	time.Sleep(30 * time.Millisecond)
	if err := b.allow(context.Background(), def); err != nil {
		t.Fatalf("allow half-open (cli, 2nd): %v", err)
	}
	b.record(def.Name, nil)
	if err := b.allow(context.Background(), def); err != nil {
		t.Fatalf("allow after success: %v", err)
	}
}

func TestBreaker_A2AHalfOpenProbesAgentCard(t *testing.T) {
	// The card endpoint is the cheap health check: while it 500s the
	// circuit stays open, once it serves a card the probe closes the
	// circuit without a real delegation.
	var cardHealthy atomic.Bool
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/agent-card.json", func(w http.ResponseWriter, r *http.Request) {
		if !cardHealthy.Load() {
			http.Error(w, "down", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"name": "half-open-agent", "url": "http://example.invalid/"})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	b := newAgentBreakerSet(1, 10*time.Millisecond)
	def := AgentDef{Name: "a2a-x", Driver: DriverA2A, URL: srv.URL + "/"}

	b.record(def.Name, errors.New("endpoint down"))
	time.Sleep(15 * time.Millisecond)

	var open *CircuitOpenError
	if err := b.allow(context.Background(), def); !errors.As(err, &open) {
		t.Fatalf("half-open probe against broken card: got %v, want CircuitOpenError", err)
	}
	if open.ProbeErr == nil {
		t.Error("ProbeErr should carry the agent-card failure")
	}

	// The failed probe re-opened the circuit (fresh cool-down); wait it
	// out before the retry against the healed card endpoint.
	time.Sleep(15 * time.Millisecond)
	cardHealthy.Store(true)
	if err := b.allow(context.Background(), def); err != nil {
		t.Fatalf("half-open probe against healthy card: %v", err)
	}
	// The probe itself closed the circuit.
	if err := b.allow(context.Background(), def); err != nil {
		t.Fatalf("allow after successful probe: %v", err)
	}
}

func TestRunCLI_CircuitBreakerFastFail(t *testing.T) {
	breakerTestPolicy(t, 3, time.Minute)

	binary := writeFakeAgent(t, "fail")
	def := AgentDef{Name: "cb-cli", Driver: DriverCLI, Profile: "generic", Binary: binary}

	// Three failing delegations trip the circuit.
	for i := 0; i < 3; i++ {
		if _, err := RunCLI(context.Background(), def, Task{Prompt: "do something"}); err == nil {
			t.Fatalf("delegation %d: expected failure", i+1)
		}
	}

	var open *CircuitOpenError
	_, err := RunCLI(context.Background(), def, Task{Prompt: "do something"})
	if !errors.As(err, &open) {
		t.Fatalf("4th delegation: got %v, want CircuitOpenError", err)
	}
	if open.Agent != "cb-cli" {
		t.Errorf("circuit-open agent = %q, want cb-cli", open.Agent)
	}
}

func TestSendMessage_CircuitBreakerOpenAndRecover(t *testing.T) {
	breakerTestPolicy(t, 3, 100*time.Millisecond)

	var healthy atomic.Bool
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/agent-card.json", func(w http.ResponseWriter, r *http.Request) {
		if !healthy.Load() {
			http.Error(w, "down", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"name": "recover-agent", "url": "http://example.invalid/"})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if !healthy.Load() {
			http.Error(w, "down", http.StatusInternalServerError)
			return
		}
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]any{
				"id":     "task-1",
				"status": map[string]any{"state": "completed"},
				"artifacts": []map[string]any{
					{"parts": []map[string]any{{"kind": "text", "text": "recovered output"}}},
				},
			},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	def := AgentDef{Name: "cb-a2a", Driver: DriverA2A, URL: srv.URL + "/"}

	// Three failing delegations trip the circuit.
	for i := 0; i < 3; i++ {
		if _, err := SendMessage(context.Background(), def, Task{Prompt: "research"}); err == nil {
			t.Fatalf("delegation %d: expected failure", i+1)
		}
	}

	var open *CircuitOpenError
	_, err := SendMessage(context.Background(), def, Task{Prompt: "research"})
	if !errors.As(err, &open) {
		t.Fatalf("4th delegation: got %v, want CircuitOpenError", err)
	}

	// Endpoint recovers; after the cool-down the half-open agent-card
	// probe re-closes the circuit and the delegation succeeds.
	healthy.Store(true)
	time.Sleep(150 * time.Millisecond)
	out, err := SendMessage(context.Background(), def, Task{Prompt: "research"})
	if err != nil {
		t.Fatalf("delegation after recovery: %v", err)
	}
	if !strings.Contains(out, "recovered output") {
		t.Errorf("output = %q, want recovered output", out)
	}
}
