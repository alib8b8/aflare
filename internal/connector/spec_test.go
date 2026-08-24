// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​​​‌‌​‌‌​‌‌‌‌​​‌​‌‌​‌​‌​‌‌‌​‌‌‌​‌​​‌​​‌‌‌‌​​​‌​‌​​​‌​​‌‌‌​​‌‌‌‌‌‌‌​​​​​​​​​​​​​​​​‌‌‌​​‌​​​‌​​‌​​‌⁠
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

package connector

import (
	"strings"
	"testing"
)

func boolPtr(b bool) *bool { return &b }

func TestSpecValidate_OK(t *testing.T) {
	cases := []Spec{
		{Name: "pg", Type: TypePostgres, Host: "db.example.com", Port: 5432, Database: "app", Username: "ro"},
		{Name: "my-mysql-1", Type: TypeMySQL, Host: "mysql.internal", Database: "app"},
		{Name: "local-db", Type: TypeSQLite, Database: "/data/app.db"},
		{Name: "pg-cred", Type: TypePostgres, Host: "db", Database: "app",
			Credential: &CredentialRef{Kind: CredentialKindSecret, Group: "connectors", Key: "pg-pass"}},
		{Name: "pg-env-cred", Type: TypePostgres, Host: "db", Database: "app",
			Credential: &CredentialRef{Kind: CredentialKindEnv, Key: "PG_PASS"}},
		{Name: "pg-writable", Type: TypePostgres, Host: "db", Database: "app", ReadOnly: boolPtr(false)},
		{Name: "pg-limits", Type: TypePostgres, Host: "db", Database: "app", MaxRows: 500, TimeoutSec: 60},
		{Name: "docs", Type: TypeFiles, Root: "/home/me/Documents"},
		{Name: "docs-inc", Type: TypeFiles, Root: "/home/me/Documents", Include: []string{"*.md", "*.txt"}},
		{Name: "docs-writable", Type: TypeFiles, Root: "/home/me/Documents", ReadOnly: boolPtr(false)},
		{Name: "vault", Type: TypeNotes, Root: "/home/me/notes"},
		{Name: "vault-bytes", Type: TypeNotes, Root: "/home/me/notes", MaxBytes: 65536},
	}
	for i, spec := range cases {
		if err := spec.Validate(); err != nil {
			t.Errorf("case %d (%s): unexpected error: %v", i, spec.Name, err)
		}
	}
}

func TestSpecValidate_Errors(t *testing.T) {
	cases := []struct {
		name string
		spec Spec
		want string
	}{
		{"bad name uppercase", Spec{Name: "MyPG", Type: TypePostgres, Host: "h", Database: "d"}, "must match"},
		{"bad name start digit", Spec{Name: "1pg", Type: TypePostgres, Host: "h", Database: "d"}, "must match"},
		{"bad name too long", Spec{Name: strings.Repeat("a", 65), Type: TypePostgres, Host: "h", Database: "d"}, "must match"},
		{"unknown type", Spec{Name: "pg", Type: "oracle", Host: "h", Database: "d"}, "unsupported type"},
		{"missing host", Spec{Name: "pg", Type: TypePostgres, Database: "d"}, "host is required"},
		{"missing database", Spec{Name: "pg", Type: TypePostgres, Host: "h"}, "database is required"},
		{"sqlite missing path", Spec{Name: "db", Type: TypeSQLite}, "database (file path) is required"},
		{"sqlite file: uri rejected", Spec{Name: "db", Type: TypeSQLite, Database: "file:/data/app.db"}, "plain file path"},
		{"sqlite query params rejected", Spec{Name: "db", Type: TypeSQLite, Database: "/data/app.db?mode=rw"}, "URI parameters"},
		{"sqlite fragment rejected", Spec{Name: "db", Type: TypeSQLite, Database: "/data/app.db#f"}, "URI parameters"},
		{"db root rejected", Spec{Name: "pg", Type: TypePostgres, Host: "h", Database: "d", Root: "/data"}, "root is not valid"},
		{"db include rejected", Spec{Name: "pg", Type: TypePostgres, Host: "h", Database: "d", Include: []string{"*.md"}}, "include is not valid"},
		{"db max_bytes rejected", Spec{Name: "db", Type: TypeSQLite, Database: "/data/app.db", MaxBytes: 10}, "max_bytes is not valid"},
		{"port out of range", Spec{Name: "pg", Type: TypePostgres, Host: "h", Database: "d", Port: 70000}, "out of range"},
		{"secret ref missing group", Spec{Name: "pg", Type: TypePostgres, Host: "h", Database: "d",
			Credential: &CredentialRef{Kind: CredentialKindSecret, Key: "k"}}, "credential.group"},
		{"secret ref missing key", Spec{Name: "pg", Type: TypePostgres, Host: "h", Database: "d",
			Credential: &CredentialRef{Kind: CredentialKindSecret, Group: "g"}}, "credential.key"},
		{"env ref missing key", Spec{Name: "pg", Type: TypePostgres, Host: "h", Database: "d",
			Credential: &CredentialRef{Kind: CredentialKindEnv}}, "credential.key"},
		{"unknown credential kind", Spec{Name: "pg", Type: TypePostgres, Host: "h", Database: "d",
			Credential: &CredentialRef{Kind: "vault", Key: "k"}}, "unknown credential.kind"},
		{"null byte in host", Spec{Name: "pg", Type: TypePostgres, Host: "h\x00", Database: "d"}, "null byte"},
		{"negative max rows", Spec{Name: "pg", Type: TypePostgres, Host: "h", Database: "d", MaxRows: -1}, "max_rows"},
		{"negative timeout", Spec{Name: "pg", Type: TypePostgres, Host: "h", Database: "d", TimeoutSec: -1}, "timeout"},
		{"files missing root", Spec{Name: "docs", Type: TypeFiles}, "root is required"},
		{"notes missing root", Spec{Name: "vault", Type: TypeNotes}, "root is required"},
		{"files relative root", Spec{Name: "docs", Type: TypeFiles, Root: "relative/notes"}, "absolute path"},
		{"files host rejected", Spec{Name: "docs", Type: TypeFiles, Root: "/data", Host: "db"}, "not valid for type"},
		{"files database rejected", Spec{Name: "docs", Type: TypeFiles, Root: "/data", Database: "app"}, "not valid for type"},
		{"files credential rejected", Spec{Name: "docs", Type: TypeFiles, Root: "/data",
			Credential: &CredentialRef{Kind: CredentialKindEnv, Key: "K"}}, "credential is not supported"},
		{"files bad include pattern", Spec{Name: "docs", Type: TypeFiles, Root: "/data", Include: []string{"["}}, "bad include pattern"},
		{"files max_rows rejected", Spec{Name: "docs", Type: TypeFiles, Root: "/data", MaxRows: 10}, "max_rows is not valid"},
		{"files timeout rejected", Spec{Name: "docs", Type: TypeFiles, Root: "/data", TimeoutSec: 10}, "timeout is not valid"},
		{"null byte in root", Spec{Name: "docs", Type: TypeFiles, Root: "/data/\x00"}, "null byte"},
		{"negative max bytes", Spec{Name: "docs", Type: TypeFiles, Root: "/data", MaxBytes: -1}, "max_bytes"},
	}
	for _, c := range cases {
		err := c.spec.Validate()
		if err == nil {
			t.Errorf("%s: expected error", c.name)
			continue
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error %q does not contain %q", c.name, err.Error(), c.want)
		}
	}
}

func TestSpecDefaults(t *testing.T) {
	// read_only unset → read-only
	spec := Spec{Name: "pg", Type: TypePostgres, Host: "h", Database: "d"}
	if !spec.IsReadOnly() {
		t.Error("unset read_only should default to read-only")
	}
	// read_only=false honored
	spec.ReadOnly = boolPtr(false)
	if spec.IsReadOnly() {
		t.Error("read_only=false should allow writes")
	}
	// max_rows / timeout defaults
	if got := spec.EffectiveMaxRows(); got != DefaultMaxRows {
		t.Errorf("EffectiveMaxRows() = %d, want %d", got, DefaultMaxRows)
	}
	if got := spec.EffectiveTimeoutSec(); got != DefaultTimeoutSec {
		t.Errorf("EffectiveTimeoutSec() = %d, want %d", got, DefaultTimeoutSec)
	}
	// explicit values honored; non-positive fall back to defaults
	spec.MaxRows, spec.TimeoutSec = 500, 60
	if got := spec.EffectiveMaxRows(); got != 500 {
		t.Errorf("EffectiveMaxRows() = %d, want 500", got)
	}
	if got := spec.EffectiveTimeoutSec(); got != 60 {
		t.Errorf("EffectiveTimeoutSec() = %d, want 60", got)
	}
	spec.MaxRows, spec.TimeoutSec = -5, -5
	if got := spec.EffectiveMaxRows(); got != DefaultMaxRows {
		t.Errorf("EffectiveMaxRows() with negative = %d, want default %d", got, DefaultMaxRows)
	}
}

func TestValidate_NilSpec(t *testing.T) {
	var spec *Spec
	if err := spec.Validate(); err == nil {
		t.Error("nil spec should fail validation")
	}
}

func TestFileConnectorCeilings(t *testing.T) {
	// max_bytes default / explicit / invalid fallback
	files := Spec{Name: "docs", Type: TypeFiles, Root: "/data"}
	if got := files.EffectiveMaxBytes(); got != DefaultMaxFileBytes {
		t.Errorf("EffectiveMaxBytes() = %d, want default %d", got, DefaultMaxFileBytes)
	}
	files.MaxBytes = 4096
	if got := files.EffectiveMaxBytes(); got != 4096 {
		t.Errorf("EffectiveMaxBytes() = %d, want 4096", got)
	}
	files.MaxBytes = -1
	if got := files.EffectiveMaxBytes(); got != DefaultMaxFileBytes {
		t.Errorf("EffectiveMaxBytes() with negative = %d, want default %d", got, DefaultMaxFileBytes)
	}

	// IsFileConnector
	if !files.IsFileConnector() {
		t.Error("files connector should be a file connector")
	}
	pg := Spec{Name: "pg", Type: TypePostgres, Host: "h", Database: "d"}
	if pg.IsFileConnector() {
		t.Error("postgres connector should not be a file connector")
	}

	// files with no include → everything matches
	files = Spec{Name: "docs", Type: TypeFiles, Root: "/data"}
	if !files.MatchInclude("a.txt") || !files.MatchInclude("b.exe") {
		t.Error("empty include should match everything")
	}
	// files with include → allowlist only
	files.Include = []string{"*.md", "*.csv"}
	if !files.MatchInclude("note.md") || !files.MatchInclude("data.csv") {
		t.Error("allowlisted extensions should match")
	}
	if files.MatchInclude("evil.exe") {
		t.Error("non-allowlisted extension should not match")
	}
}

func TestNotesConnectorDefaultInclude(t *testing.T) {
	notes := Spec{Name: "vault", Type: TypeNotes, Root: "/home/me/notes"}
	// Notes default to markdown: reading a .txt through a notes
	// connector is rejected without an explicit include.
	if !notes.MatchInclude("daily.md") {
		t.Error("notes default include should match *.md")
	}
	if notes.MatchInclude("data.txt") {
		t.Error("notes default include should not match *.txt")
	}
	// Explicit include overrides the markdown default.
	notes.Include = []string{"*.txt"}
	if !notes.MatchInclude("data.txt") || notes.MatchInclude("daily.md") {
		t.Error("explicit include should replace the notes default")
	}
}
