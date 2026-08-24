// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​​​‌‌​‌‌​‌‌‌‌​​‌​‌​​‌‌​‌​‌‌‌​​​​‌​‌‌​​​​‌‌​​‌​​​‌‌​‌​​​‌​‌‌‌‌​​‌​‌​​​​​​​​​​​​​​​​​​​​‌​‌​‌​‌‌‌‌​​⁠
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

// Package connector implements the Connector API — aflare's secure data
// source connection layer. Users register named connectors (database
// endpoints, API systems) that reference credentials stored in the
// encrypted secrets store; workflows then refer to connectors by name and
// never contain inline credentials.
//
// The layer enforces the project's core security posture:
//
//   - Credentials live in the secrets store (or env), never in workflow
//     YAML or the connectors file itself — specs only hold references.
//   - Connectors are read-only by default; writes require explicit
//     opt-in on the connector spec, and node-level flags can only
//     tighten (never loosen) connector policy.
//   - Result size and timeout ceilings are part of the spec so a
//     workflow cannot exfiltrate a whole database in one query.
package connector

import (
	"fmt"
	"regexp"
	"strings"
)

// Connector type identifiers. A type maps to a provider that knows how to
// build the driver-specific DSN and which database/sql driver name to use.
const (
	TypePostgres = "postgres"
	TypeMySQL    = "mysql"
	TypeSQLite   = "sqlite"
)

// Default limits applied when the spec leaves them unset. Node parameters
// may only request lower values.
const (
	DefaultMaxRows    = 1000
	DefaultTimeoutSec = 30
)

// Credential kinds.
const (
	CredentialKindSecret = "secret" // secrets store group+key
	CredentialKindEnv    = "env"    // environment variable
)

// namePattern constrains connector names to lowercase DNS-style labels so
// they are safe to reference from workflows and CLI commands.
var namePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)

// CredentialRef points at where a connector's credential (password, API
// key) is stored. The spec file never contains the credential value.
type CredentialRef struct {
	// Kind selects the resolution strategy: "secret" (encrypted secrets
	// store) or "env" (environment variable).
	Kind string `yaml:"kind" json:"kind"`
	// Group is the secrets store group (required when Kind=secret).
	Group string `yaml:"group,omitempty" json:"group,omitempty"`
	// Key is the secrets store key (Kind=secret) or the environment
	// variable name (Kind=env).
	Key string `yaml:"key" json:"key"`
}

// Spec describes a named data source connection.
type Spec struct {
	Name     string `yaml:"name" json:"name"`
	Type     string `yaml:"type" json:"type"`
	Host     string `yaml:"host,omitempty" json:"host,omitempty"`
	Port     int    `yaml:"port,omitempty" json:"port,omitempty"`
	Database string `yaml:"database,omitempty" json:"database,omitempty"`
	Username string `yaml:"username,omitempty" json:"username,omitempty"`
	// Credential is optional: sqlite connectors and trust/cert-auth
	// servers do not need one.
	Credential *CredentialRef `yaml:"credential,omitempty" json:"credential,omitempty"`
	// ReadOnly defaults to true. Nil means "not set" → read-only.
	ReadOnly *bool `yaml:"read_only,omitempty" json:"read_only,omitempty"`
	// MaxRows caps query result rows (default 1000).
	MaxRows int `yaml:"max_rows,omitempty" json:"max_rows,omitempty"`
	// TimeoutSec caps query execution time (default 30s).
	TimeoutSec int `yaml:"timeout,omitempty" json:"timeout,omitempty"`
}

// IsReadOnly reports whether the connector allows writes. Unset → true.
func (s *Spec) IsReadOnly() bool {
	if s.ReadOnly == nil {
		return true
	}
	return *s.ReadOnly
}

// EffectiveMaxRows returns the spec's row ceiling, applying the default
// when unset and rejecting nonsensical values.
func (s *Spec) EffectiveMaxRows() int {
	if s.MaxRows <= 0 {
		return DefaultMaxRows
	}
	return s.MaxRows
}

// EffectiveTimeoutSec returns the spec's timeout ceiling in seconds,
// applying the default when unset.
func (s *Spec) EffectiveTimeoutSec() int {
	if s.TimeoutSec <= 0 {
		return DefaultTimeoutSec
	}
	return s.TimeoutSec
}

// supportedTypes lists the connector types the current provider set can
// build DSNs for.
func supportedTypes() map[string]bool {
	return map[string]bool{
		TypePostgres: true,
		TypeMySQL:    true,
		TypeSQLite:   true,
	}
}

// Validate checks the spec for structural errors. It returns an error for
// anything that would make DSN building ambiguous or unsafe: bad names,
// unknown types, missing endpoint fields, incomplete credential refs, or
// embedded null bytes.
func (s *Spec) Validate() error {
	if s == nil {
		return fmt.Errorf("connector spec is nil")
	}
	if !namePattern.MatchString(s.Name) {
		return fmt.Errorf("connector name %q must match %s", s.Name, namePattern.String())
	}
	if !supportedTypes()[s.Type] {
		return fmt.Errorf("connector %q: unsupported type %q (supported: postgres, mysql, sqlite)", s.Name, s.Type)
	}
	if err := validateNoNullBytes(s); err != nil {
		return err
	}
	if s.Port < 0 || s.Port > 65535 {
		return fmt.Errorf("connector %q: port %d out of range", s.Name, s.Port)
	}

	switch s.Type {
	case TypePostgres, TypeMySQL:
		if s.Host == "" {
			return fmt.Errorf("connector %q: host is required for type %s", s.Name, s.Type)
		}
		if s.Database == "" {
			return fmt.Errorf("connector %q: database is required for type %s", s.Name, s.Type)
		}
	case TypeSQLite:
		if s.Database == "" {
			return fmt.Errorf("connector %q: database (file path) is required for type sqlite", s.Name)
		}
	}

	if s.Credential != nil {
		switch s.Credential.Kind {
		case CredentialKindSecret:
			if s.Credential.Group == "" {
				return fmt.Errorf("connector %q: credential.group is required when credential.kind=secret", s.Name)
			}
			if s.Credential.Key == "" {
				return fmt.Errorf("connector %q: credential.key is required when credential.kind=secret", s.Name)
			}
		case CredentialKindEnv:
			if s.Credential.Key == "" {
				return fmt.Errorf("connector %q: credential.key (env var name) is required when credential.kind=env", s.Name)
			}
		default:
			return fmt.Errorf("connector %q: unknown credential.kind %q (use secret or env)", s.Name, s.Credential.Kind)
		}
	}

	if s.MaxRows < 0 {
		return fmt.Errorf("connector %q: max_rows must be >= 0", s.Name)
	}
	if s.TimeoutSec < 0 {
		return fmt.Errorf("connector %q: timeout must be >= 0", s.Name)
	}
	return nil
}

// validateNoNullBytes rejects NUL in every string field: null bytes can
// truncate downstream DSN parsing and filesystem operations.
func validateNoNullBytes(s *Spec) error {
	credentialKey := ""
	if s.Credential != nil {
		credentialKey = s.Credential.Key
	}
	fields := map[string]string{
		"name":           s.Name,
		"type":           s.Type,
		"host":           s.Host,
		"database":       s.Database,
		"username":       s.Username,
		"credential.key": credentialKey,
	}
	for field, value := range fields {
		if strings.ContainsRune(value, '\x00') {
			return fmt.Errorf("connector %q: %s contains null byte", s.Name, field)
		}
	}
	return nil
}
