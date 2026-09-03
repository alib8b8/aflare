// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​‌‌​​‌​​‌‌​​​‌​​‌‌​‌‌‌​‌​​‌‌​​‌​​‌​‌‌‌​‌‌​​​​‌​‌​​​‌​‌‌‌‌‌​‌‌​‌​‌‌‌‌​​​​​​​​​​​​​​​​​​​‌​‌​‌​‌‌​‌​‌‌​‌⁠
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​‌‌‌‌‌​‌‌​‌​‌​​​‌‌‌​​‌‌​‌​‌‌​​‌​​‌​‌‌‌​‌​​‌​​‌​​‌‌​​‌‌​‌​​​‌​​‌‌​​‌​​​‌​‌​​‌‌​‌​​‌​‌​‌​​‌​​​​‌​​‌​​​​​​​‌​​​​‌​​
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

package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alib8b8/aflare/internal/agentx"
)

func TestProbeOne_CLI(t *testing.T) {
	// "go" is guaranteed on PATH inside the module's own test run.
	ok, err := probeOne(context.Background(), agentx.AgentDef{
		Name: "gogo", Driver: agentx.DriverCLI, Profile: "generic", Binary: "go",
	})
	if err != nil {
		t.Fatalf("probeOne(go): %v", err)
	}
	if !strings.Contains(ok, "profile generic") {
		t.Errorf("detail = %q, want profile note", ok)
	}

	_, err = probeOne(context.Background(), agentx.AgentDef{
		Name: "ghost", Driver: agentx.DriverCLI, Profile: "generic", Binary: "aflare-definitely-not-installed",
	})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("err = %v, want binary-not-found", err)
	}
}

func TestProbeOne_A2A(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/.well-known/") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"Researcher","version":"1.4","skills":[{"id":"s1"},{"id":"s2"}]}`))
	}))
	t.Cleanup(srv.Close)

	detail, err := probeOne(context.Background(), agentx.AgentDef{
		Name: "remote", Driver: agentx.DriverA2A, URL: srv.URL + "/",
	})
	if err != nil {
		t.Fatalf("probeOne(a2a): %v", err)
	}
	for _, want := range []string{"Researcher", "1.4", "2 skills"} {
		if !strings.Contains(detail, want) {
			t.Errorf("detail = %q, want %q", detail, want)
		}
	}

	dead := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(dead.Close)
	detail, err = probeOne(context.Background(), agentx.AgentDef{
		Name: "dead", Driver: agentx.DriverA2A, URL: dead.URL + "/",
	})
	if err == nil || !strings.Contains(err.Error(), "no agent card") {
		t.Fatalf("err = %v, want missing agent card", err)
	}
	if !strings.Contains(detail, "unreachable") {
		t.Errorf("detail = %q, want unreachable note", detail)
	}
}

func TestProbeOne_DisabledAgent(t *testing.T) {
	disabled := false
	_, err := probeOne(context.Background(), agentx.AgentDef{
		Name: "off", Driver: agentx.DriverCLI, Profile: "generic", Binary: "go", Enabled: &disabled,
	})
	if err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("err = %v, want disabled rejection", err)
	}
}

func TestProbeAgents_UnknownNameRejected(t *testing.T) {
	registerProbeTestAgents(t, nil)
	if err := probeAgents([]string{"ghost"}); err == nil || !strings.Contains(err.Error(), "not registered") {
		t.Fatalf("err = %v, want not-registered rejection", err)
	}
}

func TestProbeAgents_ExitCodeOnFailure(t *testing.T) {
	dead := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(dead.Close)
	registerProbeTestAgents(t, map[string]agentx.AgentDef{
		"gone":   {Driver: agentx.DriverCLI, Profile: "generic", Binary: "aflare-definitely-not-installed"},
		"remote": {Driver: agentx.DriverA2A, URL: dead.URL + "/"},
	})

	// Any probed agent not ready → non-zero exit.
	err := probeAgents([]string{"gone"})
	if err == nil {
		t.Fatal("probeAgents(gone) = nil, want exit error")
	}
	if err := probeAgents(nil); err == nil {
		t.Fatal("probeAgents(all) = nil with one broken agent, want exit error")
	}
}

// registerProbeTestAgents swaps the agentx registry loader for a test.
func registerProbeTestAgents(t *testing.T, defs map[string]agentx.AgentDef) {
	t.Helper()
	agentx.SetLoader(func() map[string]agentx.AgentDef { return defs })
	t.Cleanup(func() {
		agentx.SetLoader(func() map[string]agentx.AgentDef { return nil })
	})
}
