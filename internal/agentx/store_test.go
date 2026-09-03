// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​‌‌​​‌‌​​​‌‌​​​​​​‌​​​‌‌‌​‌​‌​‌​‌​‌​‌‌‌​‌​​‌‌​​‌‌​‌​​‌‌​‌‌‌​‌‌​​‌‌‌​​​​​​​​​​​​​​​​​​​​‌‌‌‌​‌‌‌​​‌​​‌‌⁠
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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadAgentStore_MissingFileIsEmpty(t *testing.T) {
	agents, err := LoadAgentStore(filepath.Join(t.TempDir(), "absent", "agents.yaml"))
	if err != nil {
		t.Fatalf("LoadAgentStore(missing) = %v, want nil", err)
	}
	if len(agents) != 0 {
		t.Fatalf("agents = %v, want empty", agents)
	}
}

func TestLoadAgentStore_MalformedFileIsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agents.yaml")
	if err := os.WriteFile(path, []byte("agents: [not: a: map"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadAgentStore(path); err == nil {
		t.Fatal("LoadAgentStore(malformed) = nil, want error")
	}
}

func TestAgentStore_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "agents.yaml")
	enabled := false
	saved := map[string]AgentDef{
		"researcher": {
			Driver:      DriverA2A,
			Description: "Deep research",
			URL:         "http://127.0.0.1:9000/",
			APIKeyEnv:   "RESEARCHER_TOKEN",
			Enabled:     &enabled,
		},
		"shell": {
			Driver:  DriverCLI,
			Profile: "generic",
			Binary:  "/usr/local/bin/agent",
			Args:    []string{"--fast"},
		},
	}
	if err := SaveAgentStore(path, saved); err != nil {
		t.Fatalf("SaveAgentStore: %v", err)
	}

	loaded, err := LoadAgentStore(path)
	if err != nil {
		t.Fatalf("LoadAgentStore: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("len(loaded) = %d, want 2", len(loaded))
	}
	r := loaded["researcher"]
	if r.Name != "researcher" || r.Driver != DriverA2A || r.URL != "http://127.0.0.1:9000/" {
		t.Errorf("researcher = %+v", r)
	}
	if r.APIKeyEnv != "RESEARCHER_TOKEN" {
		t.Errorf("APIKeyEnv = %q, want env var name (never the token)", r.APIKeyEnv)
	}
	if r.Enabled == nil || *r.Enabled {
		t.Errorf("Enabled = %v, want false preserved", r.Enabled)
	}
	s := loaded["shell"]
	if s.Profile != "generic" || s.Binary != "/usr/local/bin/agent" || len(s.Args) != 1 {
		t.Errorf("shell = %+v", s)
	}

	// The store must reference only the env var NAME, never a secret
	// value — there is no field on AgentDef that could carry one.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "RESEARCHER_TOKEN") {
		t.Errorf("store = %q, want api_key_env name recorded", data)
	}
}

func TestSaveAgentStore_NoStaleTmpLeftBehind(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agents.yaml")
	if err := SaveAgentStore(path, map[string]AgentDef{"a": {Driver: DriverA2A, URL: "http://x"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(path + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("stale tmp file remains: %v", err)
	}
	// Overwrite in place also cleans up after itself.
	if err := SaveAgentStore(path, map[string]AgentDef{"b": {Driver: DriverA2A, URL: "http://y"}}); err != nil {
		t.Fatal(err)
	}
	agents, err := LoadAgentStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(agents) != 1 || agents["b"].URL != "http://y" {
		t.Errorf("agents after overwrite = %v", agents)
	}
}

func TestDefaultAgentStorePath_EnvOverride(t *testing.T) {
	t.Setenv("AFLARE_AGENTS_FILE", "/custom/agents.yaml")
	if got := DefaultAgentStorePath(); got != "/custom/agents.yaml" {
		t.Errorf("DefaultAgentStorePath() = %q, want env override", got)
	}
}
