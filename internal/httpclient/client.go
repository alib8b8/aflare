// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​‌​​‌‌​‌​​‌​​‌‌​‌​‌‌​‌​​‌‌​​‌​​​‌‌‌‌‌​​​‌‌‌‌‌‌‌​​​​​​​​​​​​​​​​​​​​​‌‌‌‌​​​​​​‌​⁠
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

// Package httpclient is the single source of truth for outbound HTTP
// clients across aflare. It centralizes two concerns that were
// previously copy-pasted into four packages (nodes/core, registry, mcp,
// and the meta/memory packages' bare &http.Client{} literals):
//
//  1. SSRF defense via a DialContext that re-resolves the hostname and
//     validates every resolved IP *at connect time*, closing the
//     TOCTOU/DNS-rebinding window that exists when validation is done
//     before the request and the dial happens later.
//  2. Connection-pool tuning (MaxIdleConns / MaxIdleConnsPerHost /
//     IdleConnTimeout / TLSHandshakeTimeout / ExpectContinueTimeout).
//     The stdlib defaults (MaxIdleConnsPerHost==2) starve high-fan-out
//     workflows and LLM streaming under concurrent load, while unbounded
//     idle conns leak sockets on long-running processes.
//
// The package is an intentional leaf: it imports only the standard
// library so that low-level packages (meta, registry, mcp, memory) can
// depend on it without risking cycles through nodes/core.
package httpclient

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

// Validator decides whether a resolved IP may be dialed. displayHost is
// the original hostname (pre-resolution) and is used only for error
// messages. Returning a non-nil error aborts the dial.
//
// Callers pick one of the pre-built validators (ValidatePublic,
// ValidateAllowLoopback) or supply their own to encode package-specific
// policy (e.g. nodes/core gates loopback on an env var at dial time).
type Validator func(ip net.IP, displayHost string) error

// Options configures a client built by NewClient. The zero value is not
// usable; at minimum Timeout and Validator must be set.
type Options struct {
	// Timeout is the end-to-end client timeout applied to a single
	// request (including redirects). Zero means no timeout — almost
	// always a bug, so NewClient rejects it.
	Timeout time.Duration
	// Validator is invoked for every resolved IP at dial time. Must be
	// non-nil; use ValidatePublic if you have no opinion.
	Validator Validator
	// CheckRedirect, if non-nil, is forwarded to the resulting
	// http.Client. Callers use it to re-validate redirect targets
	// (e.g. to reject redirects onto loopback after the initial URL
	// passed validation). When nil, http.Client's default policy
	// (follow up to 10 redirects) applies.
	CheckRedirect func(req *http.Request, via []*http.Request) error
}

// Pool tuning defaults. These are package-level constants (not Options
// fields) on purpose: every outbound client in aflare should use the
// same pool sizing so that the global connection ceiling is predictable.
// If a caller genuinely needs different sizing it can build its own
// *http.Transport — but none currently does.
const (
	maxIdleConns          = 100
	maxIdleConnsPerHost   = 10
	idleConnTimeout       = 90 * time.Second
	tlsHandshakeTimeout   = 10 * time.Second
	expectContinueTimeout = 1 * time.Second
)

// NewClient builds an *http.Client whose Transport dials through the
// supplied Validator and shares the package-wide pool tuning. The
// returned client is safe for concurrent use and intended to be held as
// a package-level var (not allocated per-request, which would defeat
// connection reuse).
//
// NewClient panics on misconfiguration (zero Timeout or nil Validator)
// because these are programmer errors that should surface at init, not
// at the first request.
func NewClient(opts Options) *http.Client {
	if opts.Timeout <= 0 {
		panic("httpclient: NewClient requires a positive Timeout")
	}
	if opts.Validator == nil {
		panic("httpclient: NewClient requires a Validator")
	}
	validator := opts.Validator
	return &http.Client{
		Timeout:       opts.Timeout,
		CheckRedirect: opts.CheckRedirect,
		Transport:     newTransport(validator),
	}
}

// newTransport returns an *http.Transport that resolves addr's host,
// runs every resolved IP through validator, and then dials the first
// surviving IP. This is the SSRF core: validating *after* DNS resolution
// but *before* the TCP connect defeats DNS rebinding, where an attacker
// flips a hostname from a public IP (passes pre-request validation) to a
// private IP (would be dialed) between the two steps.
//
// Proxy is set to http.ProxyFromEnvironment so HTTP_PROXY/HTTPS_PROXY/NO_PROXY
// are honored. This matters both for production (corporate egress proxies)
// and for tests that force a dial failure by pointing the proxy at a closed
// port. A custom DialContext alone would otherwise bypass the proxy and dial
// the target host directly, silently defeating both use cases.
func newTransport(validator Validator) *http.Transport {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}
			for _, ip := range ips {
				if err := validator(ip.IP, host); err != nil {
					return nil, err
				}
			}
			// Dial the first resolved IP. We do not round-robin across
			// resolved IPs because http.Transport already cached the
			// connection by the original addr; reconnecting to a
			// different IP on a keep-alive reuse would be surprising.
			if len(ips) > 0 {
				addr = net.JoinHostPort(ips[0].IP.String(), port)
			}
			return (&net.Dialer{}).DialContext(ctx, network, addr)
		},
		MaxIdleConns:          maxIdleConns,
		MaxIdleConnsPerHost:   maxIdleConnsPerHost,
		IdleConnTimeout:       idleConnTimeout,
		TLSHandshakeTimeout:   tlsHandshakeTimeout,
		ExpectContinueTimeout: expectContinueTimeout,
	}
}

// ValidatePublic rejects every IP class that should not be reachable
// from production outbound code: loopback, private (RFC 1918),
// link-local, unspecified, multicast, and reserved ranges
// (TEST-NET/CGNAT/0.0.0.0/8/ULA). Use this for clients that talk only
// to public internet endpoints (e.g. the public registry, GitHub).
func ValidatePublic(ip net.IP, displayHost string) error {
	if ip.IsLoopback() {
		return fmt.Errorf("access to loopback address %s is not allowed", displayHost)
	}
	return validateNonLoopback(ip, displayHost)
}

// ValidateAllowLoopback is like ValidatePublic but permits loopback
// addresses. Use this for clients that may legitimately target local
// servers (local LLM endpoints via Ollama, local MCP servers, local
// embedders, the CDP endpoint on localhost). All other dangerous ranges
// remain blocked.
func ValidateAllowLoopback(ip net.IP, displayHost string) error {
	if ip.IsLoopback() {
		return nil
	}
	return validateNonLoopback(ip, displayHost)
}

// ValidateAllowAll disables all IP-class checks. It is an escape hatch for
// environments where a public hostname (e.g. github.com) is intentionally
// resolved to a private address by split-horizon DNS, a corporate GitHub
// mirror, or a zero-trust gateway. Callers that use it MUST still enforce a
// hostname allow-list at a higher layer (as meta/version.go does via
// validateGitHubURL) so that only a fixed set of trusted hostnames may be
// contacted, regardless of the IP they resolve to.
func ValidateAllowAll(ip net.IP, displayHost string) error {
	return nil
}

// validateNonLoopback contains the checks shared by ValidatePublic and
// ValidateAllowLoopback: private, link-local, unspecified, multicast,
// and reserved ranges. Splitting it out keeps the loopback policy the
// only difference between the two exported validators.
func validateNonLoopback(ip net.IP, displayHost string) error {
	if ip.IsPrivate() {
		return fmt.Errorf("access to private address %s is not allowed", displayHost)
	}
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return fmt.Errorf("access to link-local address %s is not allowed", displayHost)
	}
	if ip.IsUnspecified() {
		return fmt.Errorf("access to unspecified address %s is not allowed", displayHost)
	}
	if ip.IsMulticast() {
		return fmt.Errorf("access to multicast address %s is not allowed", displayHost)
	}
	if IsReservedIP(ip) {
		return fmt.Errorf("access to reserved address %s is not allowed", displayHost)
	}
	return nil
}

// IsReservedIP reports whether ip falls in a reserved range that should
// not be reachable from production code (TEST-NET, CGNAT, 0.0.0.0/8,
// IPv6 ULA, etc.). It is exported so package-specific validators
// (e.g. nodes/core.ValidateIP) can compose it without re-implementing
// the range table.
func IsReservedIP(ip net.IP) bool {
	// Use To4() to handle both IPv4 and IPv4-mapped IPv6 addresses
	// uniformly (a v4-in-v6 address like ::ffff:192.0.2.1 must match the
	// TEST-NET-1 rule, not be treated as a fresh IPv6).
	ip4 := ip.To4()
	if ip4 == nil {
		// Pure IPv6 - block ULA (fc00::/7)
		if len(ip) == 16 && ip[0]&0xfe == 0xfc {
			return true
		}
		return false
	}

	// 0.0.0.0/8
	if ip4[0] == 0 {
		return true
	}
	// 169.254.0.0/16 (link-local; also caught by IsLinkLocalUnicast but
	// double-checked here so a future Go version relaxing that helper
	// can't silently widen our policy).
	if ip4[0] == 169 && ip4[1] == 254 {
		return true
	}
	// 192.0.2.0/24 (TEST-NET-1)
	if ip4[0] == 192 && ip4[1] == 0 && ip4[2] == 2 {
		return true
	}
	// 198.51.100.0/24 (TEST-NET-2)
	if ip4[0] == 198 && ip4[1] == 51 && ip4[2] == 100 {
		return true
	}
	// 203.0.113.0/24 (TEST-NET-3)
	if ip4[0] == 203 && ip4[1] == 0 && ip4[2] == 113 {
		return true
	}
	// 100.64.0.0/10 (CGNAT)
	if ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127 {
		return true
	}
	return false
}
