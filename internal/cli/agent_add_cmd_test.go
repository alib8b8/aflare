// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​‌‌​​‌‌​​​‌‌​​​​​​‌​​​‌​​​‌​‌​‌​‌‌​‌​​​‌‌‌‌​‌​​​​‌​‌‌‌​​​​​‌‌‌‌‌‌​‌​‌‌​​​​​​​​​​​​​​​​‌​​‌‌‌​​​​​​​​‌​⁠
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
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alib8b8/aflare/internal/agentx"
)

// cardServer serves an A2A agent card at the well-known path.
func cardServer(t *testing.T, cardJSON string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/agent-card.json" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(cardJSON))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// useAgentStore points the agent store at a temp file for the test.
func useAgentStore(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "agents.yaml")
	t.Setenv("AFLARE_AGENTS_FILE", path)
	return path
}

func TestAddAgent_RegistersFromCard(t *testing.T) {
	storePath := useAgentStore(t)
	srv := cardServer(t, `{
		"name": "Deep Research Bot",
		"description": "Runs multi-source research",
		"url": "http://ignored.example/",
		"skills": [
			{"id": "s1", "name": "search", "description": "web search"},
			{"id": "s2", "name": "cite"}
		]
	}`)

	if err := addAgent([]string{srv.URL + "/", "--api-key-env", "RESEARCH_TOKEN"}); err != nil {
		t.Fatalf("addAgent: %v", err)
	}

	stored, err := agentx.LoadAgentStore(storePath)
	if err != nil {
		t.Fatal(err)
	}
	def, ok := stored["deep-research-bot"]
	if !ok {
		t.Fatalf("stored agents = %v, want slug deep-research-bot", stored)
	}
	if def.Driver != agentx.DriverA2A {
		t.Errorf("driver = %q, want a2a", def.Driver)
	}
	if def.URL != srv.URL {
		t.Errorf("url = %q, want %q (trailing slash stripped)", def.URL, srv.URL)
	}
	if def.APIKeyEnv != "RESEARCH_TOKEN" {
		t.Errorf("api_key_env = %q, want env var name only", def.APIKeyEnv)
	}
	for _, want := range []string{"Runs multi-source research", "Skills:", "search (web search)", "cite"} {
		if !strings.Contains(def.Description, want) {
			t.Errorf("description = %q, want to contain %q", def.Description, want)
		}
	}

	// The registered agent must resolve for delegation.
	if _, err := def.Resolve(); err != nil {
		t.Errorf("resolve: %v", err)
	}
}

func TestAddAgent_ExplicitNameAndDescription(t *testing.T) {
	storePath := useAgentStore(t)
	srv := cardServer(t, `{"name":"Whatever","description":"card text"}`)

	if err := addAgent([]string{srv.URL, "--name", "research", "--description", "override"}); err != nil {
		t.Fatalf("addAgent: %v", err)
	}
	stored, err := agentx.LoadAgentStore(storePath)
	if err != nil {
		t.Fatal(err)
	}
	if def := stored["research"]; def.Description != "override" {
		t.Errorf("description = %q, want override", def.Description)
	}
}

func TestAddAgent_Rejections(t *testing.T) {
	useAgentStore(t)
	srv := cardServer(t, `{"name":"Deep Research Bot","description":"d"}`)

	// No URL.
	if err := addAgent(nil); err == nil {
		t.Error("addAgent(no url) = nil, want usage error")
	}

	// Explicit --name colliding with a built-in preset is refused.
	if err := addAgent([]string{srv.URL, "--name", "codex"}); err == nil {
		t.Error("addAgent(--name codex) = nil, want builtin collision refusal")
	}

	// A card whose slug derives to a built-in name is refused too.
	builtin := cardServer(t, `{"name":"Claude","description":"shadow"}`)
	if err := addAgent([]string{builtin.URL}); err == nil {
		t.Error("addAgent(card named Claude) = nil, want builtin collision refusal")
	}

	// Duplicate registration is refused.
	if err := addAgent([]string{srv.URL, "--name", "research"}); err != nil {
		t.Fatalf("first add: %v", err)
	}
	if err := addAgent([]string{srv.URL, "--name", "research"}); err == nil {
		t.Error("addAgent(duplicate) = nil, want already-registered refusal")
	}
}

func TestAddAgent_UnreachableServer(t *testing.T) {
	useAgentStore(t)
	dead := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(dead.Close)

	if err := addAgent([]string{dead.URL}); err == nil {
		t.Fatal("addAgent(dead server) = nil, want error")
	}
}

func TestAddAgent_StoreIsUsableByRegistry(t *testing.T) {
	storePath := useAgentStore(t)
	srv := cardServer(t, `{"name":"Deep Research Bot","description":"Runs research"}`)
	if err := addAgent([]string{srv.URL}); err != nil {
		t.Fatalf("addAgent: %v", err)
	}

	// The same merge the workflow engine performs: store entries feed
	// the agentx registry on top of built-ins.
	stored, err := agentx.LoadAgentStore(storePath)
	if err != nil {
		t.Fatal(err)
	}
	agentx.SetLoader(func() map[string]agentx.AgentDef { return stored })
	t.Cleanup(func() { agentx.SetLoader(func() map[string]agentx.AgentDef { return nil }) })

	def, ok := agentx.Get("deep-research-bot")
	if !ok {
		t.Fatal("registry does not see the added agent")
	}
	if def.Driver != agentx.DriverA2A || def.URL != srv.URL {
		t.Errorf("registry def = %+v", def)
	}
	if _, isBuiltin := agentx.Get("codex"); !isBuiltin {
		t.Error("built-in presets lost after store merge")
	}
}

func TestSplitAgentAddArgs(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		wantURL  string
		wantRest []string
	}{
		{"url only", []string{"http://x"}, "http://x", nil},
		{"flags after url", []string{"http://x", "--name", "a", "--api-key-env", "V"}, "http://x", []string{"--name", "a", "--api-key-env", "V"}},
		{"flags before url", []string{"--name", "a", "http://x"}, "http://x", []string{"--name", "a"}},
		{"inline values", []string{"http://x", "--name=a", "--description=b c"}, "http://x", []string{"--name=a", "--description=b c"}},
		{"no url", []string{"--name", "a"}, "", []string{"--name", "a"}},
		{"second positional rejected", []string{"http://x", "http://y"}, "http://x", []string{"http://y"}},
		{"empty", nil, "", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			url, rest := splitAgentAddArgs(tc.args)
			if url != tc.wantURL {
				t.Errorf("url = %q, want %q", url, tc.wantURL)
			}
			if len(rest) != len(tc.wantRest) {
				t.Fatalf("rest = %v, want %v", rest, tc.wantRest)
			}
			for i := range rest {
				if rest[i] != tc.wantRest[i] {
					t.Errorf("rest[%d] = %q, want %q", i, rest[i], tc.wantRest[i])
				}
			}
		})
	}
}

func TestSlugifyAgentName(t *testing.T) {
	cases := map[string]string{
		"Deep Research Bot": "deep-research-bot",
		"  Trimmed  ":       "trimmed",
		"Multiple---Dashes": "multiple-dashes",
		"punct!@#$free":     "punct-free",
		"-leading-trail-":   "leading-trail",
		"":                  "",
		"///":               "",
		"upperCASE 42":      "uppercase-42",
	}
	for in, want := range cases {
		if got := slugifyAgentName(in); got != want {
			t.Errorf("slugifyAgentName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDescribeFromCard(t *testing.T) {
	card := &agentx.AgentCard{
		Name:        "Bot",
		Description: "Does things",
		Skills: []agentx.AgentSkill{
			{ID: "s1", Name: "alpha", Description: "first"},
			{ID: "s2"}, // no name: falls back to ID
		},
	}
	got := describeFromCard(card)
	for _, want := range []string{"Does things", "Skills:", "alpha (first)", "s2"} {
		if !strings.Contains(got, want) {
			t.Errorf("describeFromCard = %q, want %q", got, want)
		}
	}

	// No description: falls back to the card name; no name at all: a
	// generic label. Both still list skills.
	nameOnly := describeFromCard(&agentx.AgentCard{Name: "Solo"})
	if !strings.Contains(nameOnly, `"Solo"`) {
		t.Errorf("describeFromCard(name only) = %q", nameOnly)
	}
	generic := describeFromCard(&agentx.AgentCard{})
	if !strings.Contains(generic, "Remote A2A agent") {
		t.Errorf("describeFromCard(empty card) = %q", generic)
	}
}
