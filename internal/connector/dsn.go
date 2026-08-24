// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​​​‌‌​‌‌​‌‌‌‌​​‌​‌​​​‌‌​‌​​​‌​‌‌‌​‌‌‌‌​​‌​​‌​​​‌​​​‌​​‌‌‌​‌​‌‌​​‌‌​​​​​​​​​​​​​​​​​​​‌​​​‌‌‌​‌‌‌‌​⁠
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
	"fmt"
	"net"
	"net/url"
	"strconv"
)

// Default ports applied when the spec leaves Port unset.
const (
	defaultPostgresPort = 5432
	defaultMySQLPort    = 3306
)

// DriverName maps a connector type to its database/sql driver name. The
// driver itself must be registered by the host program (sql_query adds no
// third-party driver dependency).
func DriverName(connType string) (string, error) {
	switch connType {
	case TypePostgres:
		return "postgres", nil
	case TypeMySQL:
		return "mysql", nil
	case TypeSQLite:
		return "sqlite3", nil
	default:
		return "", fmt.Errorf("unsupported connector type %q", connType)
	}
}

// BuildDSN renders the driver name and driver-specific DSN for a validated
// spec, injecting the already-resolved credential. For postgres the
// username and password are URL-escaped (url.UserPassword), so special
// characters in the password cannot alter the DSN structure. For sqlite
// the credential is ignored.
func BuildDSN(spec Spec, password string) (driver, dsn string, err error) {
	if err := spec.Validate(); err != nil {
		return "", "", fmt.Errorf("invalid connector spec: %w", err)
	}
	switch spec.Type {
	case TypePostgres:
		return "postgres", buildPostgresDSN(spec, password), nil
	case TypeMySQL:
		return "mysql", buildMySQLDSN(spec, password), nil
	case TypeSQLite:
		return "sqlite3", buildSQLiteDSN(spec), nil
	default:
		return "", "", fmt.Errorf("unsupported connector type %q", spec.Type)
	}
}

// buildSQLiteDSN renders the sqlite DSN. Read-only connectors (the
// default) get a file: URI with mode=ro so the driver itself rejects
// writes — even if the node-level read_only gate were bypassed, the
// database file cannot be modified through this connection. Spec
// validation guarantees Database is a plain path (no file: prefix, no
// ? / # params), so the URI is built deterministically here and an
// injected mode= parameter cannot override the read-only enforcement.
func buildSQLiteDSN(spec Spec) string {
	if !spec.IsReadOnly() {
		return spec.Database
	}
	return "file:" + spec.Database + "?mode=ro"
}

// buildPostgresDSN renders a postgres:// URL DSN. url.UserPassword
// percent-encodes reserved characters in user info.
func buildPostgresDSN(spec Spec, password string) string {
	port := spec.Port
	if port == 0 {
		port = defaultPostgresPort
	}
	u := url.URL{
		Scheme: "postgres",
		Host:   net.JoinHostPort(spec.Host, strconv.Itoa(port)),
		Path:   "/" + spec.Database,
	}
	switch {
	case spec.Username != "" && password != "":
		u.User = url.UserPassword(spec.Username, password)
	case spec.Username != "":
		u.User = url.User(spec.Username)
	}
	return u.String()
}

// buildMySQLDSN renders a go-sql-driver DSN (user:pass@tcp(host:port)/db).
// Values come from the admin-managed spec and the secrets store, not from
// AI-generated workflow input.
func buildMySQLDSN(spec Spec, password string) string {
	port := spec.Port
	if port == 0 {
		port = defaultMySQLPort
	}
	auth := ""
	switch {
	case spec.Username != "" && password != "":
		auth = spec.Username + ":" + password + "@"
	case spec.Username != "":
		auth = spec.Username + "@"
	}
	return auth + "tcp(" + net.JoinHostPort(spec.Host, strconv.Itoa(port)) + ")/" + spec.Database
}
