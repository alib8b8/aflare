// Copyright (c) 2026 llm-box Contributors
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

// Security helpers re-exported from internal/nodes/core. The actual
// implementations live in core/security.go so that sub-packages under
// internal/nodes/ can use them without creating import cycles.
package nodes

import (
	"net"
	"net/http"

	"github.com/alib8b8/llm-box/internal/nodes/core"
)

// DefaultLLMTimeout is the default timeout for LLM HTTP requests.
const DefaultLLMTimeout = core.DefaultLLMTimeout

// workDir is the cached working directory used by some node files
// (code_graph.go, code_knowledge_graph.go) that need to resolve paths
// relative to the cwd. The value is sourced from core at package init
// time (after core's own init runs).
var workDir = core.GetWorkDir()

// MaxHTTPResponseSize bounds how much of an HTTP response body the nodes
// are willing to read.
const MaxHTTPResponseSize = core.MaxHTTPResponseSize

// maxHTTPResponseSize is the package-private alias kept for backward
// compatibility with existing node files (http_request.go, json_parse.go,
// fastgpt.go, search_aggregate.go) that reference the lowercase name.
const maxHTTPResponseSize = MaxHTTPResponseSize

// safeHTTPClient is a shared HTTP client with SSRF protection at dial time.
var safeHTTPClient = core.SafeHTTPClient

// safeLLMHTTPClient is like safeHTTPClient but allows loopback for local LLMs.
var safeLLMHTTPClient = core.SafeLLMHTTPClient

// httpRedirectValidator returns a CheckRedirect function that validates
// each redirect target with the given validator.
func httpRedirectValidator(validator func(string) error) func(*http.Request, []*http.Request) error {
	return core.HTTPRedirectValidator(validator)
}

// safeJoinPath joins userPath onto baseDir after validating the result
// stays within baseDir.
func safeJoinPath(baseDir, userPath string) (string, error) {
	return core.SafeJoinPath(baseDir, userPath)
}

// validateReadPath validates that path is safe to read from within the
// current working directory. It deliberately routes through the package-
// level workDir var (rather than core.ValidateReadPath) so that tests
// which reassign workDir = tmpDir take effect locally.
func validateReadPath(path string) (string, error) {
	return core.ValidateReadPathIn(workDir, path)
}

// validateWritePath validates that path is safe to write to within the
// current working directory. As with validateReadPath, it uses the
// package-level workDir so test reassignments propagate.
func validateWritePath(path string) (string, error) {
	return core.ValidateWritePathIn(workDir, path)
}

// redactAPIKey masks an API key, showing only the first 4 and last 4 chars.
func redactAPIKey(key string) string {
	return core.RedactAPIKey(key)
}

// isSensitiveKey reports whether key looks like it holds a secret.
func isSensitiveKey(key string) bool {
	return core.IsSensitiveKey(key)
}

// RedactSensitive masks known secret patterns in s.
func RedactSensitive(s string) string {
	return core.RedactSensitive(s)
}

// validateURL checks if a URL is safe to request (SSRF protection).
func validateURL(rawURL string) error {
	return core.ValidateURL(rawURL)
}

// validateIP reports whether ip is safe to connect to.
func validateIP(ip net.IP, displayHost string) error {
	return core.ValidateIP(ip, displayHost)
}

// validateLMLEndpoint validates an LLM API endpoint URL (allows loopback).
func validateLMLEndpoint(rawURL string) error {
	return core.ValidateLMLEndpoint(rawURL)
}

// validateLMLEndpointIP validates an IP for LLM endpoints. Loopback allowed.
func validateLMLEndpointIP(ip net.IP, displayHost string) error {
	return core.ValidateLMLEndpointIP(ip, displayHost)
}

// isReservedIP reports whether ip falls in a reserved range.
func isReservedIP(ip net.IP) bool {
	return core.IsReservedIP(ip)
}
