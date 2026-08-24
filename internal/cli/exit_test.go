// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​​​​‌​‌​‌‌​‌​​​​‌​​​​​‌​‌‌‌​‌​‌‌​​​​‌​‌​‌‌‌‌‌​​‌‌​‌‌​​​​​‌‌​‌‌​​​​​​​​​​​​​​​​​​​​​‌​‌‌‌‌‌​​​​‌‌‌​⁠
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
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
	"testing"
)

func TestExitCode_Nil(t *testing.T) {
	if code := ExitCode(nil); code != 0 {
		t.Errorf("ExitCode(nil) = %d, want 0", code)
	}
}

func TestExitCode_FromExitError(t *testing.T) {
	err := &ExitError{Code: 3}
	if code := ExitCode(err); code != 3 {
		t.Errorf("ExitCode(&ExitError{Code: 3}) = %d, want 3", code)
	}
	if msg := err.Error(); msg != "exit status 3" {
		t.Errorf("ExitError.Error() = %q, want %q", msg, "exit status 3")
	}
}

// TestExitCode_FromGenericError covers the fallback: any non-ExitError error
// maps to exit code 1 (with the error printed to stderr by ExitCode itself).
func TestExitCode_FromGenericError(t *testing.T) {
	if code := ExitCode(fmt.Errorf("boom")); code != 1 {
		t.Errorf("ExitCode(generic error) = %d, want 1", code)
	}
}

// TestExitCode_WrappedExitError covers errors.As unwrapping: an ExitError
// wrapped via %w still reports its own code.
func TestExitCode_WrappedExitError(t *testing.T) {
	wrapped := fmt.Errorf("dispatch failed: %w", exitErr(2))
	if code := ExitCode(wrapped); code != 2 {
		t.Errorf("ExitCode(wrapped ExitError) = %d, want 2", code)
	}
}

// TestExitCode_ExitErrHelper covers the exitErr constructor that replaced the
// old direct os.Exit calls: it returns a *ExitError carrying the code.
func TestExitCode_ExitErrHelper(t *testing.T) {
	err := exitErr(2)
	if err == nil {
		t.Fatal("exitErr(2) returned nil")
	}
	var ee *ExitError
	if !errors.As(err, &ee) {
		t.Fatalf("exitErr(2) returned %T, want *ExitError", err)
	}
	if ee.Code != 2 {
		t.Errorf("exitErr(2).Code = %d, want 2", ee.Code)
	}
	if code := ExitCode(err); code != 2 {
		t.Errorf("ExitCode(exitErr(2)) = %d, want 2", code)
	}
}
