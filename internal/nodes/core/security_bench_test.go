// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌‌​‌‌​​‌‌‌‌‌‌‌​​​​‌​​​​​‌​​​‌​‌‌‌‌‌​​​​‌‌‌‌‌‌​‌‌​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​⁠
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
	"net"
	"strings"
	"testing"
)

// BenchmarkSafeJoinPath benchmarks the per-request path validation hot path.
// L0 is the default security level (no symlink evaluation); L2+ adds
// filepath.EvalSymlinks which is disk-bound and intentionally not exercised
// here to keep the benchmark deterministic and focused on the string-op cost.
func BenchmarkSafeJoinPath(b *testing.B) {
	b.Setenv("AFLARE_SECURITY_LEVEL", "L0")
	base := b.TempDir()

	cases := []struct {
		name string
		path string
	}{
		{"simple", "file.txt"},
		{"nested", "subdir/deep/nested/file.txt"},
		{"traversal_blocked", "../../etc/passwd"},
		{"absolute_blocked", "/etc/passwd"},
	}

	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = SafeJoinPath(base, c.path)
			}
		})
	}
}

// BenchmarkSafeJoinPath_Parallel exercises path validation under concurrent
// load, approximating real HTTP traffic where many goroutines validate paths
// at once. SafeJoinPath is read-only on the filesystem tree, so concurrent
// access is safe.
func BenchmarkSafeJoinPath_Parallel(b *testing.B) {
	b.Setenv("AFLARE_SECURITY_LEVEL", "L0")
	base := b.TempDir()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = SafeJoinPath(base, "subdir/deep/file.txt")
		}
	})
}

// BenchmarkValidateURL benchmarks SSRF validation. IP literals are used to
// avoid non-deterministic DNS lookups; hostname validation requires real
// network resolution (net.LookupIP) and is intentionally not benchmarked.
func BenchmarkValidateURL(b *testing.B) {
	cases := []struct {
		name string
		url  string
	}{
		{"public_ip", "http://8.8.8.8/path/to/resource"},
		{"https_public", "https://1.1.1.1/health"},
		{"localhost_blocked", "http://localhost:8080/"},
		{"private_ip_blocked", "http://10.0.0.1/admin"},
		{"bad_scheme", "ftp://example.com/file"},
	}

	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = ValidateURL(c.url)
			}
		})
	}
}

// BenchmarkValidateIP benchmarks the pure IP-range checks (no DNS, no URL
// parsing) that underpin both ValidateURL and the dial-time re-validation.
func BenchmarkValidateIP(b *testing.B) {
	ips := []struct {
		name string
		ip   string
	}{
		{"public", "8.8.8.8"},
		{"loopback", "127.0.0.1"},
		{"private", "10.0.0.1"},
		{"link_local", "169.254.1.1"},
	}
	for _, c := range ips {
		b.Run(c.name, func(b *testing.B) {
			ip := net.ParseIP(c.ip)
			if ip == nil {
				b.Fatalf("failed to parse IP %s", c.ip)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = ValidateIP(ip, c.ip)
			}
		})
	}
}

// BenchmarkRedactSensitive benchmarks secret redaction over inputs of
// varying size and secret density. This runs on every logged/redacted string
// in the hot path.
func BenchmarkRedactSensitive(b *testing.B) {
	plain := strings.Repeat("The quick brown fox jumps over the lazy dog. ", 20)
	withSecrets := "Authorization: Bearer abc123def456 " +
		"api_key=sk-ABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890 " +
		"ghp_1234567890abcdefghijklmnopqrstuvwxyz123456 " +
		"password=hunter2 https://user:pass@host/path"
	largeWithSecrets := plain + withSecrets

	cases := []struct {
		name  string
		input string
	}{
		{"plain_small", "Hello world, no secrets here."},
		{"plain_large", plain},
		{"with_secrets", withSecrets},
		{"large_with_secrets", largeWithSecrets},
	}

	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = RedactSensitive(c.input)
			}
		})
	}
}

// BenchmarkRedactAPIKey benchmarks the lightweight API-key masking helper
// used in logging hot paths.
func BenchmarkRedactAPIKey(b *testing.B) {
	keys := []string{
		"short",
		"sk-1234567890abcdef",
		"ghp_1234567890abcdefghijklmnopqrstuvwxyz123456",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = RedactAPIKey(keys[i%len(keys)])
	}
}

// BenchmarkIsSensitiveKey benchmarks the key-name heuristic used to decide
// whether a parameter value should be redacted.
func BenchmarkIsSensitiveKey(b *testing.B) {
	keys := []string{"api_key", "model", "endpoint", "password", "token", "provider", "max_tokens", "user_api_key"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = IsSensitiveKey(keys[i%len(keys)])
	}
}
