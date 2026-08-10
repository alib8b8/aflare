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

package workflow

import (
	"sync/atomic"
)

// shutdownFlag is set to 1 when a graceful shutdown has been signalled.
// Workflow step loops check this flag between steps to avoid starting new
// work after a shutdown request, allowing the current step to complete and
// deferred cleanup (WAL close → flush, audit finalization) to run.
var shutdownFlag int32

// IsShuttingDown reports whether a graceful shutdown has been requested.
// Callers should stop starting new work and return as soon as practical,
// letting deferred WAL flush and audit finalization run.
func IsShuttingDown() bool {
	return atomic.LoadInt32(&shutdownFlag) == 1
}

// SignalShutdown marks the shutdown flag. It is called by the API server
// (on SIGINT/SIGTERM) and by the Executor's own signal handler when running
// standalone (CLI mode). It is idempotent — subsequent calls are no-ops.
func SignalShutdown() {
	atomic.StoreInt32(&shutdownFlag, 1)
}