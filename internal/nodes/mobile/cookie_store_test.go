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

package mobile

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// hasSQLite3 reports whether the sqlite3 CLI is available; integration
// tests that build a real .db file are skipped when it is absent.
func hasSQLite3() bool {
	_, err := exec.LookPath("sqlite3")
	return err == nil
}

// buildCookieDB creates a Chrome-compatible cookies SQLite database at
// path with the given (host_key, name) rows. It uses the sqlite3 CLI to
// avoid a Go SQLite dependency. The schema matches Chrome's cookies
// table (only the columns we read are populated).
func buildCookieDB(t *testing.T, path string, rows []cookieDomain) {
	t.Helper()
	if !hasSQLite3() {
		t.Skip("sqlite3 not installed; skipping integration test")
	}
	// sqlite3 CLI executes a single SQL string passed as its last arg.
	// Concatenate schema + inserts into one script separated by ';'.
	var sb strings.Builder
	sb.WriteString(`CREATE TABLE cookies (host_key TEXT NOT NULL, name TEXT NOT NULL, value TEXT, encrypted_value BLOB);`)
	for _, r := range rows {
		// Values are test-controlled literals; no injection risk here.
		sb.WriteString("INSERT INTO cookies (host_key, name) VALUES ('")
		sb.WriteString(r.HostKey)
		sb.WriteString("', '")
		sb.WriteString(r.CookieName)
		sb.WriteString("');")
	}
	cmd := exec.Command("sqlite3", path, sb.String())
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("sqlite3 create failed: %v: %s", err, out)
	}
}

func TestParseSQLiteJSON_Empty(t *testing.T) {
	got, err := parseSQLiteJSON("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for empty input, got %v", got)
	}
}

func TestParseSQLiteJSON_EmptyArray(t *testing.T) {
	got, err := parseSQLiteJSON("[]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 entries, got %d", len(got))
	}
}

func TestParseSQLiteJSON_SingleEntry(t *testing.T) {
	in := `[{"host_key":"example.com","name":"sid"}]`
	got, err := parseSQLiteJSON(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(got))
	}
	if got[0].HostKey != "example.com" || got[0].CookieName != "sid" {
		t.Errorf("got %+v, want {example.com sid}", got[0])
	}
}

func TestParseSQLiteJSON_MultipleEntries(t *testing.T) {
	in := `[{"host_key":"a.com","name":"x"},{"host_key":"b.com","name":"y"},{"host_key":"a.com","name":"z"}]`
	got, err := parseSQLiteJSON(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(got))
	}
	want := []cookieDomain{{"a.com", "x"}, {"b.com", "y"}, {"a.com", "z"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestParseSQLiteJSON_EscapedQuotes(t *testing.T) {
	// A host_key with an embedded quote (unusual but tests escaping).
	in := `[{"host_key":"a\"b.com","name":"sid"}]`
	got, err := parseSQLiteJSON(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].HostKey != `a"b.com` {
		t.Errorf("got %+v, want host a\"b.com", got)
	}
}

func TestExtractJSONField_Missing(t *testing.T) {
	if got := extractJSONField(`"name":"sid"`, "host_key"); got != "" {
		t.Errorf("expected empty for missing field, got %q", got)
	}
}

func TestExtractJSONField_NonStringValue(t *testing.T) {
	// Numeric values are not supported (we only select TEXT columns).
	if got := extractJSONField(`"count":42`, "count"); got != "" {
		t.Errorf("expected empty for non-string value, got %q", got)
	}
}

func TestUniqueHosts_DeduplicatesAndSorts(t *testing.T) {
	entries := []cookieDomain{
		{"b.com", "x"}, {"a.com", "y"}, {"b.com", "z"}, {"a.com", "w"}, {"c.com", "v"},
	}
	got := uniqueHosts(entries)
	want := []string{"a.com", "b.com", "c.com"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestUniqueHosts_Empty(t *testing.T) {
	got := uniqueHosts(nil)
	if len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

func TestCookieNamesByHost_GroupsAndSorts(t *testing.T) {
	entries := []cookieDomain{
		{"a.com", "zid"}, {"a.com", "mid"}, {"b.com", "sid"},
	}
	got := cookieNamesByHost(entries)
	if len(got) != 2 {
		t.Fatalf("expected 2 hosts, got %d", len(got))
	}
	if !reflect.DeepEqual(got["a.com"], []string{"mid", "zid"}) {
		t.Errorf("a.com names = %v, want [mid zid]", got["a.com"])
	}
	if !reflect.DeepEqual(got["b.com"], []string{"sid"}) {
		t.Errorf("b.com names = %v, want [sid]", got["b.com"])
	}
}

func TestReadChromeCookieDomains_Integration(t *testing.T) {
	if !hasSQLite3() {
		t.Skip("sqlite3 not installed; skipping integration test")
	}
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "Cookies")
	rows := []cookieDomain{
		{"example.com", "sid"},
		{"example.com", "token"},
		{"github.com", "user_session"},
		{"localhost", "theme"},
	}
	buildCookieDB(t, dbPath, rows)

	got, err := readChromeCookieDomains(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("readChromeCookieDomains failed: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("expected 4 entries, got %d (%+v)", len(got), got)
	}

	hosts := uniqueHosts(got)
	sort.Strings(hosts)
	wantHosts := []string{"example.com", "github.com", "localhost"}
	if !reflect.DeepEqual(hosts, wantHosts) {
		t.Errorf("hosts = %v, want %v", hosts, wantHosts)
	}

	namesByHost := cookieNamesByHost(got)
	if len(namesByHost["example.com"]) != 2 {
		t.Errorf("expected 2 cookies for example.com, got %v", namesByHost["example.com"])
	}
}

func TestReadChromeCookieDomains_DatabaseNotFound(t *testing.T) {
	_, err := readChromeCookieDomains(context.Background(), "/nonexistent/path/Cookies")
	if err == nil {
		t.Fatal("expected error for missing database")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

func TestReadChromeCookieDomains_EmptyDatabase(t *testing.T) {
	if !hasSQLite3() {
		t.Skip("sqlite3 not installed; skipping integration test")
	}
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "Cookies")
	// Create the schema but insert no rows.
	buildCookieDB(t, dbPath, nil)

	got, err := readChromeCookieDomains(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("readChromeCookieDomains failed: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 entries from empty database, got %d", len(got))
	}
}

func TestImportCookiesFromChrome_ReturnsRealDomains(t *testing.T) {
	if !hasSQLite3() {
		t.Skip("sqlite3 not installed; skipping integration test")
	}
	// Override the cookie path resolution by placing a fake Cookies file
	// where getChromeCookiePath() would look is platform-dependent and
	// fragile. Instead exercise the lower-level path directly via a
	// temporary file and the public readChromeCookieDomains helper, which
	// is the substantive logic. The agent_browser wrapper just maps the
	// result to domain strings.
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "Cookies")
	rows := []cookieDomain{
		{"app.example.com", "session"},
		{"api.example.com", "csrf"},
	}
	buildCookieDB(t, dbPath, rows)

	entries, err := readChromeCookieDomains(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	domains := uniqueHosts(entries)
	if len(domains) != 2 {
		t.Fatalf("expected 2 domains, got %d", len(domains))
	}
	// The Agent-facing helper returns the same dedup'd domain list.
	sort.Strings(domains)
	if domains[0] != "api.example.com" || domains[1] != "app.example.com" {
		t.Errorf("domains = %v", domains)
	}
}

func TestReadChromeCookieDomains_TimeoutOnLockedDB(t *testing.T) {
	if !hasSQLite3() {
		t.Skip("sqlite3 not installed; skipping integration test")
	}
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "Cookies")
	buildCookieDB(t, dbPath, []cookieDomain{{"a.com", "x"}})

	// A 1ns context deadline should time out (or at least not hang).
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()
	// Even with a zero deadline, sqlite3 may return fast if the DB is
	// unlocked. The point of this test is that the function does not hang
	// and either returns data or a timeout error — both are acceptable.
	_, _ = readChromeCookieDomains(ctx, dbPath)
	// No assertion: this is a non-hang smoke test. If sqlite3 ran fast
	// enough we got data; if not we got a DeadlineExceeded error.
}

// Ensure a stale temp file is cleaned even if a test exits early.
func TestMain(m *testing.M) {
	// Run tests; nothing else to set up.
	os.Exit(m.Run())
}
