// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​‌​​‌​‌‌​‌‌‌​‌​‌​‌‌‌‌‌​‌​‌‌‌​‌‌‌​​‌​​‌‌‌​‌‌​​​‌​‌​​‌​​‌‌​‌‌​​‌​‌​‌‌​​​​​​​​​​​​​​​​​​​‌​‌​​‌​​‌​‌​‌‌‌‌⁠
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

package nodes

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alib8b8/aflare/internal/connector"
)

// httpConnectorSpec builds a validated http connector spec for tests.
func httpConnectorSpec(name, authType string) connector.Spec {
	spec := connector.Spec{
		Name:    name,
		Type:    connector.TypeHTTP,
		BaseURL: "https://api.example.com",
	}
	if authType != "" {
		spec.AuthType = authType
		spec.Credential = &connector.CredentialRef{
			Kind: connector.CredentialKindEnv,
			Key:  "TEST_API_CREDENTIAL",
		}
	}
	if authType == connector.AuthTypeBasic {
		spec.Username = "testuser"
	}
	if authType == connector.AuthTypeHeader {
		spec.AuthHeader = "X-API-Key"
	}
	return spec
}

// recordingServer captures the Authorization / auth header and the request
// path of the single request the node under test makes.
func recordingServer(t *testing.T, captured *map[string]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		(*captured)["path"] = r.URL.Path
		(*captured)["authorization"] = r.Header.Get("Authorization")
		(*captured)["x-api-key"] = r.Header.Get("X-API-Key")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestHTTPRequest_ConnectorBearerAuth(t *testing.T) {
	allowLoopback(t)
	t.Setenv("TEST_API_CREDENTIAL", "secret-token")

	captured := map[string]string{}
	srv := recordingServer(t, &captured)
	spec := httpConnectorSpec("test-api", connector.AuthTypeBearer)
	spec.BaseURL = srv.URL
	setupConnectorRegistry(t, spec)

	node := &HTTPRequestNode{}
	out, err := node.Execute(t.Context(), "", map[string]string{
		"connector": "test-api",
		"url":       "/v1/stats",
	})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if !strings.Contains(out, "HTTP 200") {
		t.Fatalf("unexpected output: %q", out)
	}
	if captured["path"] != "/v1/stats" {
		t.Errorf("expected path /v1/stats, got %q", captured["path"])
	}
	if captured["authorization"] != "Bearer secret-token" {
		t.Errorf("expected Bearer auth header, got %q", captured["authorization"])
	}
}

func TestHTTPRequest_ConnectorBasicAuth(t *testing.T) {
	allowLoopback(t)
	t.Setenv("TEST_API_CREDENTIAL", "hunter2")

	captured := map[string]string{}
	srv := recordingServer(t, &captured)
	spec := httpConnectorSpec("test-api", connector.AuthTypeBasic)
	spec.BaseURL = srv.URL
	setupConnectorRegistry(t, spec)

	node := &HTTPRequestNode{}
	if _, err := node.Execute(t.Context(), "", map[string]string{
		"connector": "test-api",
		"url":       "/",
	}); err != nil {
		t.Fatalf("request failed: %v", err)
	}
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("testuser:hunter2"))
	if captured["authorization"] != want {
		t.Errorf("expected %q, got %q", want, captured["authorization"])
	}
}

func TestHTTPRequest_ConnectorHeaderAuth(t *testing.T) {
	allowLoopback(t)
	t.Setenv("TEST_API_CREDENTIAL", "key-123")

	captured := map[string]string{}
	srv := recordingServer(t, &captured)
	spec := httpConnectorSpec("test-api", connector.AuthTypeHeader)
	spec.BaseURL = srv.URL
	setupConnectorRegistry(t, spec)

	node := &HTTPRequestNode{}
	if _, err := node.Execute(t.Context(), "", map[string]string{
		"connector": "test-api",
		"url":       "/data",
	}); err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if captured["x-api-key"] != "key-123" {
		t.Errorf("expected X-API-Key: key-123, got %q", captured["x-api-key"])
	}
	if captured["authorization"] != "" {
		t.Errorf("header auth must not set Authorization, got %q", captured["authorization"])
	}
}

func TestHTTPRequest_ConnectorPathPrefixJoin(t *testing.T) {
	allowLoopback(t)

	captured := map[string]string{}
	srv := recordingServer(t, &captured)
	spec := httpConnectorSpec("test-api", "")
	spec.BaseURL = srv.URL + "/v1" // base URL carries a path prefix
	setupConnectorRegistry(t, spec)

	node := &HTTPRequestNode{}
	if _, err := node.Execute(t.Context(), "", map[string]string{
		"connector": "test-api",
		"url":       "stats", // no leading slash; must join under the prefix
	}); err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if captured["path"] != "/v1/stats" {
		t.Errorf("expected joined path /v1/stats, got %q", captured["path"])
	}
}

func TestHTTPRequest_ConnectorReadOnlyRejectsPOST(t *testing.T) {
	allowLoopback(t)
	setupConnectorRegistry(t, httpConnectorSpec("ro-api", ""))

	node := &HTTPRequestNode{}
	_, err := node.Execute(t.Context(), "", map[string]string{
		"connector": "ro-api",
		"url":       "/items",
		"method":    "POST",
	})
	if err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("expected read-only rejection, got %v", err)
	}
}

func TestHTTPRequest_ConnectorWritableAllowsPOST(t *testing.T) {
	allowLoopback(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		_, _ = w.Write([]byte("created"))
	}))
	defer srv.Close()

	writable := false
	spec := httpConnectorSpec("rw-api", "")
	spec.BaseURL = srv.URL
	spec.ReadOnly = &writable
	setupConnectorRegistry(t, spec)

	node := &HTTPRequestNode{}
	out, err := node.Execute(t.Context(), "", map[string]string{
		"connector": "rw-api",
		"url":       "/items",
		"method":    "POST",
		"body":      `{"x":1}`,
	})
	if err != nil {
		t.Fatalf("writable connector should allow POST: %v", err)
	}
	if !strings.Contains(out, "HTTP 200") {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestHTTPRequest_ConnectorAbsolutePathRejected(t *testing.T) {
	allowLoopback(t)
	setupConnectorRegistry(t, httpConnectorSpec("test-api", ""))

	node := &HTTPRequestNode{}
	_, err := node.Execute(t.Context(), "", map[string]string{
		"connector": "test-api",
		"url":       "https://evil.example.com/exfil",
	})
	if err == nil || !strings.Contains(err.Error(), "relative path") {
		t.Fatalf("expected relative-path rejection, got %v", err)
	}
}

func TestHTTPRequest_ConnectorProtocolRelativePathRejected(t *testing.T) {
	allowLoopback(t)
	setupConnectorRegistry(t, httpConnectorSpec("test-api", ""))

	node := &HTTPRequestNode{}
	_, err := node.Execute(t.Context(), "", map[string]string{
		"connector": "test-api",
		"url":       "//evil.example.com/exfil",
	})
	if err == nil || !strings.Contains(err.Error(), "relative path") {
		t.Fatalf("expected relative-path rejection, got %v", err)
	}
}

func TestHTTPRequest_ConnectorNotRegistered(t *testing.T) {
	t.Setenv("AFLARE_CONNECTORS_FILE", t.TempDir()+"/connectors.yaml")

	node := &HTTPRequestNode{}
	_, err := node.Execute(t.Context(), "", map[string]string{
		"connector": "no-such",
		"url":       "/x",
	})
	if err == nil || !strings.Contains(err.Error(), "not registered") {
		t.Fatalf("expected not-registered error, got %v", err)
	}
}

func TestHTTPRequest_ConnectorWrongType(t *testing.T) {
	setupConnectorRegistry(t, sqliteConnectorSpec("test-db"))

	node := &HTTPRequestNode{}
	_, err := node.Execute(t.Context(), "", map[string]string{
		"connector": "test-db",
		"url":       "/x",
	})
	if err == nil || !strings.Contains(err.Error(), "expects an http connector") {
		t.Fatalf("expected wrong-type error, got %v", err)
	}
}

func TestHTTPRequest_ConnectorAuthHeaderConflict(t *testing.T) {
	allowLoopback(t)
	t.Setenv("TEST_API_CREDENTIAL", "secret-token")

	captured := map[string]string{}
	srv := recordingServer(t, &captured)
	spec := httpConnectorSpec("test-api", connector.AuthTypeBearer)
	spec.BaseURL = srv.URL
	setupConnectorRegistry(t, spec)

	node := &HTTPRequestNode{}
	_, err := node.Execute(t.Context(), "", map[string]string{
		"connector": "test-api",
		"url":       "/v1/stats",
		"headers":   "Authorization: Bearer my-own-token",
	})
	if err == nil || !strings.Contains(err.Error(), "connector injects it") {
		t.Fatalf("expected auth conflict error, got %v", err)
	}
}

func TestHTTPRequest_ConnectorMissingCredential(t *testing.T) {
	allowLoopback(t)
	// TEST_API_CREDENTIAL deliberately unset.

	captured := map[string]string{}
	srv := recordingServer(t, &captured)
	spec := httpConnectorSpec("test-api", connector.AuthTypeBearer)
	spec.BaseURL = srv.URL
	setupConnectorRegistry(t, spec)

	node := &HTTPRequestNode{}
	_, err := node.Execute(t.Context(), "", map[string]string{
		"connector": "test-api",
		"url":       "/v1/stats",
	})
	if err == nil || !strings.Contains(err.Error(), "credential") {
		t.Fatalf("expected credential resolution error, got %v", err)
	}
}
