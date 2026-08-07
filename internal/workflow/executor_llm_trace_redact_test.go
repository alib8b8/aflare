// Copyright (c) 2026 aflare Contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package workflow

import (
	"strings"
	"testing"

	"github.com/alib8b8/aflare/internal/nodes"
)

// projectOne is a small helper that runs a single LLMCallTelemetry record
// through projectLLMTelemetry and returns the resulting LLMStepTrace. It
// keeps the redaction tests focused on the trace output rather than the
// plumbing.
func projectOne(t *testing.T, c nodes.LLMCallTelemetry) LLMStepTrace {
	t.Helper()
	out := projectLLMTelemetry([]nodes.LLMCallTelemetry{c})
	if len(out) != 1 {
		t.Fatalf("projectLLMTelemetry returned %d traces, want 1", len(out))
	}
	return out[0]
}

// TestStepTrace_RedactsAPIKey verifies that an AWS/OpenAI-style API key
// (sk-…) embedded in an LLM prompt is scrubbed before it reaches StepTrace.
func TestStepTrace_RedactsAPIKey(t *testing.T) {
	// core.RedactSensitive's sk- pattern requires 20+ alphanumeric chars
	// after the "sk-" prefix.
	secret := "sk-abcdefghijklmnopqrstuvwxyz1234567890"
	tr := projectOne(t, nodes.LLMCallTelemetry{
		NodeName: "openai",
		Prompt:   "Translate this document. My key is " + secret + " please help.",
	})
	if strings.Contains(tr.Prompt, secret) {
		t.Errorf("API key leaked into trace prompt: %q", tr.Prompt)
	}
	if !strings.Contains(tr.Prompt, "sk-****") {
		t.Errorf("expected sk-**** redaction marker, got %q", tr.Prompt)
	}
}

// TestStepTrace_RedactsJWT verifies that a JWT embedded in an LLM prompt is
// replaced with [REDACTED:JWT]. JWT is not covered by core.RedactSensitive;
// it is handled by the extra patterns in redactForTrace.
func TestStepTrace_RedactsJWT(t *testing.T) {
	jwt := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"
	tr := projectOne(t, nodes.LLMCallTelemetry{
		NodeName: "openai",
		Prompt:   "Validate this auth token: " + jwt + " and reply.",
	})
	if strings.Contains(tr.Prompt, jwt) {
		t.Errorf("JWT leaked into trace prompt: %q", tr.Prompt)
	}
	if !strings.Contains(tr.Prompt, "[REDACTED:JWT]") {
		t.Errorf("expected [REDACTED:JWT] marker, got %q", tr.Prompt)
	}
}

// TestStepTrace_RedactsResponse verifies that a PEM private key block
// returned in an LLM response is replaced with [REDACTED:PRIVATE_KEY].
// Private keys are not covered by core.RedactSensitive; they are handled by
// the extra patterns in redactForTrace.
func TestStepTrace_RedactsResponse(t *testing.T) {
	privKey := "-----BEGIN RSA PRIVATE KEY-----\n" +
		"MIIEpAIBAAKCAQEA0d3f5e7e9f1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9\n" +
		"a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1\n" +
		"-----END RSA PRIVATE KEY-----"
	tr := projectOne(t, nodes.LLMCallTelemetry{
		NodeName: "openai",
		Response: "Here is the signing key:\n" + privKey + "\nDone.",
	})
	if strings.Contains(tr.Response, "MIIEpAIBAAKCAQEA") {
		t.Errorf("private key body leaked into trace response: %q", tr.Response)
	}
	if strings.Contains(tr.Response, "BEGIN RSA PRIVATE KEY") {
		t.Errorf("private key header leaked into trace response: %q", tr.Response)
	}
	if !strings.Contains(tr.Response, "[REDACTED:PRIVATE_KEY]") {
		t.Errorf("expected [REDACTED:PRIVATE_KEY] marker, got %q", tr.Response)
	}
}

// TestStepTrace_NoRedactWhenDisabled verifies that setting BOTH
// AFLARE_TRACE_NO_REDACT=1 AND AFLARE_DEBUG_MODE=1 bypasses redaction
// entirely, so the raw prompt/response are preserved verbatim in the trace.
// The dual control (H-8) means a single env var is not enough; production
// safety relies on both being set together. This escape hatch is intended
// for local debugging only.
func TestStepTrace_NoRedactWhenDisabled(t *testing.T) {
	t.Setenv("AFLARE_TRACE_NO_REDACT", "1")
	t.Setenv("AFLARE_DEBUG_MODE", "1")

	secret := "sk-abcdefghijklmnopqrstuvwxyz1234567890"
	jwt := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"
	prompt := "key " + secret
	response := "tok " + jwt

	tr := projectOne(t, nodes.LLMCallTelemetry{
		NodeName: "openai",
		Prompt:   prompt,
		Response: response,
	})
	if tr.Prompt != prompt {
		t.Errorf("redaction should be disabled, but prompt changed: got %q want %q", tr.Prompt, prompt)
	}
	if tr.Response != response {
		t.Errorf("redaction should be disabled, but response changed: got %q want %q", tr.Response, response)
	}
}

// TestStepTrace_NoRedactRequiresDebugMode verifies the H-8 dual control:
// setting AFLARE_TRACE_NO_REDACT=1 ALONE (without AFLARE_DEBUG_MODE=1)
// must NOT bypass redaction. This is the production-safety property — a
// single accidentally-set env var cannot leak sensitive prompts into trace
// files. The secret must still be scrubbed as if neither flag were set.
func TestStepTrace_NoRedactRequiresDebugMode(t *testing.T) {
	t.Setenv("AFLARE_TRACE_NO_REDACT", "1")
	// AFLARE_DEBUG_MODE intentionally NOT set.

	secret := "sk-abcdefghijklmnopqrstuvwxyz1234567890"
	tr := projectOne(t, nodes.LLMCallTelemetry{
		NodeName: "openai",
		Prompt:   "Translate this document. My key is " + secret + " please help.",
	})
	if strings.Contains(tr.Prompt, secret) {
		t.Errorf("API key leaked into trace prompt without debug mode: %q", tr.Prompt)
	}
	if !strings.Contains(tr.Prompt, "sk-****") {
		t.Errorf("expected sk-**** redaction marker (debug mode required), got %q", tr.Prompt)
	}
}

// TestStepTrace_PreservesNormalText verifies that ordinary text without any
// sensitive pattern is passed through unchanged, so redaction does not
// corrupt legitimate trace content.
func TestStepTrace_PreservesNormalText(t *testing.T) {
	prompt := "What is the capital of France?"
	response := "The capital of France is Paris."
	tr := projectOne(t, nodes.LLMCallTelemetry{
		NodeName: "openai",
		Prompt:   prompt,
		Response: response,
	})
	if tr.Prompt != prompt {
		t.Errorf("normal prompt should be unchanged: got %q want %q", tr.Prompt, prompt)
	}
	if tr.Response != response {
		t.Errorf("normal response should be unchanged: got %q want %q", tr.Response, response)
	}
}
