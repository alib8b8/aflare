// Copyright (c) 2026 aflare Contributors
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

// agent_ops.go implements the opt-in ops endpoint for the agent daemon
// (`aflare agent`): a dedicated HTTP listener serving Prometheus /metrics
// and net/http/pprof debug endpoints. Everything is OFF by default — the
// local-first contract is that the daemon opens no ports unless the operator
// asks for it (and whoever enables it is responsible for access control).
//
// Gating uses the same env contract as the WebUI server:
//
//	AFLARE_METRICS=1  → GET /metrics        (Prometheus scrape)
//	AFLARE_PPROF=1    → GET /debug/pprof/*  (live profiling)
//
// The listener binds 127.0.0.1:<port> (override the address with
// AFLARE_OPS_ADDR, e.g. 0.0.0.0 in a container). Unlike the WebUI endpoints
// there is no auth middleware and no rate limiter — it is expected to be
// reached from localhost or a trusted private network only.

package cli

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"time"

	"github.com/alib8b8/aflare/internal/metrics"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// DefaultOpsPort is the daemon ops listener port used when --ops-port is not
// given.
const DefaultOpsPort = 9090

// defaultOpsAddr is the bind address for the ops listener. Localhost by
// default: the endpoints are unauthenticated, so they must not be exposed
// to the network unless the operator explicitly overrides AFLARE_OPS_ADDR.
const defaultOpsAddr = "127.0.0.1"

// opsEnabled reports whether any ops endpoint was requested via env.
func opsEnabled() bool {
	return os.Getenv("AFLARE_METRICS") == "1" || os.Getenv("AFLARE_PPROF") == "1"
}

// buildOpsHandler builds the daemon ops mux. Endpoints are gated
// independently by AFLARE_METRICS / AFLARE_PPROF, mirroring the WebUI
// server's env contract. With neither set the mux serves nothing (all
// paths 404).
func buildOpsHandler() http.Handler {
	mux := http.NewServeMux()

	if os.Getenv("AFLARE_METRICS") == "1" {
		metrics.Register()
		mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
			metrics.CollectSnapshot() // no-op without snapshot providers wired
			promhttp.Handler().ServeHTTP(w, r)
		})
	}

	if os.Getenv("AFLARE_PPROF") == "1" {
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	}

	return mux
}

// startOpsServer starts the daemon ops listener on the given port and
// returns the bound address ("" when disabled or on bind failure). The
// server is closed when ctx is cancelled. A bind failure is non-fatal —
// ops is auxiliary, so the daemon logs a warning and keeps running.
//
// port 0 binds an ephemeral port (used by tests).
func startOpsServer(ctx context.Context, port int) string {
	if !opsEnabled() {
		return ""
	}

	addr := os.Getenv("AFLARE_OPS_ADDR")
	if addr == "" {
		addr = defaultOpsAddr
	}

	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", addr, port),
		Handler:      buildOpsHandler(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second, // pprof profile?seconds=N can be long
	}

	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: ops endpoint disabled (listen %s: %v)\n", srv.Addr, err)
		return ""
	}

	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintf(os.Stderr, "Warning: ops endpoint stopped: %v\n", err)
		}
	}()
	go func() {
		<-ctx.Done()
		// Close (not Shutdown): /metrics and pprof are stateless, no drain
		// needed — determinism beats gracefulness here.
		_ = srv.Close()
	}()

	fmt.Printf("Ops endpoint listening on http://%s (metrics=%t pprof=%t, unauthenticated — trusted networks only)\n",
		ln.Addr().String(),
		os.Getenv("AFLARE_METRICS") == "1",
		os.Getenv("AFLARE_PPROF") == "1")
	return ln.Addr().String()
}
