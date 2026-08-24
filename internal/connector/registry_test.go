// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​​​‌‌​‌‌​‌‌‌‌​​‌​‌​​​‌‌​‌‌​​​‌‌‌​‌​​​​​​​​​​‌​​​‌​​‌​​​‌‌‌​‌​‌​‌​​​​​​​​​​​​​​​​​​​​​‌​​​‌‌‌‌​‌​‌​⁠
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
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func pgSpec(name string) Spec {
	return Spec{Name: name, Type: TypePostgres, Host: "db.example.com", Port: 5432, Database: "app", Username: "ro"}
}

func TestLoadRegistry_MissingFile(t *testing.T) {
	r, err := LoadRegistry(filepath.Join(t.TempDir(), "connectors.yaml"))
	if err != nil {
		t.Fatalf("missing file should yield empty registry, got %v", err)
	}
	if got := r.List(); len(got) != 0 {
		t.Errorf("expected empty registry, got %d specs", len(got))
	}
}

func TestRegistry_UpsertGetListRemove(t *testing.T) {
	r, err := LoadRegistry(filepath.Join(t.TempDir(), "connectors.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Upsert(pgSpec("pg-a")); err != nil {
		t.Fatal(err)
	}
	if err := r.Upsert(pgSpec("pg-b")); err != nil {
		t.Fatal(err)
	}

	if got := r.List(); len(got) != 2 || got[0].Name != "pg-a" || got[1].Name != "pg-b" {
		t.Errorf("List() should be sorted by name, got %v", got)
	}

	spec, ok := r.Get("pg-a")
	if !ok || spec.Host != "db.example.com" {
		t.Errorf("Get(pg-a) = %+v ok=%v", spec, ok)
	}
	if _, ok := r.Get("missing"); ok {
		t.Error("Get(missing) should not be found")
	}

	// Upsert replaces same-name spec
	replacement := pgSpec("pg-a")
	replacement.Host = "new.example.com"
	if err := r.Upsert(replacement); err != nil {
		t.Fatal(err)
	}
	spec, _ = r.Get("pg-a")
	if spec.Host != "new.example.com" {
		t.Errorf("Upsert should replace, host = %q", spec.Host)
	}
	if got := r.List(); len(got) != 2 {
		t.Errorf("replace should not add entry, got %d specs", len(got))
	}

	// Upsert validates
	if err := r.Upsert(Spec{Name: "bad name", Type: TypePostgres, Host: "h", Database: "d"}); err == nil {
		t.Error("Upsert should reject invalid spec")
	}

	if err := r.Remove("pg-b"); err != nil {
		t.Fatal(err)
	}
	if _, ok := r.Get("pg-b"); ok {
		t.Error("pg-b should be removed")
	}
	if err := r.Remove("pg-b"); err == nil {
		t.Error("removing missing spec should error")
	}
}

func TestRegistry_SaveReloadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "connectors.yaml")

	r1, err := LoadRegistry(path)
	if err != nil {
		t.Fatal(err)
	}
	spec := pgSpec("pg-a")
	writable := false
	spec.ReadOnly = &writable
	spec.MaxRows = 500
	spec.TimeoutSec = 10
	spec.Credential = &CredentialRef{Kind: CredentialKindSecret, Group: "connectors", Key: "pg-a"}
	if err := r1.Upsert(spec); err != nil {
		t.Fatal(err)
	}
	if err := r1.Save(); err != nil {
		t.Fatal(err)
	}

	r2, err := LoadRegistry(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	got, ok := r2.Get("pg-a")
	if !ok {
		t.Fatal("pg-a lost after save/reload")
	}
	if got.Host != "db.example.com" || got.Port != 5432 || got.Username != "ro" {
		t.Errorf("round-trip mismatch: %+v", got)
	}
	if got.IsReadOnly() {
		t.Error("read_only=false lost in round-trip")
	}
	if got.EffectiveMaxRows() != 500 || got.EffectiveTimeoutSec() != 10 {
		t.Errorf("limits lost in round-trip: %+v", got)
	}
	if got.Credential == nil || got.Credential.Kind != CredentialKindSecret ||
		got.Credential.Group != "connectors" || got.Credential.Key != "pg-a" {
		t.Errorf("credential ref lost in round-trip: %+v", got.Credential)
	}

	// file permissions: 0600
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0600 {
		t.Errorf("connectors file mode = %v, want 0600", fi.Mode().Perm())
	}

	// saved file must not contain any password-like secret value; it only
	// holds references. Sanity: contains the ref key, not a value column.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "connectors") || !strings.Contains(string(data), "pg-a") {
		t.Errorf("saved file missing expected content:\n%s", data)
	}
}

func TestLoadRegistry_MalformedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "connectors.yaml")
	if err := os.WriteFile(path, []byte("connectors: [not: valid: yaml"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadRegistry(path); err == nil {
		t.Error("malformed file should error")
	}
}

func TestLoadRegistry_InvalidSpecInFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "connectors.yaml")
	// Valid YAML but an invalid spec (missing host) → load must fail loudly
	// rather than silently dropping the connector.
	content := "version: 1\nconnectors:\n  - name: pg\n    type: postgres\n    database: app\n"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadRegistry(path)
	if err == nil || !strings.Contains(err.Error(), "host is required") {
		t.Errorf("expected host-required error, got %v", err)
	}
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	r, err := LoadRegistry(filepath.Join(t.TempDir(), "connectors.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = r.Upsert(pgSpec("pg-concurrent"))
			_ = r.List()
			_, _ = r.Get("pg-concurrent")
		}()
	}
	wg.Wait()
	if got := r.List(); len(got) != 1 {
		t.Errorf("concurrent upserts should converge to 1 spec, got %d", len(got))
	}
}

func TestDefaultRegistryPath_EnvOverride(t *testing.T) {
	t.Setenv("AFLARE_CONNECTORS_FILE", "/tmp/custom/connectors.yaml")
	if got := DefaultRegistryPath(); got != "/tmp/custom/connectors.yaml" {
		t.Errorf("DefaultRegistryPath() = %q, want env override", got)
	}
}
