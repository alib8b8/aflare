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

package mobile

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"
)

// cookieDomain is a single (host_key, name) pair extracted from the
// Chrome Cookies SQLite database. Values are intentionally NOT read: Chrome
// encrypts cookie values with an OS-level key (DPAPI on Windows, Keychain
// on macOS, kwallet/gnome-keyring on Linux), and decrypting them requires
// platform-specific native calls. For Agent use cases (knowing which
// sites are logged in) the domain + cookie names are sufficient; the
// browser itself re-loads real cookie values when reusing the profile.
type cookieDomain struct {
	HostKey    string
	CookieName string
}

// readChromeCookieDomains queries the Chrome Cookies SQLite database at
// cookiePath and returns the list of (host_key, name) pairs. It prefers
// the `sqlite3` CLI (zero Go dependencies, present on macOS/Linux and
// most dev machines) and returns a clear error if sqlite3 is unavailable.
//
// The query is read-only and scoped to the cookies table. It does not
// decrypt encrypted_values; only the plaintext host_key and name columns
// are read (both are stored unencrypted by Chrome).
func readChromeCookieDomains(ctx context.Context, cookiePath string) ([]cookieDomain, error) {
	if _, err := os.Stat(cookiePath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("cookie database not found: %s", cookiePath)
		}
		return nil, fmt.Errorf("cannot access cookie database: %w", err)
	}

	// Look for sqlite3 in PATH. On Windows it is not bundled by default;
	// callers get a actionable error rather than a silent failure.
	binary, err := exec.LookPath("sqlite3")
	if err != nil {
		return nil, fmt.Errorf("sqlite3 not found in PATH (required to read Chrome cookies on %s)", runtime.GOOS)
	}

	// Read-only query: distinct host_key, name. host_key and name are
	// plaintext columns in Chrome's cookies table; only the `value` and
	// `encrypted_value` columns are sensitive and we do not select them.
	// The database may be locked by a running Chrome; use a short timeout
	// and immutable read mode (-readonly).
	query := "SELECT host_key, name FROM cookies;"
	// #nosec G204 -- binary is from LookPath("sqlite3"); cookiePath is
	// validated by the caller; query is a fixed string.
	cmd := exec.CommandContext(ctx, binary, "-readonly", "-json", cookiePath, query)
	cmd.Env = append(os.Environ(), "SQLITE_OPEN_READONLY=1")

	// Chrome holds a write lock on the database while running. Bound the
	// wait so a locked DB does not hang the workflow.
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		cmd = exec.CommandContext(ctx, binary, "-readonly", "-json", cookiePath, query)
	}

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		stderrText := strings.TrimSpace(stderr.String())
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("sqlite3 timed out (Chrome may be running and holding a lock); close Chrome and retry")
		}
		if stderrText != "" {
			return nil, fmt.Errorf("sqlite3 query failed: %w: %s", err, stderrText)
		}
		return nil, fmt.Errorf("sqlite3 query failed: %w", err)
	}

	return parseSQLiteJSON(stdout.String())
}

// parseSQLiteJSON parses the JSON array emitted by `sqlite3 -json` where
// each element is an object with the selected columns as keys.
// Expected format: [{"host_key":"example.com","name":"sid"}, ...]
// The output may be pretty-printed (one object per line); whitespace is
// collapsed before splitting.
func parseSQLiteJSON(s string) ([]cookieDomain, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	// sqlite3 -json may emit one object per line. Collapse all whitespace
	// so we can split reliably on "},{" boundaries.
	s = collapseWhitespace(s)
	// After collapsing, "},{" may have become "}, {" — normalize so the
	// split on "},{" matches every object boundary.
	s = strings.ReplaceAll(s, "}, {", "},{")
	// sqlite3 -json outputs a JSON array of objects. We avoid importing
	// encoding/json here to keep the mobile package lean and because the
	// shape is simple and well-defined; a lightweight scan is sufficient
	// and robust to column-order changes.
	var results []cookieDomain
	inner := strings.TrimPrefix(s, "[")
	inner = strings.TrimSuffix(inner, "]")
	inner = strings.TrimSpace(inner)
	if inner == "" {
		return nil, nil
	}
	objects := strings.Split(inner, "},{")
	for i, obj := range objects {
		if i == 0 {
			obj = strings.TrimPrefix(obj, "{")
		}
		if i == len(objects)-1 {
			obj = strings.TrimSuffix(obj, "}")
		}
		host := extractJSONField(obj, "host_key")
		name := extractJSONField(obj, "name")
		if host != "" {
			results = append(results, cookieDomain{HostKey: host, CookieName: name})
		}
	}
	return results, nil
}

// collapseWhitespace replaces any run of whitespace (spaces, newlines,
// tabs) with a single space. JSON structural characters are not
// whitespace-sensitive, so this is safe for parsing.
func collapseWhitespace(s string) string {
	var sb strings.Builder
	sb.Grow(len(s))
	inString := false
	prevSpace := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '"' && (i == 0 || s[i-1] != '\\') {
			inString = !inString
		}
		if !inString && (c == ' ' || c == '\n' || c == '\r' || c == '\t') {
			if !prevSpace {
				sb.WriteByte(' ')
				prevSpace = true
			}
			continue
		}
		prevSpace = false
		sb.WriteByte(c)
	}
	return sb.String()
}

// extractJSONField pulls a string field value out of a flat JSON object
// string like `"host_key":"example.com","name":"sid"`. It handles basic
// escaped quotes. This is deliberately minimal — sqlite3 -json output is
// predictable and we only need two string columns.
func extractJSONField(obj, field string) string {
	key := `"` + field + `":`
	idx := strings.Index(obj, key)
	if idx < 0 {
		return ""
	}
	rest := obj[idx+len(key):]
	// rest starts with the opening quote of the value.
	if !strings.HasPrefix(rest, `"`) {
		return ""
	}
	rest = rest[1:]
	var sb strings.Builder
	for i := 0; i < len(rest); i++ {
		c := rest[i]
		if c == '\\' && i+1 < len(rest) {
			// copy escape verbatim
			sb.WriteByte(c)
			sb.WriteByte(rest[i+1])
			i++
			continue
		}
		if c == '"' {
			break
		}
		sb.WriteByte(c)
	}
	// unescape basic sequences
	s := sb.String()
	s = strings.ReplaceAll(s, `\"`, `"`)
	s = strings.ReplaceAll(s, `\\`, `\`)
	s = strings.ReplaceAll(s, `\n`, "\n")
	s = strings.ReplaceAll(s, `\t`, "\t")
	return s
}

// uniqueHosts returns the sorted unique set of host_key values from a
// list of cookieDomain entries. Hosts are deduplicated so the caller can
// report "N sites logged in" without double-counting.
func uniqueHosts(entries []cookieDomain) []string {
	seen := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		seen[e.HostKey] = struct{}{}
	}
	hosts := make([]string, 0, len(seen))
	for h := range seen {
		hosts = append(hosts, h)
	}
	sort.Strings(hosts)
	return hosts
}

// cookieNamesByHost groups cookie names by host_key, returning a map for
// structured reporting (e.g. "example.com has cookies: sid, token").
func cookieNamesByHost(entries []cookieDomain) map[string][]string {
	m := make(map[string][]string, len(entries))
	for _, e := range entries {
		m[e.HostKey] = append(m[e.HostKey], e.CookieName)
	}
	for h := range m {
		sort.Strings(m[h])
	}
	return m
}
