// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌‌​​‌‌​​​‌​‌‌​‌​‌​​​​‌​‌‌‌​​​‌‌‌‌​‌​​​‌​​​​‌‌‌​‌​​​​​​​​​​​​​​​​​​‌‌‌‌‌‌​‌​‌​‌‌​⁠
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

package webui

import (
	"context"
	"crypto/subtle"
	"net/http"
	"net/http/pprof"
	"os"
	"sync"
	"time"

	"github.com/alib8b8/aflare/internal/agent"
	"github.com/alib8b8/aflare/internal/logger"
	"github.com/alib8b8/aflare/internal/metrics"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	defaultHost       = "127.0.0.1"
	defaultPort       = "8081"
	serverReadTimeout = 30 * time.Second
	// serverWriteTimeout is 0 (disabled) so SSE streaming connections
	// (/api/chat/stream) are not cut off mid-response. This is safe because:
	//   - the server binds to localhost by default
	//   - authMiddleware gates all endpoints when a token is set
	//   - agent SendMessageStream is bounded by DefaultSendTimeout (5m)
	serverWriteTimeout    = 0
	serverShutdownTimeout = 10 * time.Second
	maxWorkflowFileSize   = 5 * 1024 * 1024 // 5MB
	// metricsRPS caps the number of /metrics scrapes per second. The endpoint
	// is unauthenticated, so a small token bucket guards against floods.
	metricsRPS = 5
)

type WebUIServer struct {
	host         string
	port         string
	workflowsDir string
	authToken    string
	sessions     *agent.SessionManager

	mu     sync.RWMutex
	server *http.Server
	stopCh chan struct{}
}

func NewWebUIServer(host, port string) *WebUIServer {
	if host == "" {
		host = defaultHost
	}
	if port == "" {
		port = defaultPort
	}
	return &WebUIServer{
		host:     host,
		port:     port,
		stopCh:   make(chan struct{}),
		sessions: agent.NewSessionManager(agent.DefaultMaxSessions, agent.DefaultSessionTTL),
	}
}

// SetAuthToken enables token-based authentication for the WebUI.
// When set, all API requests must include an "X-Auth-Token" header.
func (s *WebUIServer) SetAuthToken(token string) {
	s.authToken = token
}

func (s *WebUIServer) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.authToken != "" {
			token := r.Header.Get("X-Auth-Token")
			if subtle.ConstantTimeCompare([]byte(token), []byte(s.authToken)) != 1 {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		next(w, r)
	}
}

func (s *WebUIServer) SetWorkflowsDir(dir string) {
	s.workflowsDir = dir
}

// SetCapabilities sets the capability names to enable for new chat sessions.
func (s *WebUIServer) SetCapabilities(caps []string) {
	s.sessions.SetCapabilities(caps)
}

func (s *WebUIServer) Start() error {
	srv := &http.Server{
		Addr:         s.host + ":" + s.port,
		Handler:      s.buildHandler(),
		ReadTimeout:  serverReadTimeout,
		WriteTimeout: serverWriteTimeout,
	}

	s.mu.Lock()
	s.server = srv
	s.mu.Unlock()

	logger.Info("WebUI server started", "port", s.port)
	return srv.ListenAndServe()
}

// buildHandler builds and returns the HTTP handler with all routes registered,
// including the optional pprof and /metrics endpoints gated by their
// respective environment variables. Extracted from Start so tests can drive
// the handler via httptest without binding a real listener.
func (s *WebUIServer) buildHandler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/visualize", s.authMiddleware(s.handleVisualize))
	mux.HandleFunc("/api/workflows", s.authMiddleware(s.handleListWorkflows))
	mux.HandleFunc("/api/workflow", s.authMiddleware(s.handleWorkflow))
	mux.HandleFunc("/api/validate", s.authMiddleware(s.handleValidate))
	mux.HandleFunc("/api/chat", s.authMiddleware(s.handleChat))
	mux.HandleFunc("/api/chat/stream", s.authMiddleware(s.handleChatStream))

	// pprof 调试端点:默认关闭,仅当环境变量 AFLARE_PPROF=1 时启用。
	// 生产环境保持关闭以避免安全暴露;需要在线性能剖析时显式开启。
	// 端点受 authMiddleware 保护,访问需带 X-Auth-Token(若设置了 token)。
	if os.Getenv("AFLARE_PPROF") == "1" {
		s.registerPprof(mux)
		logger.Info("pprof endpoints enabled at /debug/pprof/ (AFLARE_PPROF=1)")
	}

	// Prometheus /metrics endpoint: disabled by default for security (the
	// endpoint exposes internal statistics and is unauthenticated so scrapers
	// do not need to carry a token). Enable by setting AFLARE_METRICS=1.
	// The handler is rate-limited via a token bucket (metricsRPS req/s) and
	// is NOT behind authMiddleware, matching the Prometheus scrape convention.
	if os.Getenv("AFLARE_METRICS") == "1" {
		registerMetricsProviders()
		metrics.Register()
		metrics.RegisterAnalytics()
		limiter := newMetricsRateLimiter(metricsRPS)
		mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
			if !limiter.allow() {
				http.Error(w, "too many requests", http.StatusTooManyRequests)
				return
			}
			metrics.CollectSnapshot()
			promhttp.Handler().ServeHTTP(w, r)
		})
		logger.Info("prometheus /metrics endpoint enabled (AFLARE_METRICS=1)")
	}

	return mux
}

// registerPprof 在 mux 上注册 net/http/pprof 调试端点。
// 所有端点经 authMiddleware 保护(若设置了 authToken 则需带 X-Auth-Token)。
func (s *WebUIServer) registerPprof(mux *http.ServeMux) {
	mux.HandleFunc("/debug/pprof/", s.authMiddleware(pprof.Index))
	mux.HandleFunc("/debug/pprof/cmdline", s.authMiddleware(pprof.Cmdline))
	mux.HandleFunc("/debug/pprof/profile", s.authMiddleware(pprof.Profile))
	mux.HandleFunc("/debug/pprof/symbol", s.authMiddleware(pprof.Symbol))
	mux.HandleFunc("/debug/pprof/trace", s.authMiddleware(pprof.Trace))
}

func (s *WebUIServer) Stop() error {
	s.mu.RLock()
	srv := s.server
	s.mu.RUnlock()
	if srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), serverShutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			return err
		}
	}
	logger.Info("WebUI server stopped")
	return nil
}
