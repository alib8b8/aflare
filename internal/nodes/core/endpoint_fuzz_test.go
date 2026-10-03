// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌‌​​​​​​​‌‌‌​‌​​​​​‌‌​​‌‌​​​‌​‌‌‌​‌​​‌​‌​​​‌​​​‌​​​‌‌​​​​‌​​​​‌‌​‌​‌​​​‌​​​​​​​​​​​​​​​​‌​‌‌‌​​‌​‌​‌​​‌​⁠
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
	"runtime"
	"strings"
	"testing"
	"time"
)

// FuzzValidateLMLEndpoint fuzzes ValidateLMLEndpoint with a stubbed DNS
// resolver (lookupIP) so the target is hermetic: real DNS for
// fuzz-generated hostnames is slow and non-deterministic on CI runners,
// which intermittently tripped the 5s hang guard below in the nightly
// soak (e.g. for the trailing-dot FQDN "exXmple.Com.", now a seed).
//
// The stub is installed before f.Fuzz and restored afterwards. go test
// -fuzz re-runs this function in every worker process, so the stub (and
// its restore) applies to seeds, corpus, and fuzzing alike.
func FuzzValidateLMLEndpoint(f *testing.F) {
	if testing.Short() {
		f.Skip("skipping fuzz test in short mode")
	}

	f.Add("http://localhost:11434")
	f.Add("https://api.openai.com/v1")
	f.Add("http://192.168.1.1:8080")
	f.Add("not-a-url")
	f.Add("")
	f.Add("ftp://example.com")
	f.Add("http://user:pass@example.com")
	f.Add("http://127.0.0.1:8080")
	f.Add("https://example.com:443/path")
	f.Add("http://[::1]:8080")
	// Regression seeds from nightly-soak findings: trailing-dot FQDN and
	// mixed-case hosts previously caused >5s real-DNS lookups on runners.
	f.Add("https://exXmple.Com.")
	f.Add("http://ExAmPlE.CoM.:8443")

	origLookup := lookupIP
	lookupIP = func(host string) ([]net.IP, error) {
		// Deterministic resolver: normalize case and the trailing FQDN
		// dot, then map known names to interesting IP classes (public,
		// private, loopback, link-local, multi-answer) and fail
		// everything else with NXDOMAIN.
		name := strings.ToLower(strings.TrimSuffix(host, "."))
		switch name {
		case "good.example":
			return []net.IP{net.ParseIP("93.184.216.34")}, nil
		case "private.example":
			return []net.IP{net.ParseIP("10.0.0.5")}, nil
		case "loopback.example":
			return []net.IP{net.ParseIP("127.0.0.1")}, nil
		case "metadata.example":
			return []net.IP{net.ParseIP("169.254.169.254")}, nil
		case "multi.example":
			return []net.IP{net.ParseIP("93.184.216.34"), net.ParseIP("192.168.1.1")}, nil
		}
		return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
	}
	defer func() { lookupIP = origLookup }()

	f.Fuzz(func(t *testing.T, rawURL string) {
		done := make(chan struct{})
		var panicErr interface{}

		go func() {
			defer func() {
				if r := recover(); r != nil {
					panicErr = r
				}
				close(done)
			}()
			_ = ValidateLMLEndpoint(rawURL)
		}()

		select {
		case <-done:
			if panicErr != nil {
				t.Fatalf("ValidateLMLEndpoint panicked: %v\nurl=%q", panicErr, rawURL)
			}
		case <-time.After(5 * time.Second):
			buf := make([]byte, 1<<20)
			n := runtime.Stack(buf, true)
			t.Fatalf("ValidateLMLEndpoint timed out\nurl=%q\n%s", rawURL, buf[:n])
		}
	})
}
