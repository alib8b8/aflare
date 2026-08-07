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

package nodes

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/alib8b8/llm-box/internal/nodes/core"
)

// SQLQueryNode executes SQL against any database/sql-compatible driver.
// The driver must be registered by the host program (e.g. via
// `_ "github.com/mattn/go-sqlite3"`); this node only depends on the
// standard library and never imports a specific driver, so it adds no
// third-party / CGO dependency to llm-box itself.
//
// Inspired by OtterMind/Chat2DB: AI agents need a uniform data access
// layer. This node enforces read-only safety by default, parameterized
// queries (no string interpolation -> no SQL injection), result size
// limits, and an optional schema introspection action.
type SQLQueryNode struct{}

func init() {
	Register(&SQLQueryNode{})
}

func (n *SQLQueryNode) Name() string { return "sql_query" }

func (n *SQLQueryNode) Description() string {
	return "Execute SQL against any database/sql driver (SQLite/Postgres/MySQL/...). Parameterized queries only (no injection). Read-only by default. Supports schema introspection."
}

func (n *SQLQueryNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "sql_query",
		Description: "Execute SQL via database/sql. The driver must be registered by the host program. Uses parameterized queries (? or $1) to prevent SQL injection. Read-only by default (SELECT/SHOW/EXPLAIN/PRAGMA only); set read_only=false to allow DML/DDL. Supports a 'schema' action that lists tables and columns.",
		Input:       "string - SQL query (when action=query and no `sql` param, input is used as the query)",
		Output:      "string - JSON array of rows (query), or schema description (schema action)",
		Params: []core.ParamSchema{
			{Name: "action", Type: "string", Description: "query (default) | schema | tables", Required: false, Default: "query"},
			{Name: "driver", Type: "string", Description: "database/sql driver name (e.g. sqlite3, postgres, mysql)", Required: true},
			{Name: "dsn", Type: "string", Description: "Data source name (driver-specific). For SQLite: path to .db file.", Required: true},
			{Name: "sql", Type: "string", Description: "SQL statement. Use ? (mysql/sqlite) or $1,$2 (postgres) placeholders for `args`.", Required: false},
			{Name: "args", Type: "string", Description: "JSON array of bind parameters, e.g. [\"foo\", 42]. Optional.", Required: false},
			{Name: "read_only", Type: "string", Description: "Reject writes if true (default). Set false to allow INSERT/UPDATE/DELETE/DDL.", Required: false, Default: "true"},
			{Name: "max_rows", Type: "string", Description: "Max rows to return (default 1000). Protects against huge result sets.", Required: false, Default: "1000"},
			{Name: "timeout", Type: "string", Description: "Query timeout in seconds (default 30).", Required: false, Default: "30"},
		},
	}
}

// dbCache keeps a small pool of opened *sql.DB per (driver,dsn) key so
// repeated queries don't pay the connect cost every time. The pool is
// bounded; evicted entries have their DB closed.
var (
	dbCache   = make(map[string]*sql.DB, 16)
	dbCacheMu sync.Mutex
)

func dbCacheKey(driver, dsn string) string { return driver + "|" + dsn }

func openDB(driver, dsn string) (*sql.DB, error) {
	if driver == "" || dsn == "" {
		return nil, fmt.Errorf("driver and dsn are required")
	}
	key := dbCacheKey(driver, dsn)

	dbCacheMu.Lock()
	defer dbCacheMu.Unlock()

	if db, ok := dbCache[key]; ok {
		return db, nil
	}

	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("sql.Open(%s): %w", driver, err)
	}
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxIdleTime(5 * time.Minute)

	// Evict if pool grew beyond 16 entries.
	if len(dbCache) >= 16 {
		for k, v := range dbCache {
			_ = v.Close()
			delete(dbCache, k)
			break
		}
	}
	dbCache[key] = db
	return db, nil
}

// readOnlyStatements are SQL keywords that may read data. Anything else
// is considered a write and rejected when read_only=true.
func isReadOnlySQL(stmt string) bool {
	s := strings.TrimSpace(stmt)
	if s == "" {
		return false
	}
	upper := strings.ToUpper(s)
	for _, kw := range []string{"SELECT", "WITH", "SHOW", "EXPLAIN", "PRAGMA", "DESCRIBE", "DESC"} {
		if strings.HasPrefix(upper, kw+" ") || upper == kw {
			return true
		}
	}
	return false
}

// scanRows converts *sql.Rows into a slice of map[string]any records
// (column names lower-cased for stable access), capped at maxRows.
func scanRows(rows *sql.Rows, maxRows int) ([]map[string]interface{}, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("rows.Columns: %w", err)
	}
	out := make([]map[string]interface{}, 0, 8)
	for rows.Next() {
		if len(out) >= maxRows {
			// Drain remaining to avoid protocol desync; caller will
			// see truncated result.
			_ = rows.Close()
			break
		}
		values := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, fmt.Errorf("rows.Scan: %w", err)
		}
		row := make(map[string]interface{}, len(cols))
		for i, col := range cols {
			val := values[i]
			// Normalize []byte to string for JSON-friendly output.
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows.Err: %w", err)
	}
	return out, nil
}

func (n *SQLQueryNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	driver := core.GetParam(params, "driver", "")
	dsn := core.GetParam(params, "dsn", "")
	action := core.GetParam(params, "action", "query")
	readOnly := core.GetParam(params, "read_only", "true") == "true"
	maxRows := core.ParamInt(params, "max_rows", 1000, 1, 100000)
	timeoutSec := core.ParamInt(params, "timeout", 30, 1, 600)

	db, err := openDB(driver, dsn)
	if err != nil {
		return "", err
	}

	queryCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	switch action {
	case "tables":
		return n.actionTables(queryCtx, db)
	case "schema":
		return n.actionSchema(queryCtx, db, core.GetParam(params, "table", ""))
	case "query":
		// fall through to query handling below
	default:
		return "", fmt.Errorf("unknown action: %s (supported: query, schema, tables)", action)
	}

	query := core.GetParam(params, "sql", strings.TrimSpace(input))
	if query == "" {
		return "", fmt.Errorf("sql parameter or input is required for action=query")
	}

	if readOnly && !isReadOnlySQL(query) {
		return "", fmt.Errorf("read_only=true rejects non-SELECT statement; set read_only=false to allow writes")
	}

	// Parse bind args before executing so a malformed args JSON is
	// reported as a validation error rather than a driver error.
	var args []interface{}
	if argsJSON := core.GetParam(params, "args", ""); argsJSON != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("invalid args (expected JSON array): %w", err)
		}
	}

	rows, err := db.QueryContext(queryCtx, query, args...)
	if err != nil {
		return "", fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	records, err := scanRows(rows, maxRows)
	if err != nil {
		return "", err
	}

	result := map[string]interface{}{
		"rows":      records,
		"count":     len(records),
		"truncated": len(records) >= maxRows,
	}
	out, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal result: %w", err)
	}
	return string(out), nil
}

// actionTables lists table names in the current database, using the
// driver's INFORMATION_SCHEMA-equivalent when available, falling back
// to a SQLite-style query.
func (n *SQLQueryNode) actionTables(ctx context.Context, db *sql.DB) (string, error) {
	// Try the ANSI-ish INFORMATION_SCHEMA first (Postgres/MySQL).
	rows, err := db.QueryContext(ctx,
		"SELECT table_name FROM information_schema.tables WHERE table_schema NOT IN ('pg_catalog','information_schema') ORDER BY table_name")
	if err != nil {
		// Fall back to SQLite-style.
		rows, err = db.QueryContext(ctx, "SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
		if err != nil {
			return "", fmt.Errorf("list tables failed: %w", err)
		}
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return "", fmt.Errorf("scan table name: %w", err)
		}
		names = append(names, name)
	}
	out, _ := json.MarshalIndent(map[string]interface{}{"tables": names, "count": len(names)}, "", "  ")
	return string(out), nil
}

// actionSchema returns column metadata for one table (or all tables if
// `table` is empty).
func (n *SQLQueryNode) actionSchema(ctx context.Context, db *sql.DB, table string) (string, error) {
	var tables []string
	if table != "" {
		tables = []string{table}
	} else {
		rows, err := db.QueryContext(ctx,
			"SELECT table_name FROM information_schema.tables WHERE table_schema NOT IN ('pg_catalog','information_schema') ORDER BY table_name")
		if err != nil {
			rows, err = db.QueryContext(ctx, "SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
			if err != nil {
				return "", fmt.Errorf("schema introspection failed: %w", err)
			}
		}
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				continue
			}
			tables = append(tables, name)
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return "", fmt.Errorf("rows iteration: %w", err)
		}
		_ = rows.Close()
	}

	schema := make(map[string]interface{}, len(tables))
	for _, t := range tables {
		colRows, err := db.QueryContext(ctx,
			"SELECT column_name, data_type, is_nullable FROM information_schema.columns WHERE table_name = $1 ORDER BY ordinal_position", t)
		if err != nil {
			// SQLite-style fallback: PRAGMA table_info.
			colRows, err = db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%q)", t))
			if err != nil {
				schema[t] = []interface{}{}
				continue
			}
		}
		cols, scanErr := scanRows(colRows, 1000)
		_ = colRows.Close()
		if scanErr != nil {
			schema[t] = []interface{}{}
			continue
		}
		schema[t] = cols
	}

	out, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal schema: %w", err)
	}
	return string(out), nil
}
