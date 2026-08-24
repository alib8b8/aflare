// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌‌​‌​​​‌‌‌​​‌​​‌​‌‌​‌​‌‌‌‌‌​​‌​​‌‌​​‌​‌​‌​‌​​‌‌‌​​​​​​​​​​​​​​​​‌‌​‌​‌‌​‌​‌‌​​​‌⁠
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

package httpclient

import (
	"net"
	"testing"
	"time"
)

func TestNewClient_PanicsOnMisconfiguration(t *testing.T) {
	t.Run("zero timeout", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected panic for zero Timeout")
			}
		}()
		_ = NewClient(Options{Timeout: 0, Validator: ValidatePublic})
	})
	t.Run("nil validator", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected panic for nil Validator")
			}
		}()
		_ = NewClient(Options{Timeout: time.Second, Validator: nil})
	})
}

func TestNewClient_AppliesTimeoutAndTransport(t *testing.T) {
	c := NewClient(Options{Timeout: 7 * time.Second, Validator: ValidatePublic})
	if c.Timeout != 7*time.Second {
		t.Fatalf("Timeout = %v, want 7s", c.Timeout)
	}
	if c.Transport == nil {
		t.Fatal("Transport must be set so the SSRF dial applies")
	}
}

func TestValidatePublic_RejectsLoopback(t *testing.T) {
	for _, ip := range []string{"127.0.0.1", "::1"} {
		if err := ValidatePublic(net.ParseIP(ip), "h"); err == nil {
			t.Errorf("ValidatePublic(%s) = nil, want error", ip)
		}
	}
}

func TestValidateAllowLoopback_AllowsLoopback(t *testing.T) {
	for _, ip := range []string{"127.0.0.1", "::1"} {
		if err := ValidateAllowLoopback(net.ParseIP(ip), "h"); err != nil {
			t.Errorf("ValidateAllowLoopback(%s) = %v, want nil", ip, err)
		}
	}
}

func TestBothValidators_RejectDangerousRanges(t *testing.T) {
	// Private, link-local, unspecified, multicast, and reserved ranges
	// must be rejected regardless of loopback policy.
	dangerous := []string{
		"10.0.0.1",     // private RFC1918
		"172.16.0.1",   // private RFC1918
		"192.168.1.1",  // private RFC1918
		"169.254.1.1",  // link-local
		"0.0.0.0",      // unspecified
		"224.0.0.1",    // multicast
		"192.0.2.1",    // TEST-NET-1 reserved
		"198.51.100.1", // TEST-NET-2 reserved
		"203.0.113.1",  // TEST-NET-3 reserved
		"100.64.0.1",   // CGNAT reserved
		"fc00::1",      // IPv6 ULA reserved
	}
	for _, v := range []Validator{ValidatePublic, ValidateAllowLoopback} {
		for _, s := range dangerous {
			if err := v(net.ParseIP(s), "h"); err == nil {
				t.Errorf("validator rejected nil for dangerous IP %s", s)
			}
		}
	}
}

func TestBothValidators_AllowPublic(t *testing.T) {
	public := []string{"8.8.8.8", "1.1.1.1", "2606:4700:4700::1111"}
	for _, v := range []Validator{ValidatePublic, ValidateAllowLoopback} {
		for _, s := range public {
			if err := v(net.ParseIP(s), "h"); err != nil {
				t.Errorf("validator returned %v for public IP %s", err, s)
			}
		}
	}
}

func TestIsReservedIP(t *testing.T) {
	cases := []struct {
		ip   string
		want bool
	}{
		{"0.0.0.0", true},
		{"169.254.1.1", true},
		{"192.0.2.1", true},
		{"198.51.100.1", true},
		{"203.0.113.1", true},
		{"100.64.0.1", true},
		{"100.127.0.1", true},
		{"100.63.0.1", false},
		{"100.128.0.1", false},
		{"fc00::1", true},
		{"8.8.8.8", false},
		// IPv4-mapped IPv6 must be treated as the v4 address.
		{"::ffff:192.0.2.1", true},
		{"::ffff:8.8.8.8", false},
	}
	for _, c := range cases {
		ip := net.ParseIP(c.ip)
		if ip == nil {
			t.Fatalf("ParseIP(%q) returned nil", c.ip)
		}
		if got := IsReservedIP(ip); got != c.want {
			t.Errorf("IsReservedIP(%s) = %v, want %v", c.ip, got, c.want)
		}
	}
}
