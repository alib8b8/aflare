// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​‌​​‌​‌‌‌​​​‌‌‌‌​‌‌‌​​‌‌​‌​‌‌​​​‌‌‌​​​‌​​‌​‌​​‌​‌‌‌​​​‌‌‌​​‌​​​‌‌‌‌‌​​​​​​​​​​​​​​​​​​​‌‌‌‌‌​​‌​​​‌‌‌​⁠
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

package nodes

import (
	"bufio"
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"fmt"
	"math/big"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeSMTPServer is a minimal SMTP dialogue server for tests. It speaks
// just enough ESMTP (EHLO, STARTTLS, AUTH PLAIN/LOGIN, MAIL, RCPT, DATA,
// QUIT) to exercise the email_send node end-to-end and records what the
// client sent.
type fakeSMTPServer struct {
	t           *testing.T
	ln          net.Listener
	cert        *tls.Certificate // non-nil => advertise STARTTLS / serve TLS
	implicitTLS bool             // wrap the listener in TLS (465 emulation)
	authMechs   string           // e.g. "PLAIN LOGIN"; "" => no AUTH ext

	mu           sync.Mutex
	authIdentity string // decoded AUTH PLAIN payload or "user/pass" for LOGIN
	mailFrom     string
	rcptTo       []string
	data         []byte
	starttlsUsed bool
}

// newFakeSMTPServer starts the fake server on 127.0.0.1:<random>.
func newFakeSMTPServer(t *testing.T, cert *tls.Certificate, implicitTLS bool, authMechs string) *fakeSMTPServer {
	t.Helper()
	var ln net.Listener
	var err error
	if implicitTLS {
		if cert == nil {
			t.Fatal("implicitTLS requires a cert")
		}
		ln, err = tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{*cert}})
	} else {
		ln, err = net.Listen("tcp", "127.0.0.1:0")
	}
	if err != nil {
		t.Fatalf("fake SMTP listen: %v", err)
	}
	s := &fakeSMTPServer{t: t, ln: ln, cert: cert, implicitTLS: implicitTLS, authMechs: authMechs}
	go s.acceptLoop()
	t.Cleanup(func() { _ = ln.Close() })
	return s
}

func (s *fakeSMTPServer) port() int {
	s.t.Helper()
	_, portStr, err := net.SplitHostPort(s.ln.Addr().String())
	if err != nil {
		s.t.Fatalf("split addr: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		s.t.Fatalf("parse port: %v", err)
	}
	return port
}

func (s *fakeSMTPServer) acceptLoop() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.handle(conn)
	}
}

func (s *fakeSMTPServer) handle(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	r := bufio.NewReader(conn)
	writeLine := func(line string) {
		if _, err := conn.Write([]byte(line + "\r\n")); err != nil {
			_ = conn.Close()
		}
	}
	writeLine("220 fake.local ESMTP")

	authStage := 0 // 0=none, 1=awaiting LOGIN username, 2=awaiting LOGIN password
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		cmd := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
			exts := "250-fake.local"
			if s.cert != nil && !s.implicitTLS {
				exts += "\r\n250-STARTTLS"
			}
			if s.authMechs != "" {
				exts += "\r\n250 AUTH " + s.authMechs
			} else {
				exts += "\r\n250 SIZE 10485760"
			}
			writeLine(exts)
		case cmd == "STARTTLS":
			if s.cert == nil {
				writeLine("454 TLS not available")
				continue
			}
			writeLine("220 Ready to start TLS")
			tlsConn := tls.Server(conn, &tls.Config{Certificates: []tls.Certificate{*s.cert}})
			if err := tlsConn.Handshake(); err != nil {
				return
			}
			conn = tlsConn
			r = bufio.NewReader(tlsConn)
			s.mu.Lock()
			s.starttlsUsed = true
			s.mu.Unlock()
		case strings.HasPrefix(cmd, "AUTH PLAIN"):
			// Initial-response form: "AUTH PLAIN <base64>".
			parts := strings.SplitN(line, " ", 3)
			payload := ""
			if len(parts) == 3 {
				payload = parts[2]
			} else {
				writeLine("334 ")
				resp, err := r.ReadString('\n')
				if err != nil {
					return
				}
				payload = strings.TrimSpace(resp)
			}
			decoded, err := decodeB64(payload)
			if err != nil {
				writeLine("535 auth failed")
				continue
			}
			s.mu.Lock()
			s.authIdentity = decoded
			s.mu.Unlock()
			writeLine("235 auth ok")
		case cmd == "AUTH LOGIN":
			authStage = 1
			writeLine("334 VXNlcm5hbWU6") // "Username:"
		case strings.HasPrefix(cmd, "MAIL FROM:"):
			s.mu.Lock()
			s.mailFrom = strings.Trim(strings.TrimPrefix(line, "MAIL FROM:"), " <>")
			s.mu.Unlock()
			writeLine("250 OK")
		case strings.HasPrefix(cmd, "RCPT TO:"):
			s.mu.Lock()
			s.rcptTo = append(s.rcptTo, strings.Trim(strings.TrimPrefix(line, "RCPT TO:"), " <>"))
			s.mu.Unlock()
			writeLine("250 OK")
		case cmd == "DATA":
			writeLine("354 End data with <CR><LF>.<CR><LF>")
			var buf bytes.Buffer
			for {
				l, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if strings.TrimRight(l, "\r\n") == "." {
					s.mu.Lock()
					s.data = buf.Bytes()
					s.mu.Unlock()
					writeLine("250 OK: queued")
					break
				}
				buf.WriteString(l)
			}
		case cmd == "QUIT":
			writeLine("221 Bye")
			return
		case cmd == "NOOP", cmd == "RSET":
			writeLine("250 OK")
		case authStage > 0:
			// LOGIN challenge/response continuation.
			decoded, err := decodeB64(strings.TrimSpace(line))
			if err != nil {
				writeLine("535 auth failed")
				authStage = 0
				continue
			}
			s.mu.Lock()
			switch authStage {
			case 1:
				authStage = 2
				writeLine("334 UGFzc3dvcmQ6") // "Password:"
			case 2:
				s.authIdentity = decoded
				authStage = 0
				writeLine("235 auth ok")
			}
			s.mu.Unlock()
		default:
			writeLine("502 Not implemented")
		}
	}
}

// decodeB64 decodes standard base64 (AUTH payloads).
func decodeB64(s string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

// selfSignedCert generates a throwaway self-signed certificate valid for
// 127.0.0.1/::1/localhost, used by the fake server's TLS handshakes.
func selfSignedCert(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "127.0.0.1"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
		DNSNames:     []string{"localhost"},
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	cert := tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(leaf)
	return cert, pool
}

// injectTLSRoots points the node's TLS verification at the fake server's
// CA for the duration of the test.
func injectTLSRoots(t *testing.T, pool *x509.CertPool) {
	t.Helper()
	old := emailTLSRootCAs
	emailTLSRootCAs = pool
	t.Cleanup(func() { emailTLSRootCAs = old })
}

// overrideLoopbackCheck forces the node's loopback classification of
// host for the duration of the test (used to exercise the non-loopback
// TLS policy against the loopback test listener).
func overrideLoopbackCheck(t *testing.T, host string, isLoopback bool) {
	t.Helper()
	old := emailIsLoopbackHost
	emailIsLoopbackHost = func(h string) bool {
		if h == host {
			return isLoopback
		}
		return old(h)
	}
	t.Cleanup(func() { emailIsLoopbackHost = old })
}

func emailParams(srv *fakeSMTPServer, extra map[string]string) map[string]string {
	params := map[string]string{
		"host": "127.0.0.1",
		"port": strconv.Itoa(srv.port()),
		"from": "aflare@example.com",
		"to":   "ops@example.com",
	}
	for k, v := range extra {
		params[k] = v
	}
	return params
}

func TestEmailSend_PlaintextLoopbackNoAuth(t *testing.T) {
	allowLoopback(t)
	srv := newFakeSMTPServer(t, nil, false, "")

	node := &EmailSendNode{}
	out, err := node.Execute(t.Context(), "line1\nline2", emailParams(srv, nil))
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}
	if !strings.Contains(out, "email sent via 127.0.0.1") || !strings.Contains(out, "ops@example.com") {
		t.Fatalf("unexpected output: %q", out)
	}

	srv.mu.Lock()
	defer srv.mu.Unlock()
	if srv.mailFrom != "aflare@example.com" {
		t.Errorf("MAIL FROM = %q", srv.mailFrom)
	}
	if len(srv.rcptTo) != 1 || srv.rcptTo[0] != "ops@example.com" {
		t.Errorf("RCPT TO = %v", srv.rcptTo)
	}
	data := string(srv.data)
	for _, want := range []string{
		"From: aflare@example.com\r\n",
		"To: ops@example.com\r\n",
		"Subject: aflare notification\r\n",
		"MIME-Version: 1.0\r\n",
		"Content-Type: text/plain; charset=utf-8\r\n",
		"\r\nline1\r\nline2",
	} {
		if !strings.Contains(data, want) {
			t.Errorf("message missing %q\ngot:\n%s", want, data)
		}
	}
	if srv.starttlsUsed {
		t.Error("STARTTLS unexpectedly used")
	}
}

func TestEmailSend_StartTLSAuthPlain(t *testing.T) {
	allowLoopback(t)
	cert, pool := selfSignedCert(t)
	injectTLSRoots(t, pool)
	srv := newFakeSMTPServer(t, &cert, false, "PLAIN LOGIN")

	node := &EmailSendNode{}
	_, err := node.Execute(t.Context(), "disk usage over threshold", emailParams(srv, map[string]string{
		"username": "alerts",
		"password": "s3cret",
		"subject":  "disk alert",
	}))
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}

	srv.mu.Lock()
	defer srv.mu.Unlock()
	if !srv.starttlsUsed {
		t.Error("expected STARTTLS upgrade")
	}
	// AUTH PLAIN payload is "\x00user\x00password".
	nul := string([]byte{0})
	if srv.authIdentity != nul+"alerts"+nul+"s3cret" {
		t.Errorf("AUTH PLAIN identity = %q", srv.authIdentity)
	}
	if !strings.Contains(string(srv.data), "Subject: disk alert\r\n") {
		t.Errorf("message missing subject; data:\n%s", srv.data)
	}
}

func TestEmailSend_AuthLoginFallback(t *testing.T) {
	allowLoopback(t)
	cert, pool := selfSignedCert(t)
	injectTLSRoots(t, pool)
	srv := newFakeSMTPServer(t, &cert, false, "LOGIN")

	node := &EmailSendNode{}
	_, err := node.Execute(t.Context(), "body", emailParams(srv, map[string]string{
		"username": "alerts",
		"password": "hunter2",
	}))
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}

	srv.mu.Lock()
	defer srv.mu.Unlock()
	// The fake server records the last LOGIN stage payload (password).
	if srv.authIdentity != "hunter2" {
		t.Errorf("AUTH LOGIN password = %q", srv.authIdentity)
	}
	if !srv.starttlsUsed {
		t.Error("expected STARTTLS before AUTH")
	}
}

func TestEmailSend_ImplicitTLS(t *testing.T) {
	allowLoopback(t)
	cert, pool := selfSignedCert(t)
	injectTLSRoots(t, pool)
	srv := newFakeSMTPServer(t, &cert, true, "PLAIN")

	node := &EmailSendNode{}
	_, err := node.Execute(t.Context(), "body", emailParams(srv, map[string]string{
		"tls_mode": "tls",
	}))
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}

	srv.mu.Lock()
	defer srv.mu.Unlock()
	if srv.starttlsUsed {
		t.Error("implicit TLS must not run the STARTTLS command")
	}
	if srv.mailFrom != "aflare@example.com" {
		t.Errorf("MAIL FROM = %q", srv.mailFrom)
	}
}

func TestEmailSend_StartTLSRequired(t *testing.T) {
	allowLoopback(t)
	srv := newFakeSMTPServer(t, nil, false, "") // no STARTTLS advertised

	node := &EmailSendNode{}
	_, err := node.Execute(t.Context(), "body", emailParams(srv, map[string]string{
		"tls_mode": "starttls",
	}))
	if err == nil || !strings.Contains(err.Error(), "STARTTLS") {
		t.Fatalf("expected STARTTLS-required error, got: %v", err)
	}
}

func TestEmailSend_PlaintextRefusedForNonLoopback(t *testing.T) {
	allowLoopback(t)
	srv := newFakeSMTPServer(t, nil, false, "")
	overrideLoopbackCheck(t, "127.0.0.1", false)

	node := &EmailSendNode{}
	_, err := node.Execute(t.Context(), "body", emailParams(srv, nil))
	if err == nil || !strings.Contains(err.Error(), "refusing plaintext") {
		t.Fatalf("expected refuse-plaintext error, got: %v", err)
	}
}

func TestEmailSend_SSRFBlockedWithoutLoopbackEnv(t *testing.T) {
	// Deliberately NOT calling allowLoopback: loopback dials must be
	// blocked by the dial-time IP validator.
	srv := newFakeSMTPServer(t, nil, false, "")

	node := &EmailSendNode{}
	_, err := node.Execute(t.Context(), "body", emailParams(srv, nil))
	if err == nil || !strings.Contains(err.Error(), "blocked") {
		t.Fatalf("expected SSRF block error, got: %v", err)
	}
}

func TestEmailSend_HeaderInjectionRejected(t *testing.T) {
	allowLoopback(t)
	srv := newFakeSMTPServer(t, nil, false, "")
	node := &EmailSendNode{}

	cases := map[string]map[string]string{
		"subject": {"subject": "ok\r\nBcc: attacker@evil.com"},
		"from":    {"from": "a@b.com\r\nBcc: attacker@evil.com"},
		"to":      {"to": "ops@example.com\nBcc: attacker@evil.com"},
		"cc":      {"cc": "x@y.com\r\nBcc: attacker@evil.com"},
	}
	for name, extra := range cases {
		_, err := node.Execute(t.Context(), "body", emailParams(srv, extra))
		if err == nil || !strings.Contains(err.Error(), "CR/LF") {
			t.Errorf("%s: expected header-injection rejection, got: %v", name, err)
		}
	}
}

func TestEmailSend_MissingParams(t *testing.T) {
	allowLoopback(t)
	srv := newFakeSMTPServer(t, nil, false, "")
	node := &EmailSendNode{}
	base := emailParams(srv, nil)

	if _, err := node.Execute(t.Context(), "b", map[string]string{"port": "25", "from": "a@b.c", "to": "x@y.z"}); err == nil || !strings.Contains(err.Error(), "host") {
		t.Errorf("missing host: got %v", err)
	}
	noFrom := map[string]string{"host": "127.0.0.1", "port": strconv.Itoa(srv.port()), "to": base["to"]}
	if _, err := node.Execute(t.Context(), "b", noFrom); err == nil || !strings.Contains(err.Error(), "from") {
		t.Errorf("missing from: got %v", err)
	}
	noTo := map[string]string{"host": "127.0.0.1", "port": strconv.Itoa(srv.port()), "from": base["from"]}
	if _, err := node.Execute(t.Context(), "b", noTo); err == nil || !strings.Contains(err.Error(), "to") {
		t.Errorf("missing to: got %v", err)
	}
}

func TestEmailSend_PasswordEnvResolution(t *testing.T) {
	allowLoopback(t)
	t.Setenv("AFLARE_TEST_SMTP_PASSWORD", "env-secret")
	cert, pool := selfSignedCert(t)
	injectTLSRoots(t, pool)
	srv := newFakeSMTPServer(t, &cert, false, "PLAIN")

	node := &EmailSendNode{}
	_, err := node.Execute(t.Context(), "body", emailParams(srv, map[string]string{
		"username":     "alerts",
		"password_env": "AFLARE_TEST_SMTP_PASSWORD",
	}))
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}

	srv.mu.Lock()
	defer srv.mu.Unlock()
	nul := string([]byte{0})
	if srv.authIdentity != nul+"alerts"+nul+"env-secret" {
		t.Errorf("AUTH PLAIN identity = %q", srv.authIdentity)
	}
}

func TestEmailSend_PasswordAndEnvConflict(t *testing.T) {
	node := &EmailSendNode{}
	_, err := node.Execute(t.Context(), "b", map[string]string{
		"host":         "smtp.example.com",
		"from":         "a@b.c",
		"to":           "x@y.z",
		"username":     "u",
		"password":     "inline",
		"password_env": "SOME_VAR",
	})
	if err == nil || !strings.Contains(err.Error(), "only one") {
		t.Fatalf("expected conflict error, got: %v", err)
	}
}

func TestEmailSend_UsernameWithoutPassword(t *testing.T) {
	node := &EmailSendNode{}
	_, err := node.Execute(t.Context(), "b", map[string]string{
		"host":     "smtp.example.com",
		"from":     "a@b.c",
		"to":       "x@y.z",
		"username": "u",
	})
	if err == nil || !strings.Contains(err.Error(), "password") {
		t.Fatalf("expected username-without-password error, got: %v", err)
	}
}

func TestEmailSend_NoAuthMechanismForUsername(t *testing.T) {
	allowLoopback(t)
	srv := newFakeSMTPServer(t, nil, false, "") // no AUTH extension

	node := &EmailSendNode{}
	_, err := node.Execute(t.Context(), "body", emailParams(srv, map[string]string{
		"username": "alerts",
		"password": "pw",
	}))
	if err == nil || !strings.Contains(err.Error(), "no AUTH") {
		t.Fatalf("expected no-AUTH error, got: %v", err)
	}
}

func TestEmailSend_InvalidAddresses(t *testing.T) {
	allowLoopback(t)
	srv := newFakeSMTPServer(t, nil, false, "")
	node := &EmailSendNode{}

	if _, err := node.Execute(t.Context(), "b", emailParams(srv, map[string]string{"to": "not-an-email"})); err == nil {
		t.Error("expected error for invalid to address")
	}
	if _, err := node.Execute(t.Context(), "b", emailParams(srv, map[string]string{"from": "no-at-sign"})); err == nil {
		t.Error("expected error for invalid from address")
	}
}

func TestEmailSend_TooManyRecipients(t *testing.T) {
	allowLoopback(t)
	srv := newFakeSMTPServer(t, nil, false, "")
	node := &EmailSendNode{}

	many := make([]string, maxSMTPRecipients+1)
	for i := range many {
		many[i] = fmt.Sprintf("user%d@example.com", i)
	}
	_, err := node.Execute(t.Context(), "b", emailParams(srv, map[string]string{"to": strings.Join(many, ",")}))
	if err == nil || !strings.Contains(err.Error(), "too many") {
		t.Fatalf("expected too-many-recipients error, got: %v", err)
	}
}

func TestEmailSend_BodyTooLarge(t *testing.T) {
	allowLoopback(t)
	srv := newFakeSMTPServer(t, nil, false, "")
	node := &EmailSendNode{}

	big := strings.Repeat("x", maxEmailBodySize+1)
	_, err := node.Execute(t.Context(), big, emailParams(srv, nil))
	if err == nil || !strings.Contains(err.Error(), "maximum size") {
		t.Fatalf("expected body-size error, got: %v", err)
	}
}

func TestEmailSend_PortAndTimeoutValidation(t *testing.T) {
	node := &EmailSendNode{}

	for _, port := range []string{"0", "70000", "abc"} {
		_, err := node.Execute(t.Context(), "b", map[string]string{
			"host": "smtp.example.com", "port": port, "from": "a@b.c", "to": "x@y.z",
		})
		if err == nil || !strings.Contains(err.Error(), "port") {
			t.Errorf("port %q: expected port error, got %v", port, err)
		}
	}
	if _, err := node.Execute(t.Context(), "b", map[string]string{
		"host": "smtp.example.com", "timeout": "0", "from": "a@b.c", "to": "x@y.z",
	}); err == nil || !strings.Contains(err.Error(), "timeout") {
		t.Errorf("expected timeout error, got %v", err)
	}
	if _, err := node.Execute(t.Context(), "b", map[string]string{
		"host": "smtp.example.com", "tls_mode": "bogus", "from": "a@b.c", "to": "x@y.z",
	}); err == nil || !strings.Contains(err.Error(), "tls_mode") {
		t.Errorf("expected tls_mode error, got %v", err)
	}
}

func TestEmailSend_NonASCIISubjectEncoded(t *testing.T) {
	allowLoopback(t)
	srv := newFakeSMTPServer(t, nil, false, "")

	node := &EmailSendNode{}
	_, err := node.Execute(t.Context(), "body", emailParams(srv, map[string]string{
		"subject": "磁盘告警",
	}))
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}

	srv.mu.Lock()
	defer srv.mu.Unlock()
	for _, line := range strings.Split(string(srv.data), "\r\n") {
		if strings.HasPrefix(line, "Subject: ") {
			if !strings.Contains(line, "=?utf-8") {
				t.Errorf("subject not Q-encoded: %q", line)
			}
			if strings.Contains(line, "磁盘") {
				t.Errorf("raw non-ASCII leaked into header: %q", line)
			}
			return
		}
	}
	t.Errorf("no Subject header found in data:\n%s", srv.data)
}

func TestEmailSend_DefaultEmailIsLoopbackHost(t *testing.T) {
	cases := map[string]bool{
		"localhost": true, "127.0.0.1": true, "::1": true, "[::1]": true,
		"smtp.gmail.com": false, "10.0.0.1": false, "": false,
	}
	for host, want := range cases {
		if got := defaultEmailIsLoopbackHost(host); got != want {
			t.Errorf("defaultEmailIsLoopbackHost(%q) = %v, want %v", host, got, want)
		}
	}
}

func TestEmailSend_ValidateSMTPHost(t *testing.T) {
	valid := []string{"smtp.example.com", "smtp.gmail.com", "127.0.0.1", "::1", "mail.local"}
	invalid := []string{"bad host", "smtp.example.com/../../etc", "a\r\nb", "", "..", "ex ample.com", "smtp..com"}

	for _, h := range valid {
		if err := validateSMTPHost(h); err != nil {
			t.Errorf("validateSMTPHost(%q) unexpected error: %v", h, err)
		}
	}
	for _, h := range invalid {
		if err := validateSMTPHost(h); err == nil {
			t.Errorf("validateSMTPHost(%q) expected error, got nil", h)
		}
	}
}

func TestEmailSend_CCHandle(t *testing.T) {
	allowLoopback(t)
	srv := newFakeSMTPServer(t, nil, false, "")

	node := &EmailSendNode{}
	_, err := node.Execute(t.Context(), "body", emailParams(srv, map[string]string{
		"to": "ops@example.com, dev@example.com",
		"cc": "audit@example.com",
	}))
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}

	srv.mu.Lock()
	defer srv.mu.Unlock()
	if len(srv.rcptTo) != 3 {
		t.Fatalf("expected 3 RCPT TO (to+cc), got %v", srv.rcptTo)
	}
	if !strings.Contains(string(srv.data), "Cc: audit@example.com\r\n") {
		t.Errorf("message missing Cc header; data:\n%s", srv.data)
	}
}
