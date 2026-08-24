// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​​​​‌​‌​‌‌​‌​​​​‌​​‌​​‌​​​​​‌‌​​‌‌​‌‌‌​​‌​‌‌‌​​​​‌‌​‌‌​​​​​‌​​​‌‌​​​​​​​​​​​​​​​​​​​​​‌‌​​​‌‌‌‌​‌‌⁠
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
	"os"
)

// ExitError signals the command dispatcher to terminate the process with a
// specific exit code. Handlers return it instead of calling os.Exit so they
// stay testable and reusable from long-running hosts (serve, WebUI, SDK).
//
// By the time an ExitError is returned the handler has already printed
// whatever user-facing message is appropriate; the dispatcher only maps
// Code to the process exit status.
type ExitError struct {
	Code int
}

// Error implements the error interface.
func (e *ExitError) Error() string {
	return fmt.Sprintf("exit status %d", e.Code)
}

// exitErr returns an ExitError carrying the given exit code. Use it where the
// old code called os.Exit(code) after printing a message.
func exitErr(code int) error {
	return &ExitError{Code: code}
}

// ExitCode maps a handler error to a process exit code: nil → 0, *ExitError →
// its code, any other error → 1 (with the error printed, since a raw error
// means the handler failed without reporting to the user).
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var ee *ExitError
	if errors.As(err, &ee) {
		return ee.Code
	}
	fmt.Fprintf(os.Stderr, "❌ %v\n", err)
	return 1
}
