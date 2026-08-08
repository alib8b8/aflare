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

package policy

import (
	"context"
	"testing"
)

func TestEngineDefaultPolicy(t *testing.T) {
	engine := NewEngine(DefaultPolicy(), nil)
	ctx := context.Background()

	// Filesystem: read allowed, write allowed, delete requires approval
	dec, err := engine.Check(ctx, ActionFileRead, "test.txt")
	if err != nil || dec != DecisionAllowed {
		t.Fatalf("file read should be allowed, got %s/%v", dec, err)
	}

	dec, err = engine.Check(ctx, ActionFileWrite, "test.txt")
	if err != nil || dec != DecisionAllowed {
		t.Fatalf("file write should be allowed, got %s/%v", dec, err)
	}

	// Delete requires approval but no approval func -> denied
	dec, err = engine.Check(ctx, ActionFileDelete, "test.txt")
	if err == nil {
		t.Fatalf("file delete without approval func should be denied, got %s", dec)
	}
}

func TestEngineStrictPolicy(t *testing.T) {
	engine := NewEngine(StrictPolicy(), nil)
	ctx := context.Background()

	// Shell is disabled
	_, err := engine.Check(ctx, ActionShellExec, "ls")
	if err == nil {
		t.Fatal("shell should be denied in strict mode")
	}

	// Network: allowlisted domains
	_, err = engine.Check(ctx, ActionNetworkHTTP, "api.github.com")
	if err != nil {
		t.Fatalf("api.github.com should be allowlisted: %v", err)
	}

	_, err = engine.Check(ctx, ActionNetworkHTTP, "evil.com")
	if err == nil {
		t.Fatal("evil.com should not be allowlisted")
	}

	// Financial: requires approval
	_, err = engine.Check(ctx, ActionFinancial, "transfer")
	if err == nil {
		t.Fatal("financial transfer should require approval")
	}
}

func TestEngineWithApproval(t *testing.T) {
	engine := NewEngine(StrictPolicy(), func(ctx context.Context, action Action, details string) (bool, error) {
		return true, nil // auto-approve
	})
	ctx := context.Background()

	dec, err := engine.Check(ctx, ActionFinancial, "transfer")
	if err != nil || dec != DecisionApproval {
		t.Fatalf("financial transfer with approval should be approved, got %s/%v", dec, err)
	}
}

func TestEngineWithDenial(t *testing.T) {
	engine := NewEngine(StrictPolicy(), func(ctx context.Context, action Action, details string) (bool, error) {
		return false, nil // auto-deny
	})
	ctx := context.Background()

	_, err := engine.Check(ctx, ActionFinancial, "transfer")
	if err == nil {
		t.Fatal("financial transfer should be denied when approval returns false")
	}
}

func TestShellAllowlist(t *testing.T) {
	p := DefaultPolicy()
	p.Shell.ApprovalRequired = false
	p.Shell.Enabled = true
	p.Shell.Allowlist = []string{"ls", "cat", "echo"}
	p.Shell.Denylist = []string{"rm", "sudo"}

	engine := NewEngine(p, nil)
	ctx := context.Background()

	// Allowed commands
	for _, cmd := range []string{"ls", "cat", "echo"} {
		_, err := engine.Check(ctx, ActionShellExec, cmd)
		if err != nil {
			t.Fatalf("command %q should be allowed: %v", cmd, err)
		}
	}

	// Denylisted commands
	for _, cmd := range []string{"rm", "sudo"} {
		_, err := engine.Check(ctx, ActionShellExec, cmd)
		if err == nil {
			t.Fatalf("command %q should be denied", cmd)
		}
	}

	// Not in allowlist
	_, err := engine.Check(ctx, ActionShellExec, "curl")
	if err == nil {
		t.Fatal("curl should not be in allowlist")
	}
}

func TestNetworkDenylist(t *testing.T) {
	p := DefaultPolicy()
	p.Network.Denylist = []string{"169.254.169.254", "metadata.google.internal"}

	engine := NewEngine(p, nil)
	ctx := context.Background()

	_, err := engine.Check(ctx, ActionNetworkHTTP, "169.254.169.254")
	if err == nil {
		t.Fatal("AWS metadata endpoint should be denied")
	}

	_, err = engine.Check(ctx, ActionNetworkHTTP, "api.github.com")
	if err != nil {
		t.Fatalf("api.github.com should be allowed: %v", err)
	}
}

func TestDomainMatching(t *testing.T) {
	tests := []struct {
		host    string
		pattern string
		match   bool
	}{
		{"api.openai.com", "*.openai.com", true},
		{"chat.openai.com", "*.openai.com", true},
		{"openai.com", "*.openai.com", true},
		{"evil-openai.com", "*.openai.com", false},
		{"api.github.com", "api.github.com", true},
		{"api.github.com", "api.gitlab.com", false},
		{"anything", "*", true},
	}

	for _, tt := range tests {
		result := matchDomain(tt.host, tt.pattern)
		if result != tt.match {
			t.Errorf("matchDomain(%q, %q) = %v, want %v", tt.host, tt.pattern, result, tt.match)
		}
	}
}

func TestLoadPolicy(t *testing.T) {
	// Load from the project's policy.yaml
	p, err := LoadPolicy("../../policy.yaml")
	if err != nil {
		t.Fatalf("failed to load policy.yaml: %v", err)
	}

	if p.Shell.Enabled {
		t.Error("shell should be disabled in default policy.yaml")
	}
	// The stored value is "approval_required", but evaluateFinancial normalizes it
	if p.Financial.Transfer != "approval_required" && p.Financial.Transfer != "approval" {
		t.Errorf("financial transfer should be approval_required, got %q", p.Financial.Transfer)
	}
}

func TestPolicyFieldAccess(t *testing.T) {
	engine := NewEngine(StrictPolicy(), nil)
	p := engine.Policy()
	if p == nil {
		t.Fatal("Policy() should return non-nil")
	}
	if p.Shell.Enabled {
		t.Error("strict policy should disable shell")
	}
}
