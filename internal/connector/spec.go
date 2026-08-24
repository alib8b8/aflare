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
	"path/filepath"
	"regexp"
	"strings"
)

// Connector type identifiers. Database types map to a provider that knows
// how to build the driver-specific DSN; file types (files/notes) route
// through the file nodes (file_read/file_write/files_list) with the root
// directory acting as the containment boundary.
const (
	TypePostgres = "postgres"
	TypeMySQL    = "mysql"
	TypeSQLite   = "sqlite"
	TypeFiles    = "files"
	TypeNotes    = "notes"
)

// typeNames lists supported types for error messages, in help order.
var typeNames = []string{TypePostgres, TypeMySQL, TypeSQLite, TypeFiles, TypeNotes}

// Default limits applied when the spec leaves them unset. Node parameters
// may only request lower values.
const (
	DefaultMaxRows    = 1000
	DefaultTimeoutSec = 30
	// DefaultMaxFileBytes matches the file_read node's hard read cap so a
	// files/notes connector never loosens it.
	DefaultMaxFileBytes = 10 * 1024 * 1024
)

// notesDefaultInclude is the extension allowlist applied to notes
// connectors that declare none: a notes vault is markdown.
var notesDefaultInclude = []string{"*.md", "*.markdown"}

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
	// Root is the absolute directory root for files/notes connectors.
	// Node paths resolve relative to it and can never escape it — the
	// same containment rules (no absolute paths, no traversal, L2+
	// symlink checks) that confine nodes to the working directory.
	Root string `yaml:"root,omitempty" json:"root,omitempty"`
	// Credential is optional: sqlite connectors and trust/cert-auth
	// servers do not need one.
	Credential *CredentialRef `yaml:"credential,omitempty" json:"credential,omitempty"`
	// ReadOnly defaults to true. Nil means "not set" → read-only.
	ReadOnly *bool `yaml:"read_only,omitempty" json:"read_only,omitempty"`
	// MaxRows caps query result rows (default 1000).
	MaxRows int `yaml:"max_rows,omitempty" json:"max_rows,omitempty"`
	// TimeoutSec caps query execution time (default 30s).
	TimeoutSec int `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	// MaxBytes caps a single file read through files/notes connectors
	// (default 10MB, matching the file_read node's hard cap).
	MaxBytes int `yaml:"max_bytes,omitempty" json:"max_bytes,omitempty"`
	// Include is an optional glob allowlist matched against the file's
	// base name. Notes connectors default to *.md / *.markdown.
	Include []string `yaml:"include,omitempty" json:"include,omitempty"`
}

// IsFileConnector reports whether the type routes through the file nodes
// (file_read/file_write/files_list) rather than sql_query.
func (s *Spec) IsFileConnector() bool {
	return s.Type == TypeFiles || s.Type == TypeNotes
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

// EffectiveMaxBytes returns the per-file read ceiling for file
// connectors, applying the default when unset.
func (s *Spec) EffectiveMaxBytes() int {
	if s.MaxBytes <= 0 {
		return DefaultMaxFileBytes
	}
	return s.MaxBytes
}

// EffectiveInclude returns the glob allowlist for file connectors.
// Notes connectors default to markdown files; files connectors with no
// include list accept every file (still subject to the node-level
// security checks).
func (s *Spec) EffectiveInclude() []string {
	if s.Type == TypeNotes && len(s.Include) == 0 {
		return notesDefaultInclude
	}
	return s.Include
}

// MatchInclude reports whether name (a file base name) passes the
// connector's include allowlist. An empty allowlist matches everything.
func (s *Spec) MatchInclude(name string) bool {
	include := s.EffectiveInclude()
	if len(include) == 0 {
		return true
	}
	for _, pattern := range include {
		if ok, err := filepath.Match(pattern, name); err == nil && ok {
			return true
		}
	}
	return false
}

// supportedTypes lists the connector types the current provider set can
// build DSNs for or route to the file nodes.
func supportedTypes() map[string]bool {
	return map[string]bool{
		TypePostgres: true,
		TypeMySQL:    true,
		TypeSQLite:   true,
		TypeFiles:    true,
		TypeNotes:    true,
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
		return fmt.Errorf("connector %q: unsupported type %q (supported: %s)", s.Name, s.Type, strings.Join(typeNames, ", "))
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
		// The database must be a plain file path: aflare builds the
		// read-only DSN itself ("file:<path>?mode=ro"). A value that
		// already looks like a URI (file: prefix) or carries a query
		// string could inject its own mode= parameter and silently
		// defeat the driver-level read-only enforcement.
		if strings.HasPrefix(s.Database, "file:") {
			return fmt.Errorf("connector %q: database must be a plain file path, not a file: URI (the read-only URI is built by aflare)", s.Name)
		}
		if strings.ContainsAny(s.Database, "?#") {
			return fmt.Errorf("connector %q: database must not contain URI parameters (got %q)", s.Name, s.Database)
		}
		// Fall through to the database-type field checks below (file-only
		// fields like root/include/max_bytes are rejected there).
	}

	// File-connector-only fields on a database connector would be
	// silently ignored — reject them so a stored spec never lies about
	// what it enforces (mirrors the files-branch checks above).
	if !s.IsFileConnector() {
		if s.Root != "" {
			return fmt.Errorf("connector %q: root is not valid for type %s (files/notes only)", s.Name, s.Type)
		}
		if len(s.Include) > 0 {
			return fmt.Errorf("connector %q: include is not valid for type %s (files/notes only)", s.Name, s.Type)
		}
		if s.MaxBytes != 0 {
			return fmt.Errorf("connector %q: max_bytes is not valid for type %s (use max_rows)", s.Name, s.Type)
		}
	}

	switch s.Type {
	case TypeFiles, TypeNotes:
		if s.Root == "" {
			return fmt.Errorf("connector %q: root is required for type %s", s.Name, s.Type)
		}
		if !filepath.IsAbs(s.Root) {
			return fmt.Errorf("connector %q: root must be an absolute path (got %q)", s.Name, s.Root)
		}
		// File connectors live on the local filesystem: database
		// endpoint and credential fields would be silently ignored, so
		// reject them instead of storing misleading specs.
		if s.Host != "" || s.Port != 0 || s.Database != "" || s.Username != "" {
			return fmt.Errorf("connector %q: host/port/database/username are not valid for type %s (use root)", s.Name, s.Type)
		}
		if s.Credential != nil {
			return fmt.Errorf("connector %q: credential is not supported for type %s (local files need none)", s.Name, s.Type)
		}
		for _, pattern := range s.Include {
			if _, err := filepath.Match(pattern, "x"); err != nil {
				return fmt.Errorf("connector %q: bad include pattern %q: %w", s.Name, pattern, err)
			}
		}
		if s.MaxRows != 0 {
			return fmt.Errorf("connector %q: max_rows is not valid for type %s (use max_bytes)", s.Name, s.Type)
		}
		if s.TimeoutSec != 0 {
			return fmt.Errorf("connector %q: timeout is not valid for type %s", s.Name, s.Type)
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
	if s.MaxBytes < 0 {
		return fmt.Errorf("connector %q: max_bytes must be >= 0", s.Name)
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
		"root":           s.Root,
		"credential.key": credentialKey,
	}
	for field, value := range fields {
		if strings.ContainsRune(value, '\x00') {
			return fmt.Errorf("connector %q: %s contains null byte", s.Name, field)
		}
	}
	return nil
}
