// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​‌​​‌​‌‌‌​​​‌‌‌‌​‌‌‌​​​‌​‌​‌​‌‌​‌​​​​​‌​​​‌‌‌‌​​​‌​‌‌​​‌​​‌​​​‌​​​​‌‌​​​​​​​​​​​​​​​​​‌‌‌​​‌​‌‌‌​​​​‌‌⁠
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
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"os"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// EmailSendNode sends an email over SMTP — the channel most alerting and
// reporting workflows still need when chat-webhook channels are
// unavailable (enterprise mail relays, pager systems, archival).
//
// Security model (mirrors the http_request / notify nodes):
//
//   - Credentials: the SMTP password never belongs in a workflow file.
//     Prefer password_env (an environment variable name); the inline
//     password param exists for parity with other nodes but no code path
//     ever logs it.
//   - TLS: port 465 dials implicit TLS, every other port upgrades via
//     STARTTLS. Plaintext SMTP is only permitted for loopback relays
//     (and loopback dials only pass the IP validator when
//     AFLARE_ALLOW_LOOPBACK=1, i.e. local dev/demo mode).
//   - SSRF: the connected IP is validated in the dialer Control hook —
//     the same policy as the HTTP nodes (private/reserved ranges blocked
//     unless loopback mode is on) — which closes the DNS-rebinding TOCTOU
//     gap a resolve-then-dial check would leave open.
//   - Header injection: from/to/cc/subject are parsed with net/mail and
//     reject CR/LF, so a crafted subject cannot smuggle extra headers
//     into the DATA stream.
type EmailSendNode struct{}

func init() {
	Register(&EmailSendNode{})
}

const (
	defaultSMTPPort    = 587
	implicitTLSSMTPort = 465
	maxSMTPRecipients  = 50
	maxEmailBodySize   = 100 * 1024 // matches the notify node's payload cap
)

var (
	defaultEmailTimeout = 30 * time.Second
	maxEmailTimeout     = 120 * time.Second

	// emailTLSRootCAs, when non-nil, overrides the system root pool for
	// STARTTLS / implicit-TLS handshakes. Tests inject a pool holding the
	// fake server's certificate; production leaves it nil (system roots).
	emailTLSRootCAs *x509.CertPool

	// emailIsLoopbackHost reports whether host points at the machine
	// itself. Overridable in tests to exercise the non-loopback TLS
	// policy without a second network interface.
	emailIsLoopbackHost = defaultEmailIsLoopbackHost
)

// defaultEmailIsLoopbackHost is the production loopback check: literal
// loopback IPs (v4/v6) or the "localhost" name.
func defaultEmailIsLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	if ip := net.ParseIP(strings.Trim(host, "[]")); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

func (n *EmailSendNode) Name() string { return "email_send" }

func (n *EmailSendNode) Description() string {
	return "Send an email over SMTP (implicit TLS on port 465, STARTTLS elsewhere; plaintext only for loopback relays)"
}

func (n *EmailSendNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "email_send",
		Description: "Send an email over SMTP (implicit TLS on port 465, STARTTLS elsewhere; plaintext only for loopback relays)",
		Input:       "string - email body (used when the body param is empty)",
		Output:      "string - delivery summary (host, recipients, subject)",
		Params: []ParamSchema{
			{Name: "host", Type: "string", Description: "SMTP server hostname or IP (e.g. smtp.gmail.com)", Required: true},
			{Name: "port", Type: "int", Description: "SMTP port (465=implicit TLS, 587=STARTTLS; default 587)", Required: false, Default: "587"},
			{Name: "from", Type: "string", Description: "Sender address (plain email; display names are stripped)", Required: true},
			{Name: "to", Type: "string", Description: "Comma-separated recipient addresses (max 50 incl. cc)", Required: true},
			{Name: "cc", Type: "string", Description: "Comma-separated CC addresses (optional)", Required: false},
			{Name: "subject", Type: "string", Description: "Subject line (default 'aflare notification')", Required: false, Default: "aflare notification"},
			{Name: "body", Type: "string", Description: "Email body (overrides the node input)", Required: false},
			{Name: "username", Type: "string", Description: "SMTP AUTH username (omit for unauthenticated local relays)", Required: false},
			{Name: "password", Type: "string", Description: "SMTP password. Prefer password_env so secrets stay out of workflow files", Required: false},
			{Name: "password_env", Type: "string", Description: "Environment variable holding the SMTP password (e.g. AFLARE_SMTP_PASSWORD)", Required: false},
			{Name: "tls_mode", Type: "string", Description: "TLS strategy: auto (465=implicit TLS, else STARTTLS), starttls (always require STARTTLS), tls (always implicit TLS)", Required: false, Default: "auto"},
			{Name: "timeout", Type: "int", Description: "Dial+dialogue timeout in seconds (default 30, max 120)", Required: false, Default: "30"},
		},
	}
}

func (n *EmailSendNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	host := strings.TrimSpace(params["host"])
	if host == "" {
		return "", fmt.Errorf("host parameter is required")
	}
	if err := validateSMTPHost(host); err != nil {
		return "", err
	}

	port := defaultSMTPPort
	if raw := params["port"]; raw != "" {
		p, err := strconv.Atoi(raw)
		if err != nil || p < 1 || p > 65535 {
			return "", fmt.Errorf("invalid port %q (must be 1-65535)", raw)
		}
		port = p
	}

	tlsMode := strings.ToLower(strings.TrimSpace(params["tls_mode"]))
	if tlsMode == "" {
		tlsMode = "auto"
	}
	implicitTLS := port == implicitTLSSMTPort
	requireStartTLS := false
	switch tlsMode {
	case "auto":
	case "starttls":
		implicitTLS = false
		requireStartTLS = true
	case "tls":
		implicitTLS = true
	default:
		return "", fmt.Errorf("invalid tls_mode %q (supported: auto, starttls, tls)", params["tls_mode"])
	}

	fromAddrs, err := parseEmailAddresses(params["from"], 1)
	if err != nil {
		return "", fmt.Errorf("from: %w", err)
	}
	from := fromAddrs[0]

	toAddrs, err := parseEmailAddresses(params["to"], maxSMTPRecipients)
	if err != nil {
		return "", fmt.Errorf("to: %w", err)
	}
	var ccAddrs []string
	if raw := params["cc"]; raw != "" {
		ccAddrs, err = parseEmailAddresses(raw, maxSMTPRecipients-len(toAddrs))
		if err != nil {
			return "", fmt.Errorf("cc: %w", err)
		}
	}

	subject := params["subject"]
	if subject == "" {
		subject = "aflare notification"
	}
	if strings.ContainsAny(subject, "\r\n") {
		return "", fmt.Errorf("subject contains CR/LF (header injection rejected)")
	}

	body := params["body"]
	if body == "" {
		body = input
	}
	if len(body) > maxEmailBodySize {
		return "", fmt.Errorf("email body exceeds maximum size of %d bytes", maxEmailBodySize)
	}

	username := params["username"]
	password := params["password"]
	if envName := params["password_env"]; envName != "" {
		if password != "" {
			return "", fmt.Errorf("set only one of password / password_env")
		}
		password = os.Getenv(envName)
	}
	if username != "" && password == "" {
		return "", fmt.Errorf("username requires a password (set password or password_env)")
	}

	timeout := defaultEmailTimeout
	if raw := params["timeout"]; raw != "" {
		secs, err := strconv.Atoi(raw)
		if err != nil || secs < 1 {
			return "", fmt.Errorf("invalid timeout %q (seconds)", raw)
		}
		timeout = time.Duration(secs) * time.Second
		if timeout > maxEmailTimeout {
			timeout = maxEmailTimeout
		}
	}

	msg, err := buildEmailMessage(from, toAddrs, ccAddrs, subject, body)
	if err != nil {
		return "", err
	}

	recipients := append(slices.Clone(toAddrs), ccAddrs...)
	if err := deliverEmail(ctx, host, port, implicitTLS, requireStartTLS, username, password, from, recipients, msg, timeout); err != nil {
		return "", err
	}

	summary := fmt.Sprintf("email sent via %s:%d to %s (subject %q)", host, port, strings.Join(recipients, ", "), subject)
	fmt.Printf("[email_send] %s\n", summary)
	return summary, nil
}

// validateSMTPHost performs the pre-dial format checks on the SMTP host:
// plausible hostname or IP literal, no whitespace/CR/LF/slashes. IP-range
// policy (loopback/private/reserved) is enforced at dial time in the
// dialer Control hook so DNS rebinding cannot race the check.
func validateSMTPHost(host string) error {
	if strings.ContainsAny(host, " \t\r\n/\\") {
		return fmt.Errorf("invalid SMTP host %q", host)
	}
	trimmed := strings.Trim(host, "[]")
	if trimmed == "" {
		return fmt.Errorf("invalid SMTP host %q", host)
	}
	if net.ParseIP(trimmed) != nil {
		return nil // literal IP; range policy enforced at dial time
	}
	for _, label := range strings.Split(trimmed, ".") {
		if !isHostnameLabel(label) {
			return fmt.Errorf("invalid SMTP host %q", host)
		}
	}
	return nil
}

// isHostnameLabel reports whether label is a valid RFC 1123 hostname
// label: letters/digits/hyphens, hyphen not leading/trailing.
func isHostnameLabel(label string) bool {
	if label == "" {
		return false
	}
	for i, r := range label {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '-':
			if i == 0 || i == len(label)-1 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// parseEmailAddresses parses a comma-separated address list into bare
// addr-specs. CR/LF is rejected up front (header injection) and anything
// net/mail cannot parse fails; display names are accepted on input but
// stripped, keeping the generated headers trivially well-formed.
func parseEmailAddresses(raw string, max int) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("address list is empty")
	}
	if strings.ContainsAny(raw, "\r\n") {
		return nil, fmt.Errorf("address %q contains CR/LF (header injection rejected)", raw)
	}
	list, err := mail.ParseAddressList(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid address list %q: %w", raw, err)
	}
	if len(list) > max {
		return nil, fmt.Errorf("too many addresses (%d > %d)", len(list), max)
	}
	addrs := make([]string, 0, len(list))
	for _, a := range list {
		addrs = append(addrs, a.Address)
	}
	return addrs, nil
}

// buildEmailMessage assembles the RFC 5322 message: single-line headers,
// Q-encoded subject (non-ASCII safe), and a CRLF-normalized body.
func buildEmailMessage(from string, to, cc []string, subject, body string) ([]byte, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", strings.Join(to, ", "))
	if len(cc) > 0 {
		fmt.Fprintf(&b, "Cc: %s\r\n", strings.Join(cc, ", "))
	}
	fmt.Fprintf(&b, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", subject))
	fmt.Fprintf(&b, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	b.WriteString("\r\n")
	// SMTP DATA requires CRLF line endings; normalize bare LF and CR.
	normalized := strings.ReplaceAll(body, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	normalized = strings.ReplaceAll(normalized, "\n", "\r\n")
	b.WriteString(normalized)
	return []byte(b.String()), nil
}

// smtpDialer returns a dialer whose Control hook validates the actually
// connected IP with the same policy as the HTTP nodes' safe transport
// (loopback allowed only in AFLARE_ALLOW_LOOPBACK mode). Validating at
// dial time closes the DNS-rebinding TOCTOU gap.
func smtpDialer(timeout time.Duration) *net.Dialer {
	return &net.Dialer{
		Timeout: timeout,
		Control: validateSMTPDialAddress,
	}
}

// validateSMTPDialAddress is the net.Dialer Control hook for SMTP
// connections: validate the resolved IP before the socket is usable.
func validateSMTPDialAddress(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("smtp dial: bad address %q", address)
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return fmt.Errorf("smtp dial: non-IP address %q", address)
	}
	validator := validateIP
	if loopbackAllowed() {
		validator = validateLMLEndpointIP
	}
	if err := validator(ip, host); err != nil {
		return fmt.Errorf("smtp dial %s blocked: %w", address, err)
	}
	return nil
}

// emailTLSConfig builds the client TLS config for STARTTLS / implicit
// TLS: SNI from the host, TLS 1.2 minimum, system roots unless tests
// injected a pool via emailTLSRootCAs.
func emailTLSConfig(host string) *tls.Config {
	conf := &tls.Config{
		ServerName: host,
		MinVersion: tls.VersionTLS12,
	}
	if emailTLSRootCAs != nil {
		conf.RootCAs = emailTLSRootCAs
	}
	return conf
}

// deliverEmail runs one SMTP dialogue: dial (implicit TLS or plain),
// optional STARTTLS upgrade, optional AUTH, then MAIL FROM / RCPT TO /
// DATA / QUIT.
func deliverEmail(ctx context.Context, host string, port int, implicitTLS, requireStartTLS bool, username, password, from string, recipients []string, msg []byte, timeout time.Duration) error {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	deadline := time.Now().Add(timeout)

	var conn net.Conn
	var err error
	if implicitTLS {
		d := &tls.Dialer{NetDialer: smtpDialer(timeout), Config: emailTLSConfig(host)}
		conn, err = d.DialContext(ctx, "tcp", addr) // codeql[go/insecure-randomness] -- ServerName is a format-validated hostname (validateSMTPHost); all TLS randomness (keys, client hello) stays inside crypto/tls, never derived from workflow data
	} else {
		conn, err = smtpDialer(timeout).DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("smtp dial %s failed: %w", addr, err)
	}
	defer conn.Close()

	if err := conn.SetDeadline(deadline); err != nil {
		return fmt.Errorf("smtp deadline failed: %w", err)
	}

	cl, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("smtp handshake with %s failed: %w", addr, err)
	}
	defer cl.Close() //nolint:errcheck -- best-effort; QUIT path closes cleanly

	if !implicitTLS {
		if hasSTARTTLS, _ := cl.Extension("STARTTLS"); hasSTARTTLS {
			if err := cl.StartTLS(emailTLSConfig(host)); err != nil {
				return fmt.Errorf("STARTTLS with %s failed: %w", addr, err)
			}
		} else if requireStartTLS || !emailIsLoopbackHost(host) {
			return fmt.Errorf("SMTP server %s does not support STARTTLS; refusing plaintext SMTP to a non-loopback host (use port 465 or tls_mode=tls)", addr)
		}
	}

	if username != "" {
		if err := smtpAuthenticate(cl, host, username, password); err != nil {
			return err
		}
	}

	if err := cl.Mail(from); err != nil {
		return fmt.Errorf("MAIL FROM rejected: %w", err)
	}
	for _, rcpt := range recipients {
		if err := cl.Rcpt(rcpt); err != nil {
			return fmt.Errorf("RCPT TO %s rejected: %w", rcpt, err)
		}
	}
	w, err := cl.Data()
	if err != nil {
		return fmt.Errorf("DATA failed: %w", err)
	}
	if _, err := w.Write(msg); err != nil { // codeql[go/email-injection] -- the body is workflow output by design; headers are net/mail-parsed with CR/LF rejected (parseEmailAddresses), and this writer is smtp.Data()'s dot-stuffer, so body content can neither forge headers nor terminate the DATA phase early
		_ = w.Close()
		return fmt.Errorf("writing message body failed: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("finishing message failed: %w", err)
	}
	return cl.Quit()
}

// smtpAuthenticate authenticates via AUTH PLAIN when advertised (the
// near-universal mechanism) and falls back to AUTH LOGIN for servers
// that only offer it (some Exchange / older providers).
func smtpAuthenticate(cl *smtp.Client, host, username, password string) error {
	ok, ext := cl.Extension("AUTH")
	if !ok {
		return fmt.Errorf("SMTP server %s offers no AUTH mechanisms but a username was configured", host)
	}
	mechs := strings.Fields(strings.ToUpper(ext))
	if slices.Contains(mechs, "PLAIN") {
		if err := cl.Auth(smtp.PlainAuth("", username, password, host)); err != nil {
			return fmt.Errorf("AUTH PLAIN failed: %w", err)
		}
		return nil
	}
	if slices.Contains(mechs, "LOGIN") {
		if err := cl.Auth(&smtpLoginAuth{username: username, password: password}); err != nil {
			return fmt.Errorf("AUTH LOGIN failed: %w", err)
		}
		return nil
	}
	return fmt.Errorf("SMTP server %s does not advertise AUTH PLAIN or LOGIN (got %q)", host, ext)
}

// smtpLoginAuth implements AUTH LOGIN (RFC 4616-style challenge/response:
// "Username:" then "Password:" prompts), which net/smtp does not ship.
type smtpLoginAuth struct {
	username, password string
}

func (a *smtpLoginAuth) Start(_ *smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", nil, nil
}

func (a *smtpLoginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}
	switch strings.ToLower(strings.TrimSpace(string(fromServer))) {
	case "username:":
		return []byte(a.username), nil
	case "password:":
		return []byte(a.password), nil
	default:
		return nil, fmt.Errorf("smtp LOGIN auth: unexpected server challenge %q", fromServer)
	}
}
