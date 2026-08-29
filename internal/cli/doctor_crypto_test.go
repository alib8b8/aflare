// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​​‌‌​‌‌​‌‌‌‌‌​​​​‌‌‌​‌​​‌‌​​‌​​‌​​​​‌​‌‌​​‌​​‌​‌​​​​​​​​​​​​​​​​​‌​‌​‌​‌​​‌​​‌​‌⁠
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
	"strings"
	"testing"

	"github.com/alib8b8/aflare/internal/history"
	"github.com/alib8b8/aflare/internal/i18n"
	"github.com/alib8b8/aflare/internal/secrets"
)

// runCryptoCompat executes checkCryptoCompat against a temp history dir and
// secrets path, capturing stdout and the collected problems.
func runCryptoCompat(t *testing.T, seedAudit func()) (string, []doctorProblem) {
	t.Helper()
	t.Setenv("AFLARE_AUDIT_HMAC_KEY", "test-key")
	history.SetHistoryDir(t.TempDir())
	if seedAudit != nil {
		seedAudit()
	}

	secretsPath := filepath.Join(t.TempDir(), "secrets.dat")
	var problems []doctorProblem

	out := captureStdout(t, func() {
		checkCryptoCompat(&problems, secretsPath)
	})
	return out, problems
}

// captureStdout redirects os.Stdout during fn and returns what was printed.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	tmp, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatalf("failed to create stdout capture: %v", err)
	}
	old := os.Stdout
	os.Stdout = tmp
	fn()
	os.Stdout = old
	if err := tmp.Close(); err != nil {
		t.Fatalf("failed to close capture: %v", err)
	}
	data, err := os.ReadFile(tmp.Name())
	if err != nil {
		t.Fatalf("failed to read captured stdout: %v", err)
	}
	return string(data)
}

// TestCheckCryptoCompat_AllDefault proves a fully-default deployment (no
// guomi data, legacy or absent secrets) reports no compatibility problems.
func TestCheckCryptoCompat_AllDefault(t *testing.T) {
	t.Setenv("AFLARE_AUDIT_HMAC_ALGO", "")
	t.Setenv("AFLARE_SECRETS_CIPHER", "")

	out, problems := runCryptoCompat(t, func() {
		for i := 0; i < 2; i++ {
			if err := history.AppendAuditLog(history.AuditLog{Action: "workflow_run"}); err != nil {
				t.Fatalf("AppendAuditLog failed: %v", err)
			}
		}
	})

	if len(problems) != 0 {
		t.Fatalf("expected no problems on default data, got %d: %+v", len(problems), problems)
	}
	if !strings.Contains(out, i18n.T("doctor.crypto.audit_sha256", 2)) {
		t.Errorf("output should report 2 sha256 records, got:\n%s", out)
	}
	if !strings.Contains(out, i18n.T("doctor.crypto.secrets_none")) {
		t.Errorf("output should report no secrets store, got:\n%s", out)
	}
}

// TestCheckCryptoCompat_SM3RecordsFlagsProblem proves SM3 audit records turn
// into an actionable compatibility problem with rollback guidance.
func TestCheckCryptoCompat_SM3RecordsFlagsProblem(t *testing.T) {
	t.Setenv("AFLARE_AUDIT_HMAC_ALGO", "sm3")

	out, problems := runCryptoCompat(t, func() {
		if err := history.AppendAuditLog(history.AuditLog{Action: "workflow_run"}); err != nil {
			t.Fatalf("AppendAuditLog failed: %v", err)
		}
	})

	if len(problems) != 1 {
		t.Fatalf("expected 1 problem for SM3 records, got %d: %+v", len(problems), problems)
	}
	if problems[0].category != i18n.T("doctor.cat.compat") || !strings.Contains(problems[0].desc, "SM3") {
		t.Errorf("unexpected problem: %+v", problems[0])
	}
	if !strings.Contains(out, "SM3") || !strings.Contains(out, "⚠") {
		t.Errorf("output should flag SM3 and warn about the env var, got:\n%s", out)
	}
}

// TestCheckCryptoCompat_SM4SecretsFlagsProblem proves an SM4-encrypted store
// is reported with the rollback steps.
func TestCheckCryptoCompat_SM4SecretsFlagsProblem(t *testing.T) {
	t.Setenv("AFLARE_AUDIT_HMAC_ALGO", "")
	t.Setenv("AFLARE_SECRETS_CIPHER", "sm4-gcm")

	dir := t.TempDir()
	path := filepath.Join(dir, "secrets.dat")
	sm, err := secrets.NewSecretManager("pw")
	if err != nil {
		t.Fatalf("NewSecretManager failed: %v", err)
	}
	if err := sm.SaveToFile(path); err != nil {
		t.Fatalf("SaveToFile failed: %v", err)
	}

	history.SetHistoryDir(t.TempDir())
	var problems []doctorProblem
	out := captureStdout(t, func() {
		checkCryptoCompat(&problems, path)
	})

	if len(problems) != 1 {
		t.Fatalf("expected 1 problem for SM4 store, got %d: %+v", len(problems), problems)
	}
	if !strings.Contains(problems[0].hint, "aes-gcm") {
		t.Errorf("rollback hint should mention aes-gcm: %+v", problems[0])
	}
	if !strings.Contains(out, "SM4-GCM") {
		t.Errorf("output should name SM4-GCM, got:\n%s", out)
	}
}
