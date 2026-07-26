// Copyright (c) 2026 llm-box Contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package nodes

import (
	"context"
	"strings"
	"testing"
)

func TestAgentBrowser_Schema(t *testing.T) {
	node, ok := Get("agent_browser")
	if !ok {
		t.Fatal("agent_browser not found in registry")
	}

	if node.Name() != "agent_browser" {
		t.Errorf("expected node name 'agent_browser', got %q", node.Name())
	}

	schema := node.Schema()
	if schema.Name != "agent_browser" {
		t.Errorf("expected schema name 'agent_browser', got %q", schema.Name)
	}
	if schema.Description == "" {
		t.Error("expected non-empty schema description")
	}
}

func TestAgentBrowser_DefaultAction(t *testing.T) {
	node, ok := Get("agent_browser")
	if !ok {
		t.Fatal("agent_browser not found in registry")
	}

	ctx := context.Background()

	// With no action param, getParam defaults to "visit". Without a URL,
	// Execute should fail with a "visit"-specific error, proving the default
	// was applied.
	_, err := node.Execute(ctx, "", map[string]string{})
	if err == nil {
		t.Fatal("expected error for missing URL with default action")
	}
	if !strings.Contains(err.Error(), "visit") {
		t.Errorf("expected error to mention default action 'visit', got: %v", err)
	}
}

func TestAgentBrowser_EmptyURL(t *testing.T) {
	node, ok := Get("agent_browser")
	if !ok {
		t.Fatal("agent_browser not found in registry")
	}

	ctx := context.Background()

	t.Run("explicit visit with no URL", func(t *testing.T) {
		_, err := node.Execute(ctx, "", map[string]string{"action": "visit"})
		if err == nil {
			t.Error("expected error for empty URL on visit action")
		}
	})

	t.Run("input is not a URL", func(t *testing.T) {
		// Plain text input is not promoted to a URL since it lacks the
		// http:// or https:// prefix.
		_, err := node.Execute(ctx, "not a url", map[string]string{"action": "visit"})
		if err == nil {
			t.Error("expected error when input is not a URL")
		}
	})

	t.Run("extract action requires URL", func(t *testing.T) {
		_, err := node.Execute(ctx, "", map[string]string{"action": "extract"})
		if err == nil {
			t.Error("expected error for empty URL on extract action")
		}
	})
}

func TestAgentBrowser_InvalidSummaryLen(t *testing.T) {
	node, ok := Get("agent_browser")
	if !ok {
		t.Fatal("agent_browser not found in registry")
	}

	ctx := context.Background()

	// The screenshot action does not perform network I/O, so it lets us
	// verify that an invalid summary_length value falls back to the default
	// (2000) without crashing Execute. If the fallback failed, the parse
	// error would propagate up or the action would never run.
	output, err := node.Execute(ctx, "", map[string]string{
		"action":         "screenshot",
		"url":            "https://example.com",
		"summary_length": "not-a-number",
	})
	if err != nil {
		t.Fatalf("unexpected error with invalid summary_length: %v", err)
	}
	if !strings.Contains(output, "Screenshot of") {
		t.Errorf("expected screenshot output, got: %s", output)
	}
}

func TestAgentBrowser_SummaryLenBelowMinimum(t *testing.T) {
	node, ok := Get("agent_browser")
	if !ok {
		t.Fatal("agent_browser not found in registry")
	}

	ctx := context.Background()

	// summary_length < 100 is reset to the default of 2000. Use the
	// screenshot action so no network call is made.
	output, err := node.Execute(ctx, "", map[string]string{
		"action":         "screenshot",
		"url":            "https://example.com",
		"summary_length": "10",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "Screenshot of") {
		t.Errorf("expected screenshot output, got: %s", output)
	}
}

func TestAgentBrowser_InvalidURL(t *testing.T) {
	node, ok := Get("agent_browser")
	if !ok {
		t.Fatal("agent_browser not found in registry")
	}

	ctx := context.Background()

	// Use URLs that url.Parse rejects.
	invalidURLs := []string{
		"http://[",                // missing ']' in IPv6 host
		"http://example.com\nfoo", // control character in URL
	}

	for _, u := range invalidURLs {
		_, err := node.Execute(ctx, "", map[string]string{
			"action": "screenshot",
			"url":    u,
		})
		if err == nil {
			t.Errorf("expected error for invalid URL %q", u)
		}
		if err != nil && !strings.Contains(err.Error(), "invalid URL") {
			t.Errorf("expected 'invalid URL' error for %q, got: %v", u, err)
		}
	}
}

func TestAgentBrowser_UnknownAction(t *testing.T) {
	node, ok := Get("agent_browser")
	if !ok {
		t.Fatal("agent_browser not found in registry")
	}

	ctx := context.Background()

	_, err := node.Execute(ctx, "", map[string]string{
		"action": "unknown_action",
		"url":    "https://example.com",
	})
	if err == nil {
		t.Error("expected error for unknown browser action")
	}
	if !strings.Contains(err.Error(), "unknown browser action") {
		t.Errorf("expected 'unknown browser action' error, got: %v", err)
	}
}
