// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​​​‌‌​‌‌​‌‌‌‌​​‌​‌​‌‌​‌‌‌​​‌​‌​​​‌​​​‌​‌​​​‌‌​‌​‌​​‌​​‌‌‌‌‌‌‌‌‌​‌​​​​​​​​​​​​​​​​​​‌‌‌‌‌​‌‌‌‌​‌‌​​⁠
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
	"net/url"
	"testing"
)

func TestDriverName(t *testing.T) {
	cases := map[string]string{
		TypePostgres: "postgres",
		TypeMySQL:    "mysql",
		TypeSQLite:   "sqlite3",
	}
	for connType, want := range cases {
		got, err := DriverName(connType)
		if err != nil {
			t.Errorf("DriverName(%s): %v", connType, err)
			continue
		}
		if got != want {
			t.Errorf("DriverName(%s) = %q, want %q", connType, got, want)
		}
	}
	if _, err := DriverName("oracle"); err == nil {
		t.Error("DriverName(oracle) should error")
	}
}

func TestBuildDSN_Postgres(t *testing.T) {
	spec := Spec{Name: "pg", Type: TypePostgres, Host: "db.example.com", Port: 5432, Database: "app", Username: "ro"}
	driver, dsn, err := BuildDSN(spec, "pass")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if driver != "postgres" {
		t.Errorf("driver = %q, want postgres", driver)
	}
	want := "postgres://ro:pass@db.example.com:5432/app"
	if dsn != want {
		t.Errorf("dsn = %q, want %q", dsn, want)
	}
}

func TestBuildDSN_PostgresDefaultPort(t *testing.T) {
	spec := Spec{Name: "pg", Type: TypePostgres, Host: "db", Database: "app"}
	_, dsn, err := BuildDSN(spec, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "postgres://db:5432/app"; dsn != want {
		t.Errorf("dsn = %q, want %q", dsn, want)
	}
}

func TestBuildDSN_PostgresEscapesPassword(t *testing.T) {
	// A password containing URL-reserved characters must not break the DSN
	// structure: after parsing, the decoded userinfo must round-trip.
	spec := Spec{Name: "pg", Type: TypePostgres, Host: "db", Database: "app", Username: "u"}
	_, dsn, err := BuildDSN(spec, "p@ss:word/?")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("generated DSN %q does not parse: %v", dsn, err)
	}
	if got := u.User.Username(); got != "u" {
		t.Errorf("username = %q, want u", got)
	}
	pass, ok := u.User.Password()
	if !ok || pass != "p@ss:word/?" {
		t.Errorf("password round-trip failed: %q ok=%v", pass, ok)
	}
}

func TestBuildDSN_MySQL(t *testing.T) {
	spec := Spec{Name: "my", Type: TypeMySQL, Host: "mysql.internal", Port: 3307, Database: "app", Username: "ro"}
	driver, dsn, err := BuildDSN(spec, "pass")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if driver != "mysql" {
		t.Errorf("driver = %q, want mysql", driver)
	}
	if want := "ro:pass@tcp(mysql.internal:3307)/app"; dsn != want {
		t.Errorf("dsn = %q, want %q", dsn, want)
	}
}

func TestBuildDSN_MySQLDefaultPortNoUser(t *testing.T) {
	spec := Spec{Name: "my", Type: TypeMySQL, Host: "mysql.internal", Database: "app"}
	_, dsn, err := BuildDSN(spec, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "tcp(mysql.internal:3306)/app"; dsn != want {
		t.Errorf("dsn = %q, want %q", dsn, want)
	}
}

func TestBuildDSN_SQLite(t *testing.T) {
	spec := Spec{Name: "db", Type: TypeSQLite, Database: "/data/app.db"}
	driver, dsn, err := BuildDSN(spec, "ignored")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if driver != "sqlite3" {
		t.Errorf("driver = %q, want sqlite3", driver)
	}
	if dsn != "/data/app.db" {
		t.Errorf("dsn = %q, want /data/app.db", dsn)
	}
}

func TestBuildDSN_InvalidSpec(t *testing.T) {
	// invalid spec must be rejected before any DSN is produced
	spec := Spec{Name: "pg", Type: TypePostgres, Host: "", Database: "app"}
	if _, _, err := BuildDSN(spec, "x"); err == nil {
		t.Error("expected error for invalid spec")
	}
}
