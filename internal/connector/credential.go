// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​​​‌‌​‌‌​‌‌‌‌​​‌​‌​​‌​‌‌​‌‌​​​‌‌‌​‌‌​‌​‌​​‌​‌​​​​‌​‌​​​‌​​​​‌‌‌‌​​​​​​​​​​​​​​​​​​‌‌​‌​​​​‌​​​‌‌‌​⁠
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
	"os"

	"github.com/alib8b8/aflare/internal/secrets"
)

// CredentialResolver resolves a CredentialRef into the credential value.
// It is the deployment-profile seam of the Connector API:
//
//   - Personal profile (default): secrets stored in the encrypted local
//     store (~/.config/aflare/secrets.dat) or environment variables.
//   - Enterprise profile (roadmap): a Vault / SSO-backed resolver
//     implementing this same interface, injected at startup.
//
// Resolvers never persist resolved values.
type CredentialResolver interface {
	Resolve(ref CredentialRef) (string, error)
}

// SecretStore is the minimal secrets-store surface the connector package
// needs. secrets.SecretManager satisfies it, and tests can supply fakes.
type SecretStore interface {
	GetSecret(group, key string) (string, error)
}

// secretsResolver resolves kind=secret refs through a SecretStore.
type secretsResolver struct {
	store SecretStore
}

// NewSecretsResolver returns a resolver backed by the given secret store.
func NewSecretsResolver(store SecretStore) CredentialResolver {
	return &secretsResolver{store: store}
}

// defaultResolver resolves refs against the process environment and the
// global secrets store. The secrets store is opened lazily on first
// kind=secret resolution so environments that only use kind=env never
// need a master password.
type defaultResolver struct{}

// DefaultResolver returns the personal-profile resolver: kind=env reads
// the process environment, kind=secret reads the encrypted secrets store
// (opening it lazily via secrets.GetSecretManager).
func DefaultResolver() CredentialResolver {
	return &defaultResolver{}
}

// Resolve implements CredentialResolver.
func (d *defaultResolver) Resolve(ref CredentialRef) (string, error) {
	switch ref.Kind {
	case CredentialKindEnv:
		value := os.Getenv(ref.Key)
		if value == "" {
			return "", fmt.Errorf("environment variable %s is not set", ref.Key)
		}
		return value, nil
	case CredentialKindSecret:
		sm, err := secrets.GetSecretManager()
		if err != nil {
			return "", fmt.Errorf("failed to open secrets store: %w", err)
		}
		value, err := sm.GetSecret(ref.Group, ref.Key)
		if err != nil {
			return "", fmt.Errorf("secret %s/%s: %w", ref.Group, ref.Key, err)
		}
		return value, nil
	default:
		return "", fmt.Errorf("unknown credential kind %q (use secret or env)", ref.Kind)
	}
}

// Resolve implements CredentialResolver for the secrets-backed resolver.
func (s *secretsResolver) Resolve(ref CredentialRef) (string, error) {
	if ref.Kind != CredentialKindSecret {
		return "", fmt.Errorf("secrets resolver only handles kind=secret, got %q", ref.Kind)
	}
	if s.store == nil {
		return "", fmt.Errorf("secret store is not configured")
	}
	value, err := s.store.GetSecret(ref.Group, ref.Key)
	if err != nil {
		return "", fmt.Errorf("secret %s/%s: %w", ref.Group, ref.Key, err)
	}
	return value, nil
}
