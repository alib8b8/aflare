// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​‌‌​‌​‌​‌​‌‌‌‌​​​‌​‌​‌​​‌​‌​‌‌​​​‌​​‌‌‌​​​‌‌​‌​​‌‌‌​​‌‌​​‌​​​‌​​‌​​​​​​​​​​​​​​​​​​​​​‌​​‌​​‌​​​‌​⁠
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation; either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package taskqueue

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain guards against goroutine leaks: every test in this package
// starts and stops queue workers, so a leak here compounds over a
// long-running daemon. goleak fails the package if any goroutine outlives
// the tests.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
