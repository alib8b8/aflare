// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​‌‌​​​‌​‌‌‌​‌‌‌​‌​‌​​​​‌‌‌​‌​​‌‌‌‌​​‌​​‌‌‌‌​‌‌‌​​‌​‌​​‌​​​‌​‌‌​​‌​​​​​​​​​​​​​​​​​‌​‌​​‌‌‌​‌‌‌‌​​‌⁠
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
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/alib8b8/aflare/internal/logger"
	"github.com/alib8b8/aflare/internal/nodes"
	"github.com/alib8b8/aflare/internal/webhook"
)

// HandleWebhook handles the "webhook" command: starts the event-driven
// webhook trigger server in the foreground.
//
// Unlike `aflare serve` (whose /run endpoint executes arbitrary workflow
// definitions from the request body), the webhook server only triggers
// workflow files already present on disk by name — external callers can
// inject data (body → {{var.input}}, query params → vars) but never new
// logic. That makes it the correct entry point for untrusted event sources
// (GitHub/Gitea/Forgejo webhooks, alert callbacks, n8n/Make steps).
//
// Authentication when --secret (or AFLARE_WEBHOOK_SECRET) is configured:
//   - GitHub-style X-Hub-Signature-256 signature (also Gitea/Forgejo) —
//     configure the same secret in the repository webhook settings;
//     platform deliveries are deduplicated by delivery ID
//   - timestamped X-Hub-Signature-256 for own automation callers: send
//     X-Timestamp (unix seconds) and sign "<timestamp>." + body; the
//     request is rejected once the timestamp is more than 5 minutes
//     from server time (replay protection)
//   - plain X-Webhook-Secret header for trusted automation callers
//
// Without a secret the server binds 127.0.0.1 only; binding a non-loopback
// address without a secret is refused.
func HandleWebhook(args []string) error {
	host := ""
	port := ""
	secret := ""
	workflowsDir := ""

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--host", "-H":
			if i+1 < len(args) {
				host = args[i+1]
				i++
			}
		case "--port", "-p":
			if i+1 < len(args) {
				port = args[i+1]
				i++
			}
		case "--secret", "-s":
			if i+1 < len(args) {
				secret = args[i+1]
				i++
			}
		case "--dir", "-d":
			if i+1 < len(args) {
				workflowsDir = args[i+1]
				i++
			}
		case "--help", "-h":
			PrintWebhookUsage()
			return nil
		default:
			fmt.Printf("Unknown argument: %s\n", args[i])
			PrintWebhookUsage()
			return exitErr(1)
		}
	}

	// Env fallback keeps the secret off the command line (visible in ps).
	if secret == "" {
		secret = os.Getenv("AFLARE_WEBHOOK_SECRET")
	}

	// 安全检查：绑定非回环地址时必须有认证（与 serve 同款守卫）。
	if host != "" && host != "127.0.0.1" && host != "localhost" && secret == "" {
		fmt.Fprintln(os.Stderr,
			"Refusing to start: webhook server would bind to a non-loopback address "+
				"with no authentication. Use --secret <key> or set AFLARE_WEBHOOK_SECRET.")
		return exitErr(1)
	}

	reg := nodes.GetGlobalRegistry()
	// Load external nodes from ./nodes, same as PrepareWorkflow (non-fatal).
	if wd, err := os.Getwd(); err == nil {
		nodesDir := filepath.Join(wd, "nodes")
		if loadErr := reg.LoadExternalNodes(nodesDir); loadErr != nil {
			logger.Warn("failed to load external nodes", "dir", nodesDir, "error", loadErr)
		}
	}

	server := webhook.NewWebhookServer(port, secret, reg)
	if host != "" {
		server.SetHost(host)
	}
	if workflowsDir != "" {
		server.SetWorkflowsDir(workflowsDir)
	}

	if workflowsDir == "" {
		if wd, err := os.Getwd(); err == nil {
			workflowsDir = wd
		} else {
			workflowsDir = "."
		}
	}
	fmt.Printf("Starting webhook server on %s\n", server.Addr())
	fmt.Println("Workflows dir: " + workflowsDir)
	fmt.Println("Endpoints:")
	fmt.Println("  POST /webhook/{name}      - Trigger a workflow by name")
	fmt.Println("  GET  /webhook/status/{id} - Query task status (needs secret)")
	fmt.Println("  GET  /webhook/health      - Health check")
	if secret == "" {
		fmt.Println("Auth: none (loopback only)")
	} else {
		fmt.Println("Auth: X-Hub-Signature-256 + X-Timestamp (freshness, ±5min) or X-Webhook-Secret")
	}
	fmt.Println("Press Ctrl+C to stop")

	errCh := make(chan error, 1)
	go func() { errCh <- server.Start() }()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Printf("Webhook server error: %v\n", err)
			return exitErr(1)
		}
	case <-sigCh:
		fmt.Println("\n⏹  Stopping webhook server...")
		if err := server.Stop(); err != nil {
			fmt.Printf("Error during shutdown: %v\n", err)
			return exitErr(1)
		}
		fmt.Println("✅ Webhook server stopped.")
	}
	return nil
}

// PrintWebhookUsage prints usage information for the webhook command.
func PrintWebhookUsage() {
	fmt.Println("Usage: aflare webhook [options]")
	fmt.Println("\nStart the event-driven webhook trigger server.")
	fmt.Println("External callers trigger local workflow files by name — they can inject")
	fmt.Println("data (request body → {{var.input}}, query params → vars) but never define")
	fmt.Println("new workflow logic, so this is the entry point for untrusted sources")
	fmt.Println("(GitHub/Gitea/Forgejo webhooks, alert callbacks, n8n/Make).")
	fmt.Println("\nOptions:")
	fmt.Println("  --port, -p <port>     - Port to listen on (default: 8080)")
	fmt.Println("  --host, -H <host>     - Host to bind (default: 127.0.0.1 without a secret)")
	fmt.Println("  --secret, -s <key>    - Webhook secret (or set AFLARE_WEBHOOK_SECRET)")
	fmt.Println("  --dir, -d <dir>       - Directory with workflow files (default: cwd)")
	fmt.Println("  --help, -h            - Show this help message")
	fmt.Println("\nAuthentication (when a secret is configured):")
	fmt.Println("  GitHub/Gitea/Forgejo: configure the same secret in the repository webhook")
	fmt.Println("    settings; deliveries are accepted via X-Hub-Signature-256 signature and")
	fmt.Println("    deduplicated by delivery ID (replayed deliveries are rejected).")
	fmt.Println("  Own automation (curl/n8n/scripts): send X-Timestamp (unix seconds) and")
	fmt.Println("    X-Hub-Signature-256 = HMAC-SHA256(secret, \"<timestamp>.\" + body);")
	fmt.Println("    requests older than 5 minutes are rejected (replay protection).")
	fmt.Println("  Trusted callers: send the secret in the X-Webhook-Secret header.")
	fmt.Println("\nExamples:")
	fmt.Println("  aflare webhook")
	fmt.Println("  aflare webhook --port 9090 --secret $(cat ~/.aflare/webhook-secret)")
	fmt.Println("  AFLARE_WEBHOOK_SECRET=s3cret aflare webhook --host 0.0.0.0")
	fmt.Println("  curl -X POST -H 'X-Webhook-Secret: s3cret' \\")
	fmt.Println("       --data '{\"pr\": 123}' http://localhost:8080/webhook/pr-review-gate")
}
