// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​​​‌‌​‌‌​‌‌‌‌​​‌​‌‌‌‌​​​​​‌‌‌‌​​​‌​​‌​​​‌​​​‌​​‌​​‌‌​‌​​​​​‌​‌‌‌​‌​​​​​​​​​​​​​​​​‌‌​​​​‌‌‌‌‌​​​‌‌⁠
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

package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alib8b8/aflare/internal/connector"
)

func connectorTestFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "connectors.yaml")
	t.Setenv("AFLARE_CONNECTORS_FILE", path)
	return path
}

func TestHandleConnector_AddListShowRemove(t *testing.T) {
	connectorTestFile(t)

	// add
	err := HandleConnector([]string{
		"add", "my-pg", "--type", "postgres",
		"--host", "db.internal", "--port", "5432",
		"--database", "app", "--username", "ro",
		"--credential-group", "connectors",
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}

	// The registry on disk must hold the spec with the secret credential
	// reference (credential key defaults to the connector name).
	reg, err := connector.LoadDefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	spec, ok := reg.Get("my-pg")
	if !ok {
		t.Fatal("my-pg not registered")
	}
	if spec.Host != "db.internal" || spec.Port != 5432 || spec.Database != "app" || spec.Username != "ro" {
		t.Errorf("spec mismatch: %+v", spec)
	}
	if !spec.IsReadOnly() {
		t.Error("connector must be read-only by default")
	}
	if spec.Credential == nil || spec.Credential.Kind != connector.CredentialKindSecret ||
		spec.Credential.Group != "connectors" || spec.Credential.Key != "my-pg" {
		t.Errorf("credential ref mismatch: %+v", spec.Credential)
	}

	// list / show
	if err := HandleConnector([]string{"list"}); err != nil {
		t.Errorf("list: %v", err)
	}
	if err := HandleConnector([]string{"show", "my-pg"}); err != nil {
		t.Errorf("show: %v", err)
	}

	// remove
	if err := HandleConnector([]string{"remove", "my-pg"}); err != nil {
		t.Errorf("remove: %v", err)
	}
	reg, err = connector.LoadDefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := reg.Get("my-pg"); ok {
		t.Error("my-pg should be removed")
	}
}

func TestHandleConnector_AddWritable(t *testing.T) {
	connectorTestFile(t)
	err := HandleConnector([]string{
		"add", "rw", "--type", "sqlite", "--database", "/tmp/x.db",
		"--writable", "--max-rows", "500", "--timeout", "10",
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	reg, err := connector.LoadDefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	spec, ok := reg.Get("rw")
	if !ok {
		t.Fatal("rw not registered")
	}
	if spec.IsReadOnly() {
		t.Error("--writable should produce a writable connector")
	}
	if spec.EffectiveMaxRows() != 500 || spec.EffectiveTimeoutSec() != 10 {
		t.Errorf("limits mismatch: %+v", spec)
	}
}

func TestHandleConnector_AddEnvCredential(t *testing.T) {
	connectorTestFile(t)
	err := HandleConnector([]string{
		"add", "pg-env", "--type", "postgres", "--host", "db", "--database", "app",
		"--credential-env", "PG_PASS",
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	reg, err := connector.LoadDefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	spec, ok := reg.Get("pg-env")
	if !ok {
		t.Fatal("pg-env not registered")
	}
	if spec.Credential == nil || spec.Credential.Kind != connector.CredentialKindEnv || spec.Credential.Key != "PG_PASS" {
		t.Errorf("credential ref mismatch: %+v", spec.Credential)
	}
}

func TestHandleConnector_AddErrors(t *testing.T) {
	connectorTestFile(t)

	cases := [][]string{
		// missing name
		{"add", "--type", "postgres"},
		// unknown type
		{"add", "x", "--type", "oracle", "--host", "h", "--database", "d"},
		// postgres without host
		{"add", "x", "--type", "postgres", "--database", "d"},
		// conflicting credential sources
		{"add", "x", "--type", "sqlite", "--database", "/tmp/x.db",
			"--credential-group", "g", "--credential-env", "E"},
		// invalid name
		{"add", "BadName", "--type", "sqlite", "--database", "/tmp/x.db"},
	}
	for i, args := range cases {
		if err := HandleConnector(args); err == nil {
			t.Errorf("case %d: expected error for %v", i, args)
		}
	}
}

func TestHandleConnector_DuplicateAdd(t *testing.T) {
	connectorTestFile(t)
	addArgs := []string{"add", "dup", "--type", "sqlite", "--database", "/tmp/dup.db"}
	if err := HandleConnector(addArgs); err != nil {
		t.Fatalf("first add: %v", err)
	}
	if err := HandleConnector(addArgs); err == nil {
		t.Error("duplicate add should fail")
	}
}

func TestHandleConnector_ShowRemoveMissing(t *testing.T) {
	connectorTestFile(t)
	if err := HandleConnector([]string{"show", "ghost"}); err == nil {
		t.Error("show of missing connector should fail")
	}
	if err := HandleConnector([]string{"remove", "ghost"}); err == nil {
		t.Error("remove of missing connector should fail")
	}
}

func TestHandleConnector_UnknownSubcommand(t *testing.T) {
	connectorTestFile(t)
	if err := HandleConnector([]string{"bogus"}); err == nil {
		t.Error("unknown subcommand should fail")
	}
	if err := HandleConnector(nil); err == nil {
		t.Error("no args should fail")
	}
	// help must not error
	if err := HandleConnector([]string{"--help"}); err != nil {
		t.Errorf("help: %v", err)
	}
}

func TestHandleConnector_ListEmpty(t *testing.T) {
	connectorTestFile(t)
	if err := HandleConnector([]string{"list"}); err != nil {
		t.Errorf("list on empty registry: %v", err)
	}
}

func TestHandleConnector_AddFilesConnector(t *testing.T) {
	connectorTestFile(t)
	root := t.TempDir()

	err := HandleConnector([]string{
		"add", "my-notes", "--type", "notes", "--root", root,
		"--max-bytes", "65536",
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	reg, err := connector.LoadDefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	spec, ok := reg.Get("my-notes")
	if !ok {
		t.Fatal("my-notes not registered")
	}
	if spec.Root != root {
		t.Errorf("root = %q, want %q", spec.Root, root)
	}
	if !spec.IsReadOnly() {
		t.Error("notes connector must be read-only by default")
	}
	if spec.EffectiveMaxBytes() != 65536 {
		t.Errorf("max_bytes = %d, want 65536", spec.EffectiveMaxBytes())
	}
	// No explicit include → notes default is markdown.
	if !spec.MatchInclude("a.md") || spec.MatchInclude("a.txt") {
		t.Errorf("notes default include should be markdown-only: %v", spec.EffectiveInclude())
	}

	// list / show must not error on file connectors
	if err := HandleConnector([]string{"list"}); err != nil {
		t.Errorf("list: %v", err)
	}
	if err := HandleConnector([]string{"show", "my-notes"}); err != nil {
		t.Errorf("show: %v", err)
	}
}

func TestHandleConnector_AddFilesConnectorResolvesSymlinkRoot(t *testing.T) {
	connectorTestFile(t)
	real := t.TempDir()
	link := filepath.Join(t.TempDir(), "vault")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlink creation failed: %v", err)
	}

	// A symlink root must be resolved to the real path at registration:
	// the stored spec is the actual containment boundary.
	if err := HandleConnector([]string{"add", "my-notes", "--type", "notes", "--root", link}); err != nil {
		t.Fatalf("add: %v", err)
	}
	reg, err := connector.LoadDefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	spec, ok := reg.Get("my-notes")
	if !ok {
		t.Fatal("my-notes not registered")
	}
	if spec.Root != real {
		t.Errorf("root = %q, want resolved %q", spec.Root, real)
	}
}

func TestHandleConnector_AddSQLiteRejectsURIDatabase(t *testing.T) {
	connectorTestFile(t)
	// A database value carrying URI parameters could inject its own
	// mode= parameter and defeat the driver-level read-only DSN —
	// registration must reject it.
	for _, db := range []string{"file:/tmp/x.db", "/tmp/x.db?mode=rw"} {
		err := HandleConnector([]string{"add", "db", "--type", "sqlite", "--database", db})
		if err == nil {
			t.Errorf("database %q should be rejected", db)
		}
	}
}

func TestHandleConnector_AddFilesConnectorWithIncludeAndWritable(t *testing.T) {
	connectorTestFile(t)
	root := t.TempDir()

	err := HandleConnector([]string{
		"add", "my-docs", "--type", "files", "--root", root,
		"--writable", "--include", "*.md", "--include", "*.txt",
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	reg, err := connector.LoadDefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	spec, ok := reg.Get("my-docs")
	if !ok {
		t.Fatal("my-docs not registered")
	}
	if spec.IsReadOnly() {
		t.Error("--writable should produce a writable files connector")
	}
	if len(spec.Include) != 2 || spec.Include[0] != "*.md" || spec.Include[1] != "*.txt" {
		t.Errorf("include mismatch: %v", spec.Include)
	}
	if spec.MatchInclude("a.exe") {
		t.Error("include allowlist should reject non-listed extensions")
	}
}

func TestHandleConnector_AddFilesConnectorErrors(t *testing.T) {
	connectorTestFile(t)
	root := t.TempDir()
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	filePath := filepath.Join(root, "file.md")
	if err := os.WriteFile(filePath, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	cases := [][]string{
		// files without root
		{"add", "x", "--type", "files"},
		// root that does not exist
		{"add", "x", "--type", "files", "--root", missing},
		// root that is not a directory
		{"add", "x", "--type", "files", "--root", filePath},
		// relative root
		{"add", "x", "--type", "files", "--root", "relative/dir"},
		// credential on a file connector
		{"add", "x", "--type", "files", "--root", root, "--credential-env", "E"},
		// database fields on a file connector
		{"add", "x", "--type", "files", "--root", root, "--database", "app"},
		// bad include pattern
		{"add", "x", "--type", "files", "--root", root, "--include", "["},
	}
	for i, args := range cases {
		if err := HandleConnector(args); err == nil {
			t.Errorf("case %d: expected error for %v", i, args)
		}
	}
}

func TestHandleConnector_HomeExpansion(t *testing.T) {
	connectorTestFile(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	notesDir := filepath.Join(home, "notes")
	if err := os.MkdirAll(notesDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// --root ~/notes must expand to $HOME/notes before validation.
	if err := HandleConnector([]string{"add", "vault", "--type", "notes", "--root", "~/notes"}); err != nil {
		t.Fatalf("add with ~ root: %v", err)
	}
	reg, err := connector.LoadDefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	spec, ok := reg.Get("vault")
	if !ok {
		t.Fatal("vault not registered")
	}
	if spec.Root != notesDir {
		t.Errorf("root = %q, want %q (unexpanded ~)", spec.Root, notesDir)
	}
}
