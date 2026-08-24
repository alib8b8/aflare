// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​​​‌‌​‌‌​‌‌‌‌​​‌​‌​‌‌‌​‌​​‌‌​​‌​​​​‌​​‌​​​​​​​‌‌​​‌‌‌‌‌‌​‌​​‌​‌​​‌​​​​​​​​​​​​​​​​‌​‌‌‌​‌‌‌​​​​​‌‌⁠
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
	"errors"
	"strings"
	"testing"
)

// fakeSecretStore is an in-memory SecretStore for tests.
type fakeSecretStore struct {
	secrets map[string]string
}

func (f *fakeSecretStore) GetSecret(group, key string) (string, error) {
	v, ok := f.secrets[group+"/"+key]
	if !ok {
		return "", errors.New("not found")
	}
	return v, nil
}

func TestSecretsResolver(t *testing.T) {
	store := &fakeSecretStore{secrets: map[string]string{"connectors/pg": "s3cret"}}
	r := NewSecretsResolver(store)

	got, err := r.Resolve(CredentialRef{Kind: CredentialKindSecret, Group: "connectors", Key: "pg"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "s3cret" {
		t.Errorf("Resolve() = %q, want s3cret", got)
	}

	// missing secret → error mentioning group/key
	_, err = r.Resolve(CredentialRef{Kind: CredentialKindSecret, Group: "connectors", Key: "nope"})
	if err == nil || !strings.Contains(err.Error(), "connectors/nope") {
		t.Errorf("expected error mentioning connectors/nope, got %v", err)
	}

	// wrong kind → error
	_, err = r.Resolve(CredentialRef{Kind: CredentialKindEnv, Key: "X"})
	if err == nil || !strings.Contains(err.Error(), "kind=secret") {
		t.Errorf("expected kind error, got %v", err)
	}
}

func TestSecretsResolver_NilStore(t *testing.T) {
	r := NewSecretsResolver(nil)
	_, err := r.Resolve(CredentialRef{Kind: CredentialKindSecret, Group: "g", Key: "k"})
	if err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Errorf("expected not-configured error, got %v", err)
	}
}

func TestDefaultResolver_Env(t *testing.T) {
	t.Setenv("AFLARE_TEST_CONN_PASS", "env-value")
	r := DefaultResolver()

	got, err := r.Resolve(CredentialRef{Kind: CredentialKindEnv, Key: "AFLARE_TEST_CONN_PASS"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "env-value" {
		t.Errorf("Resolve() = %q, want env-value", got)
	}

	// unset env var → error
	_, err = r.Resolve(CredentialRef{Kind: CredentialKindEnv, Key: "AFLARE_TEST_CONN_UNSET"})
	if err == nil || !strings.Contains(err.Error(), "not set") {
		t.Errorf("expected not-set error, got %v", err)
	}
}

func TestDefaultResolver_UnknownKind(t *testing.T) {
	r := DefaultResolver()
	_, err := r.Resolve(CredentialRef{Kind: "vault", Key: "k"})
	if err == nil || !strings.Contains(err.Error(), "unknown credential kind") {
		t.Errorf("expected unknown-kind error, got %v", err)
	}
}
