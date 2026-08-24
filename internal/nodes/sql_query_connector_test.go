// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​​​‌‌​‌‌​‌‌‌‌​​‌​‌‌‌‌​​​‌​‌‌​​​‌‌‌‌‌‌​‌​‌​​‌​‌‌‌​​‌​‌​‌​​​‌‌‌​​‌​‌​​​​​​​​​​​​​​​​​‌‌​‌​​​‌​​​‌​‌‌⁠
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
	"database/sql"
	"database/sql/driver"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/alib8b8/aflare/internal/connector"
)

// stubSQLite is a database/sql driver registered under the name "sqlite3"
// for these tests. It records the DSN it was opened with and every query
// it was asked to prepare, and fails all prepares with a sentinel error.
// This proves end-to-end connector resolution (spec → DSN → driver)
// without any CGO dependency.
var stubSQLite = &stubDriver{}

type stubDriver struct {
	mu        sync.Mutex
	lastDSN   string
	lastQuery string
}

func (d *stubDriver) Open(dsn string) (driver.Conn, error) {
	d.mu.Lock()
	d.lastDSN = dsn
	d.mu.Unlock()
	return &stubConn{d: d}, nil
}

func (d *stubDriver) recordedDSN() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.lastDSN
}

func (d *stubDriver) recordedQuery() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.lastQuery
}

type stubConn struct {
	d *stubDriver
}

func (c *stubConn) Prepare(query string) (driver.Stmt, error) {
	c.d.mu.Lock()
	c.d.lastQuery = query
	c.d.mu.Unlock()
	return nil, errors.New("stub: prepare not supported")
}

func (c *stubConn) Close() error              { return nil }
func (c *stubConn) Begin() (driver.Tx, error) { return nil, errors.New("stub: tx not supported") }

func init() {
	// "sqlite3" is the driver name connector.DriverName maps the sqlite
	// type to; registering it here lets connector-mode tests run through
	// the real openDB/dbCache path.
	sql.Register("sqlite3", stubSQLite)
}

// setupConnectorRegistry points the connector registry at a temp file and
// registers the given spec. Tests must call this before executing the
// node with a `connector` param.
func setupConnectorRegistry(t *testing.T, spec connector.Spec) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "connectors.yaml")
	t.Setenv("AFLARE_CONNECTORS_FILE", path)
	reg, err := connector.LoadRegistry(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.Upsert(spec); err != nil {
		t.Fatal(err)
	}
	if err := reg.Save(); err != nil {
		t.Fatal(err)
	}
}

func sqliteConnectorSpec(name string) connector.Spec {
	return connector.Spec{Name: name, Type: connector.TypeSQLite, Database: "/tmp/connector-test.db"}
}

func TestSQLQueryNode_Execute_ConnectorResolvesDSN(t *testing.T) {
	setupConnectorRegistry(t, sqliteConnectorSpec("test-db"))

	n := &SQLQueryNode{}
	_, err := n.Execute(t.Context(), "", map[string]string{
		"connector": "test-db",
		"sql":       "SELECT 1",
	})
	if err == nil || !strings.Contains(err.Error(), "stub") {
		t.Fatalf("expected stub driver sentinel error, got %v", err)
	}
	// The DSN that reached the driver must be the spec's database path
	// in read-only mode (sqlite connectors are read-only by default) —
	// proving driver+dsn came from the connector, not inline params.
	if got, want := stubSQLite.recordedDSN(), "file:/tmp/connector-test.db?mode=ro"; got != want {
		t.Errorf("driver opened DSN %q, want %q", got, want)
	}
	if got := stubSQLite.recordedQuery(); got != "SELECT 1" {
		t.Errorf("driver saw query %q, want SELECT 1", got)
	}
}

func TestSQLQueryNode_Execute_ConnectorAndInlineConflict(t *testing.T) {
	n := &SQLQueryNode{}
	_, err := n.Execute(t.Context(), "", map[string]string{
		"connector": "test-db",
		"driver":    "sqlite3",
		"dsn":       ":memory:",
	})
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("expected mutually-exclusive error, got %v", err)
	}
	// dsn alone is also a conflict
	_, err = n.Execute(t.Context(), "", map[string]string{
		"connector": "test-db",
		"dsn":       ":memory:",
	})
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("expected mutually-exclusive error for dsn, got %v", err)
	}
}

func TestSQLQueryNode_Execute_ConnectorUnknown(t *testing.T) {
	t.Setenv("AFLARE_CONNECTORS_FILE", filepath.Join(t.TempDir(), "connectors.yaml"))
	n := &SQLQueryNode{}
	_, err := n.Execute(t.Context(), "", map[string]string{
		"connector": "no-such-connector",
		"sql":       "SELECT 1",
	})
	if err == nil || !strings.Contains(err.Error(), "not registered") {
		t.Errorf("expected not-registered error, got %v", err)
	}
}

func TestSQLQueryNode_Execute_ConnectorReadOnlyCeiling(t *testing.T) {
	// Spec leaves read_only unset → connector is read-only. The node
	// asks for read_only=false, but connector policy must win.
	setupConnectorRegistry(t, sqliteConnectorSpec("ro-db"))

	n := &SQLQueryNode{}
	_, err := n.Execute(t.Context(), "", map[string]string{
		"connector": "ro-db",
		"sql":       "DROP TABLE t",
		"read_only": "false",
	})
	if err == nil || !strings.Contains(err.Error(), "read_only") {
		t.Fatalf("expected read_only rejection, got %v", err)
	}
	if got := stubSQLite.recordedQuery(); got == "DROP TABLE t" {
		t.Error("write query must not reach the driver on a read-only connector")
	}
}

func TestSQLQueryNode_Execute_ConnectorWritable(t *testing.T) {
	// Explicitly writable connector + node opt-in → the query reaches
	// the driver.
	writable := false
	spec := sqliteConnectorSpec("rw-db")
	spec.ReadOnly = &writable
	setupConnectorRegistry(t, spec)

	n := &SQLQueryNode{}
	_, err := n.Execute(t.Context(), "", map[string]string{
		"connector": "rw-db",
		"sql":       "DROP TABLE t",
		"read_only": "false",
	})
	if err == nil || !strings.Contains(err.Error(), "stub") {
		t.Fatalf("expected stub sentinel error (query should reach driver), got %v", err)
	}
	if got := stubSQLite.recordedQuery(); got != "DROP TABLE t" {
		t.Errorf("driver saw query %q, want DROP TABLE t", got)
	}
}

func TestSQLQueryNode_Execute_ConnectorCredentialFailure(t *testing.T) {
	// A postgres connector whose env credential is unset must fail at
	// resolution time with a clear error, before any driver call.
	spec := connector.Spec{
		Name:       "pg-cred",
		Type:       connector.TypePostgres,
		Host:       "db.example.com",
		Database:   "app",
		Credential: &connector.CredentialRef{Kind: connector.CredentialKindEnv, Key: "AFLARE_TEST_CONN_DEFINITELY_UNSET"},
	}
	setupConnectorRegistry(t, spec)

	n := &SQLQueryNode{}
	_, err := n.Execute(t.Context(), "", map[string]string{
		"connector": "pg-cred",
		"sql":       "SELECT 1",
	})
	if err == nil || !strings.Contains(err.Error(), "credential") {
		t.Errorf("expected credential resolution error, got %v", err)
	}
}

func TestApplyConnectorCeiling(t *testing.T) {
	cases := []struct {
		name             string
		nodeVal, ceiling int
		nodeSet          bool
		want             int
	}{
		{"unset adopts connector ceiling", 1000, 500, false, 500},
		{"set value below ceiling kept", 100, 500, true, 100},
		{"set value above ceiling capped", 900, 500, true, 500},
		{"equal", 500, 500, true, 500},
	}
	for _, c := range cases {
		got := applyConnectorCeiling(c.nodeVal, c.nodeSet, c.ceiling)
		if got != c.want {
			t.Errorf("%s: applyConnectorCeiling(%d, %v, %d) = %d, want %d",
				c.name, c.nodeVal, c.nodeSet, c.ceiling, got, c.want)
		}
	}
}
