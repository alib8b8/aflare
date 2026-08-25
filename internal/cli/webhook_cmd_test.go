// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​‌‌​​​‌​‌‌‌​‌‌‌​‌​‌‌‌‌‌​‌‌​​‌‌​​‌‌‌‌‌​​‌‌​​‌​‌‌​​‌‌​​​​​​‌‌​​​‌​​​​​​​​​​​​​​​​​​​‌‌​​​​​‌​‌​​‌​​​⁠
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌​‌​‌​​​‌​‌​‌​‌​‌​​​‌‌​‌‌‌‌‌​‌‌​​​​​‌‌​​​​‌​‌​‌‌​​‌‌​‌​​​​​​​‌​​​​​​‌​‌‌​‌​‌​​​‌‌‌​‌​‌‌‌‌‌​‌‌‌​​‌​​‌‌‌‌​‌‌​​​​‌​​‌​‌​‌‌​​‌‌​​​‌‌​‌‌​​‌​‌‌‌‌​​​​‌‌​‌‌⁠
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
	"strings"
	"testing"
)

// TestHandleWebhook_HelpDispatch verifies the --help/-h paths print usage
// without starting a server.
func TestHandleWebhook_HelpDispatch(t *testing.T) {
	for _, flag := range []string{"-h", "--help"} {
		out := captureStdout(t, func() {
			if err := HandleWebhook([]string{flag}); err != nil {
				t.Errorf("HandleWebhook(%q) returned error: %v", flag, err)
			}
		})
		if !strings.Contains(out, "Usage: aflare webhook") {
			t.Errorf("HandleWebhook(%q) = %q, want usage text", flag, out)
		}
	}
}

// TestHandleWebhook_UnknownArg verifies bad arguments are rejected with the
// usage text instead of starting a server.
func TestHandleWebhook_UnknownArg(t *testing.T) {
	out := captureStdout(t, func() {
		if err := HandleWebhook([]string{"--bogus"}); err == nil {
			t.Error("HandleWebhook(--bogus) should return an error")
		}
	})
	if !strings.Contains(out, "Unknown argument: --bogus") {
		t.Errorf("output = %q, want unknown-argument message", out)
	}
	if !strings.Contains(out, "Usage: aflare webhook") {
		t.Errorf("output = %q, want usage text", out)
	}
}

// TestHandleWebhook_NonLoopbackWithoutSecretRefused guards the security
// invariant from HandleServe: binding a non-loopback address without any
// authentication is refused outright.
func TestHandleWebhook_NonLoopbackWithoutSecretRefused(t *testing.T) {
	t.Setenv("AFLARE_WEBHOOK_SECRET", "")
	err := HandleWebhook([]string{"--host", "0.0.0.0"})
	if err == nil {
		t.Error("expected error when binding 0.0.0.0 without a secret")
	}
}

// TestValidateCommand_Webhook guards the dispatch wiring: `aflare webhook`
// must be a recognized command (main.go dispatches it to HandleWebhook).
func TestValidateCommand_Webhook(t *testing.T) {
	if err := ValidateCommand("webhook"); err != nil {
		t.Errorf("expected command %q to be valid, got error: %v", "webhook", err)
	}
}
