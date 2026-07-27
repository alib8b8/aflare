// Copyright (c) 2026 llm-box Contributors
//
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

package core

import (
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setSecurityLevel sets LLM_BOX_SECURITY_LEVEL for the duration of the test.
// Using t.Setenv means these tests must not call t.Parallel.
func setSecurityLevel(t *testing.T, level string) {
	t.Helper()
	t.Setenv("LLM_BOX_SECURITY_LEVEL", level)
}

// --- SafeJoinPath ---

func TestSafeJoinPath_NormalRelative(t *testing.T) {
	base := t.TempDir()
	got, err := SafeJoinPath(base, "subdir/file.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(base, "subdir", "file.txt")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSafeJoinPath_EmptyPath(t *testing.T) {
	base := t.TempDir()
	_, err := SafeJoinPath(base, "")
	if err == nil {
		t.Fatal("expected error for empty path, got nil")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("error should mention empty, got: %v", err)
	}
}

func TestSafeJoinPath_AbsolutePath(t *testing.T) {
	base := t.TempDir()
	for _, p := range []string{"/etc/passwd", "\\windows\\system32"} {
		_, err := SafeJoinPath(base, p)
		if err == nil {
			t.Errorf("expected error for absolute path %q, got nil", p)
		}
		if !strings.Contains(err.Error(), "absolute") {
			t.Errorf("error should mention absolute for %q, got: %v", p, err)
		}
	}
}

func TestSafeJoinPath_ParentTraversal(t *testing.T) {
	base := t.TempDir()
	for _, p := range []string{"..", "../secret", "../../etc/passwd", "sub/../../.."} {
		_, err := SafeJoinPath(base, p)
		if err == nil {
			t.Errorf("expected error for traversal path %q, got nil", p)
		}
		if !strings.Contains(err.Error(), "escapes") {
			t.Errorf("error should mention escapes for %q, got: %v", p, err)
		}
	}
}

func TestSafeJoinPath_WithinSubdir(t *testing.T) {
	base := t.TempDir()
	// ".." that stays inside base (e.g. subdir/.. == base) should be allowed.
	got, err := SafeJoinPath(base, "sub/../file.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(base, "file.txt")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSafeJoinPath_SymlinkEscapeAtL2(t *testing.T) {
	setSecurityLevel(t, "L2")
	base := t.TempDir()
	// target outside base
	outside := t.TempDir()
	target := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(target, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "escape")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	_, err := SafeJoinPath(base, "escape")
	if err == nil {
		t.Fatal("expected error for symlink escape at L2, got nil")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Errorf("error should mention symlink, got: %v", err)
	}
}

func TestSafeJoinPath_SymlinkInsideAtL2(t *testing.T) {
	setSecurityLevel(t, "L2")
	base := t.TempDir()
	// target inside base
	target := filepath.Join(base, "real.txt")
	if err := os.WriteFile(target, []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	got, err := SafeJoinPath(base, "link.txt")
	if err != nil {
		t.Fatalf("unexpected error for internal symlink at L2: %v", err)
	}
	// EvalSymlinks resolves to the real target path.
	want := target
	if got != want {
		t.Errorf("got %q, want resolved %q", got, want)
	}
}

func TestSafeJoinPath_SymlinkNotResolvedAtL0(t *testing.T) {
	setSecurityLevel(t, "L0")
	base := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(target, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "escape")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	got, err := SafeJoinPath(base, "escape")
	if err != nil {
		t.Fatalf("at L0 symlink should not be validated: %v", err)
	}
	// At L0 the unresolved abs path is returned.
	want := filepath.Join(base, "escape")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// --- ValidateReadPathIn / ValidateWritePathIn ---

func TestValidateReadPathIn_Normal(t *testing.T) {
	base := t.TempDir()
	got, err := ValidateReadPathIn(base, "data.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(base, "data.txt")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestValidateReadPathIn_EmptyBase(t *testing.T) {
	_, err := ValidateReadPathIn("", "data.txt")
	if err == nil {
		t.Fatal("expected error for empty base dir, got nil")
	}
}

func TestValidateReadPathIn_Traversal(t *testing.T) {
	base := t.TempDir()
	_, err := ValidateReadPathIn(base, "../../etc/passwd")
	if err == nil {
		t.Fatal("expected error for traversal, got nil")
	}
}

func TestValidateWritePathIn_NormalTxt(t *testing.T) {
	setSecurityLevel(t, "L1")
	base := t.TempDir()
	got, err := ValidateWritePathIn(base, "out.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(base, "out.txt")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestValidateWritePathIn_EmptyBase(t *testing.T) {
	_, err := ValidateWritePathIn("", "out.txt")
	if err == nil {
		t.Fatal("expected error for empty base dir, got nil")
	}
}

func TestValidateWritePathIn_Traversal(t *testing.T) {
	setSecurityLevel(t, "L1")
	base := t.TempDir()
	_, err := ValidateWritePathIn(base, "../../etc/passwd")
	if err == nil {
		t.Fatal("expected error for traversal, got nil")
	}
}

func TestValidateWritePathIn_DotfileRejectedAtL1(t *testing.T) {
	setSecurityLevel(t, "L1")
	base := t.TempDir()
	_, err := ValidateWritePathIn(base, ".hidden")
	if err == nil {
		t.Fatal("expected error for dotfile at L1, got nil")
	}
	if !strings.Contains(err.Error(), "dotfile") {
		t.Errorf("error should mention dotfile, got: %v", err)
	}
}

func TestValidateWritePathIn_DotfileAllowedAtL0(t *testing.T) {
	setSecurityLevel(t, "L0")
	base := t.TempDir()
	// ".hidden" has extension ".hidden" which is not in the forbidden map,
	// and the dotfile check is L1-gated, so it should pass at L0.
	got, err := ValidateWritePathIn(base, ".hidden")
	if err != nil {
		t.Fatalf("expected dotfile allowed at L0, got error: %v", err)
	}
	if !strings.HasSuffix(got, ".hidden") {
		t.Errorf("got %q", got)
	}
}

func TestValidateWritePathIn_ForbiddenExtensions(t *testing.T) {
	setSecurityLevel(t, "L1")
	base := t.TempDir()
	for _, ext := range []string{".exe", ".dll", ".so", ".dylib", ".env", ".sh", ".bat", ".ps1", ".msi", ".apk", ".deb", ".rpm"} {
		_, err := ValidateWritePathIn(base, "file"+ext)
		if err == nil {
			t.Errorf("expected error for forbidden ext %s, got nil", ext)
		}
		if err != nil && !strings.Contains(err.Error(), "not allowed") {
			t.Errorf("error for %s should mention not allowed, got: %v", ext, err)
		}
	}
}

func TestValidateWritePathL3_ScriptExtsRejected(t *testing.T) {
	setSecurityLevel(t, "L3")
	base := t.TempDir()
	for _, ext := range []string{".py", ".rb", ".php", ".pl"} {
		_, err := ValidateWritePathIn(base, "script"+ext)
		if err == nil {
			t.Errorf("expected error for script ext %s at L3, got nil", ext)
		}
	}
}

func TestValidateWritePathL3_UnknownExtRejected(t *testing.T) {
	setSecurityLevel(t, "L3")
	base := t.TempDir()
	_, err := ValidateWritePathIn(base, "file.xyz")
	if err == nil {
		t.Fatal("expected error for unknown ext at L3, got nil")
	}
	if !strings.Contains(err.Error(), "not allowed") {
		t.Errorf("error should mention not allowed, got: %v", err)
	}
}

func TestValidateWritePathL3_AllowedExt(t *testing.T) {
	setSecurityLevel(t, "L3")
	base := t.TempDir()
	for _, ext := range []string{".txt", ".md", ".json", ".yaml", ".csv", ".html", ".go", ".png"} {
		_, err := ValidateWritePathIn(base, "file"+ext)
		if err != nil {
			t.Errorf("expected ext %s allowed at L3, got error: %v", ext, err)
		}
	}
}

func TestValidateWritePathL3_NoExtAllowed(t *testing.T) {
	setSecurityLevel(t, "L3")
	base := t.TempDir()
	// Empty extension: the allowlist check skips when ext == "".
	_, err := ValidateWritePathIn(base, "README")
	if err != nil {
		t.Errorf("expected no-ext file allowed at L3, got error: %v", err)
	}
}

// --- ValidateReadPath / ValidateWritePath (workDir-based wrappers) ---

func TestValidateReadPath_UsesWorkDir(t *testing.T) {
	setSecurityLevel(t, "L1")
	base := t.TempDir()
	old := workDir
	workDir = base
	defer func() { workDir = old }()
	got, err := ValidateReadPath("file.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(base, "file.txt")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestValidateWritePath_UsesWorkDir(t *testing.T) {
	setSecurityLevel(t, "L1")
	base := t.TempDir()
	old := workDir
	workDir = base
	defer func() { workDir = old }()
	_, err := ValidateWritePath("out.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// dotfile via wrapper still blocked
	_, err = ValidateWritePath(".hidden")
	if err == nil {
		t.Fatal("expected dotfile blocked via wrapper, got nil")
	}
}

func TestGetWorkDir(t *testing.T) {
	old := workDir
	workDir = "/some/path"
	defer func() { workDir = old }()
	if got := GetWorkDir(); got != "/some/path" {
		t.Errorf("GetWorkDir = %q, want /some/path", got)
	}
}

// --- ValidateURL ---

func TestValidateURL_PublicIPLiteral(t *testing.T) {
	// 8.8.8.8 is a public IP; no DNS lookup needed.
	if err := ValidateURL("http://8.8.8.8/path"); err != nil {
		t.Errorf("expected nil for public IP, got: %v", err)
	}
	if err := ValidateURL("https://8.8.8.8/"); err != nil {
		t.Errorf("expected nil for https public IP, got: %v", err)
	}
}

func TestValidateURL_LocalhostRejected(t *testing.T) {
	for _, raw := range []string{
		"http://localhost/",
		"http://localhost.localdomain/",
		"http://ip6-localhost/",
		"http://ip6-loopback/",
	} {
		if err := ValidateURL(raw); err == nil {
			t.Errorf("expected error for %q, got nil", raw)
		}
	}
}

func TestValidateURL_PrivateIPRejected(t *testing.T) {
	for _, raw := range []string{
		"http://192.168.1.1/",
		"http://10.0.0.1/",
		"http://172.16.0.1/",
		"http://127.0.0.1/",
		"http://169.254.169.254/", // link-local + reserved
		"http://0.0.0.0/",         // unspecified + reserved
	} {
		if err := ValidateURL(raw); err == nil {
			t.Errorf("expected error for %q, got nil", raw)
		}
	}
}

func TestValidateURL_NonHTTPSchemeRejected(t *testing.T) {
	for _, raw := range []string{
		"ftp://8.8.8.8/",
		"file:///etc/passwd",
		"gopher://8.8.8.8/",
	} {
		if err := ValidateURL(raw); err == nil {
			t.Errorf("expected error for %q, got nil", raw)
		}
		if err := ValidateURL(raw); err != nil && !strings.Contains(err.Error(), "http") {
			t.Errorf("error should mention http for %q, got: %v", raw, err)
		}
	}
}

func TestValidateURL_NoSchemeRejected(t *testing.T) {
	if err := ValidateURL("8.8.8.8"); err == nil {
		t.Fatal("expected error for URL without scheme, got nil")
	}
}

func TestValidateURL_NoHostRejected(t *testing.T) {
	if err := ValidateURL("http:///path"); err == nil {
		t.Fatal("expected error for URL without host, got nil")
	}
}

func TestValidateURL_UserInfoRejected(t *testing.T) {
	if err := ValidateURL("http://user:pass@8.8.8.8/"); err == nil {
		t.Fatal("expected error for URL with userinfo, got nil")
	}
	if !strings.Contains(ValidateURL("http://user:pass@8.8.8.8/").Error(), "userinfo") {
		t.Error("error should mention userinfo")
	}
}

func TestValidateURL_PublicIPv6(t *testing.T) {
	// 2606:4700:4700::1111 is Cloudflare DNS (public), literal IP -> no DNS.
	if err := ValidateURL("http://[2606:4700:4700::1111]/"); err != nil {
		t.Errorf("expected nil for public IPv6, got: %v", err)
	}
}

func TestValidateURL_PrivateIPv6Rejected(t *testing.T) {
	for _, raw := range []string{
		"http://[::1]/",     // loopback
		"http://[fc00::1]/", // ULA private
		"http://[fe80::1]/", // link-local
	} {
		if err := ValidateURL(raw); err == nil {
			t.Errorf("expected error for %q, got nil", raw)
		}
	}
}

// --- ValidateIP (direct) ---

func TestValidateIP(t *testing.T) {
	cases := []struct {
		name    string
		ip      net.IP
		display string
		wantErr bool
	}{
		{"public-v4", net.IPv4(8, 8, 8, 8), "8.8.8.8", false},
		{"public-v6", net.ParseIP("2606:4700:4700::1111"), "2606:4700:4700::1111", false},
		{"loopback-v4", net.IPv4(127, 0, 0, 1), "127.0.0.1", true},
		{"loopback-v6", net.ParseIP("::1"), "::1", true},
		{"private-10", net.IPv4(10, 0, 0, 1), "10.0.0.1", true},
		{"private-192168", net.IPv4(192, 168, 1, 1), "192.168.1.1", true},
		{"private-17216", net.IPv4(172, 16, 0, 1), "172.16.0.1", true},
		{"linklocal", net.IPv4(169, 254, 1, 1), "169.254.1.1", true},
		{"unspecified", net.IPv4(0, 0, 0, 0), "0.0.0.0", true},
		{"multicast", net.IPv4(224, 0, 0, 1), "224.0.0.1", true},
		{"reserved-testnet1", net.IPv4(192, 0, 2, 1), "192.0.2.1", true},
		{"reserved-cgnat", net.IPv4(100, 64, 0, 1), "100.64.0.1", true},
		{"reserved-ula", net.ParseIP("fc00::1"), "fc00::1", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateIP(c.ip, c.display)
			if c.wantErr && err == nil {
				t.Errorf("ValidateIP(%s) expected error, got nil", c.display)
			}
			if !c.wantErr && err != nil {
				t.Errorf("ValidateIP(%s) expected nil, got: %v", c.display, err)
			}
		})
	}
}

// --- IsReservedIP ---

func TestIsReservedIP(t *testing.T) {
	reserved := []net.IP{
		net.IPv4(0, 0, 0, 0),
		net.IPv4(169, 254, 1, 1),
		net.IPv4(192, 0, 2, 1),    // TEST-NET-1
		net.IPv4(198, 51, 100, 1), // TEST-NET-2
		net.IPv4(203, 0, 113, 1),  // TEST-NET-3
		net.IPv4(100, 64, 0, 1),   // CGNAT low
		net.IPv4(100, 127, 0, 1),  // CGNAT high
		net.ParseIP("fc00::1"),    // ULA
	}
	for _, ip := range reserved {
		if !IsReservedIP(ip) {
			t.Errorf("IsReservedIP(%s) = false, want true", ip)
		}
	}
	notReserved := []net.IP{
		net.IPv4(8, 8, 8, 8),
		net.IPv4(1, 1, 1, 1),
		net.IPv4(100, 128, 0, 1), // just above CGNAT
		net.IPv4(100, 63, 0, 1),  // just below CGNAT
		net.ParseIP("2606:4700:4700::1111"),
		net.ParseIP("::1"), // loopback handled separately, not "reserved" here
	}
	for _, ip := range notReserved {
		if IsReservedIP(ip) {
			t.Errorf("IsReservedIP(%s) = true, want false", ip)
		}
	}
}

// --- LLM Endpoint validators ---

func TestValidateLMLEndpoint_LocalhostAllowed(t *testing.T) {
	for _, raw := range []string{
		"http://localhost:11434/",
		"http://127.0.0.1:11434/",
		"http://localhost.localdomain:11434/",
		"http://ip6-localhost:11434/",
		"http://[::1]:11434/",
	} {
		if err := ValidateLMLEndpoint(raw); err != nil {
			t.Errorf("expected nil for LLM endpoint %q, got: %v", raw, err)
		}
	}
}

func TestValidateLMLEndpoint_PrivateNonLoopbackRejected(t *testing.T) {
	for _, raw := range []string{
		"http://192.168.1.1:11434/",
		"http://10.0.0.1:11434/",
		"http://169.254.169.254:11434/",
	} {
		if err := ValidateLMLEndpoint(raw); err == nil {
			t.Errorf("expected error for LLM endpoint %q, got nil", raw)
		}
	}
}

func TestValidateLMLEndpoint_NonHTTPRejected(t *testing.T) {
	if err := ValidateLMLEndpoint("ftp://localhost:11434/"); err == nil {
		t.Fatal("expected error for non-http LLM endpoint, got nil")
	}
}

func TestValidateLMLEndpoint_UserInfoRejected(t *testing.T) {
	if err := ValidateLMLEndpoint("http://user:pass@localhost:11434/"); err == nil {
		t.Fatal("expected error for userinfo LLM endpoint, got nil")
	}
}

func TestValidateLMLEndpointIPAllowLoopback(t *testing.T) {
	cases := []struct {
		name    string
		ip      net.IP
		wantErr bool
	}{
		{"loopback-v4", net.IPv4(127, 0, 0, 1), false},
		{"loopback-v6", net.ParseIP("::1"), false},
		{"public", net.IPv4(8, 8, 8, 8), false},
		{"private-10", net.IPv4(10, 0, 0, 1), false}, // not blocked by this func
		{"linklocal", net.IPv4(169, 254, 1, 1), true},
		{"unspecified", net.IPv4(0, 0, 0, 0), true},
		{"multicast", net.IPv4(224, 0, 0, 1), true},
		{"reserved", net.IPv4(192, 0, 2, 1), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateLMLEndpointIPAllowLoopback(c.ip, c.ip.String())
			if c.wantErr && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !c.wantErr && err != nil {
				t.Errorf("expected nil, got: %v", err)
			}
		})
	}
}

func TestValidateLMLEndpointIP_LoopbackAllowed(t *testing.T) {
	if err := ValidateLMLEndpointIP(net.IPv4(127, 0, 0, 1), "127.0.0.1"); err != nil {
		t.Errorf("expected nil for loopback, got: %v", err)
	}
	if err := ValidateLMLEndpointIP(net.IPv4(192, 168, 1, 1), "192.168.1.1"); err == nil {
		t.Error("expected error for private, got nil")
	}
}

// --- RedactAPIKey ---

func TestRedactAPIKey(t *testing.T) {
	if got := RedactAPIKey("short"); got != "****" {
		t.Errorf("RedactAPIKey(\"short\") = %q, want ****", got)
	}
	if got := RedactAPIKey("12345678"); got != "****" {
		t.Errorf("RedactAPIKey(8 chars) = %q, want ****", got)
	}
	got := RedactAPIKey("sk-1234567890abcdef")
	want := "sk-1****cdef"
	if got != want {
		t.Errorf("RedactAPIKey(long) = %q, want %q", got, want)
	}
}

// --- IsSensitiveKey ---

func TestIsSensitiveKey(t *testing.T) {
	sensitive := []string{"api_key", "apikey", "token", "bearer", "password", "passwd", "secret", "auth", "user_token", "x-api-key", "my-secret-field", "Authorization"}
	for _, k := range sensitive {
		if !IsSensitiveKey(k) {
			t.Errorf("IsSensitiveKey(%q) = false, want true", k)
		}
	}
	notSensitive := []string{"name", "value", "data", "model", "endpoint", "input", "output", "user", "id"}
	for _, k := range notSensitive {
		if IsSensitiveKey(k) {
			t.Errorf("IsSensitiveKey(%q) = true, want false", k)
		}
	}
}

// --- RedactSensitive ---

func TestRedactSensitive(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"bearer", "Bearer abc123def456", "Bearer ****"},
		{"authorization", "authorization: secrettoken", "authorization: ****"},
		{"api_key", "api_key=ABCDEF123456", "api_key=****"},
		{"password", "password=hunter2", "password=****"},
		{"passwd", "passwd=hunter2", "passwd=****"},
		{"token", "token=xyz123", "token=****"},
		{"secret", "secret=abc", "secret=****"},
		{"ghp", "key: ghp_0123456789abcdefghij", "key: ghp_****"},
		{"sk", "key=sk-0123456789abcdefghij", "key=sk-****"},
		{"xoxb", "key: xoxb-0123456789-abcd", "key: xoxb-****"},
		{"url-creds", "https://user:secret@host/path", "https://user:****@host/path"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := RedactSensitive(c.input)
			if got != c.want {
				t.Errorf("RedactSensitive(%q) = %q, want %q", c.input, got, c.want)
			}
		})
	}
}

func TestRedactSensitive_Empty(t *testing.T) {
	if got := RedactSensitive(""); got != "" {
		t.Errorf("RedactSensitive(\"\") = %q, want \"\"", got)
	}
}

func TestRedactSensitive_NoMatch(t *testing.T) {
	in := "just some plain text with no secrets"
	if got := RedactSensitive(in); got != in {
		t.Errorf("RedactSensitive unchanged expected, got %q", got)
	}
}

func TestRedactSensitive_Truncation(t *testing.T) {
	in := strings.Repeat("a", 1500)
	got := RedactSensitive(in)
	if !strings.HasSuffix(got, "... (truncated)") {
		t.Errorf("expected truncated suffix, got suffix %q", got[len(got)-30:])
	}
	if len(got) != 1000+len("... (truncated)") {
		t.Errorf("expected length %d, got %d", 1000+len("... (truncated)"), len(got))
	}
	if !strings.HasPrefix(got, strings.Repeat("a", 1000)) {
		t.Errorf("expected first 1000 'a' preserved, got prefix %q", got[:30])
	}
}

// --- HTTPRedirectValidator ---

func TestHTTPRedirectValidator_UnderLimit(t *testing.T) {
	called := false
	validator := HTTPRedirectValidator(func(s string) error {
		called = true
		return nil
	})
	req := &http.Request{URL: &url.URL{Scheme: "http", Host: "8.8.8.8"}}
	via := make([]*http.Request, 9) // 9 prior redirects
	if err := validator(req, via); err != nil {
		t.Errorf("expected nil for 9 redirects, got: %v", err)
	}
	if !called {
		t.Error("validator function was not called")
	}
}

func TestHTTPRedirectValidator_TooMany(t *testing.T) {
	validator := HTTPRedirectValidator(func(s string) error { return nil })
	req := &http.Request{URL: &url.URL{Scheme: "http", Host: "8.8.8.8"}}
	via := make([]*http.Request, 10) // 10 prior redirects -> rejected
	err := validator(req, via)
	if err == nil {
		t.Fatal("expected error for 10 redirects, got nil")
	}
	if !strings.Contains(err.Error(), "too many redirects") {
		t.Errorf("error should mention too many redirects, got: %v", err)
	}
}

func TestHTTPRedirectValidator_ValidatorError(t *testing.T) {
	wantErr := errors.New("blocked by validator")
	validator := HTTPRedirectValidator(func(s string) error {
		return wantErr
	})
	req := &http.Request{URL: &url.URL{Scheme: "http", Host: "8.8.8.8"}}
	via := make([]*http.Request, 1)
	if err := validator(req, via); err == nil {
		t.Fatal("expected error from validator, got nil")
	}
}
