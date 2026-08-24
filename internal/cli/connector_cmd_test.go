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
