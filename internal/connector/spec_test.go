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
