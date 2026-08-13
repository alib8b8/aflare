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

package nodes

import (
	"database/sql"
	"testing"
)

func TestSQLQueryNode_Metadata(t *testing.T) {
	n := &SQLQueryNode{}
	if n.Name() != "sql_query" {
		t.Errorf("Name() = %q, want sql_query", n.Name())
	}
	s := n.Schema()
	if s.Name != "sql_query" {
		t.Errorf("Schema().Name = %q, want sql_query", s.Name)
	}
	// Ensure required params are declared.
	var hasDriver, hasDSN bool
	for _, p := range s.Params {
		if p.Name == "driver" && p.Required {
			hasDriver = true
		}
		if p.Name == "dsn" && p.Required {
			hasDSN = true
		}
	}
	if !hasDriver {
		t.Error("schema missing required 'driver' param")
	}
	if !hasDSN {
		t.Error("schema missing required 'dsn' param")
	}
}

func TestIsReadOnlySQL(t *testing.T) {
	cases := []struct {
		stmt string
		want bool
	}{
		{"SELECT 1", true},
		{"select * from t", true},
		{"  WITH cte AS (SELECT 1) SELECT * FROM cte", true},
		{"SHOW TABLES", true},
		{"EXPLAIN SELECT * FROM t", true},
		{"PRAGMA table_info(t)", true},
		{"DESCRIBE t", true},
		{"DESC t", true},
		{"INSERT INTO t VALUES (1)", false},
		{"UPDATE t SET x=1", false},
		{"DELETE FROM t", false},
		{"DROP TABLE t", false},
		{"CREATE TABLE t (x int)", false},
		{"ALTER TABLE t ADD COLUMN y int", false},
		{"", false},
		{"   ", false},
		{"TRUNCATE t", false},
	}
	for _, c := range cases {
		got := isReadOnlySQL(c.stmt)
		if got != c.want {
			t.Errorf("isReadOnlySQL(%q) = %v, want %v", c.stmt, got, c.want)
		}
	}
}

func TestDBCacheKey(t *testing.T) {
	got := dbCacheKey("postgres", "host=localhost")
	want := "postgres|host=localhost"
	if got != want {
		t.Errorf("dbCacheKey = %q, want %q", got, want)
	}
}

func TestOpenDB_MissingParams(t *testing.T) {
	_, err := openDB("", "")
	if err == nil {
		t.Error("openDB('', '') should error")
	}
	_, err = openDB("sqlite3", "")
	if err == nil {
		t.Error("openDB('sqlite3', '') should error")
	}
	_, err = openDB("", ":memory:")
	if err == nil {
		t.Error("openDB('', ':memory:') should error")
	}
}

func TestOpenDB_UnknownDriver(t *testing.T) {
	// sql.Open rejects an unregistered driver immediately (Go's
	// database/sql validates the driver name at Open time).
	_, err := openDB("definitely-not-a-real-driver", ":memory:")
	if err == nil {
		t.Fatal("openDB with unknown driver should error")
	}
}

func TestScanRows(t *testing.T) {
	// Build a *sql.Rows manually is hard; instead, exercise the helper
	// via an in-memory driver. Since we cannot import a CGO sqlite
	// driver in unit tests, we skip the rows-scanning path here. The
	// isReadOnlySQL and openDB tests above already cover the pure logic.
	t.Skip("requires a registered database driver; covered by integration")
}

func TestSQLQueryNode_Execute_MissingDriver(t *testing.T) {
	n := &SQLQueryNode{}
	_, err := n.Execute(t.Context(), "", map[string]string{"sql": "SELECT 1"})
	if err == nil {
		t.Error("expected error when driver/dsn missing")
	}
}

func TestSQLQueryNode_Execute_UnknownAction(t *testing.T) {
	n := &SQLQueryNode{}
	_, err := n.Execute(t.Context(), "", map[string]string{
		"driver": "sqlite3",
		"dsn":    ":memory:",
		"action": "bogus",
	})
	if err == nil {
		t.Error("expected error for unknown action")
	}
}

func TestSQLQueryNode_Execute_EmptyQuery(t *testing.T) {
	n := &SQLQueryNode{}
	_, err := n.Execute(t.Context(), "  ", map[string]string{
		"driver": "sqlite3",
		"dsn":    ":memory:",
		"action": "query",
	})
	if err == nil {
		t.Error("expected error when query is empty")
	}
}

func TestSQLQueryNode_Execute_ReadOnlyRejectsWrite(t *testing.T) {
	n := &SQLQueryNode{}
	_, err := n.Execute(t.Context(), "", map[string]string{
		"driver":    "sqlite3",
		"dsn":       ":memory:",
		"sql":       "DROP TABLE t",
		"read_only": "true",
	})
	if err == nil {
		t.Error("expected read_only to reject DROP TABLE")
	}
}

func TestSQLQueryNode_Execute_InvalidArgs(t *testing.T) {
	n := &SQLQueryNode{}
	_, err := n.Execute(t.Context(), "", map[string]string{
		"driver": "sqlite3",
		"dsn":    ":memory:",
		"sql":    "SELECT 1",
		"args":   "{not valid json",
	})
	if err == nil {
		t.Error("expected error for invalid args JSON")
	}
}

// Silence unused import warnings if the test build adds helpers later.
var _ = sql.ErrNoRows
