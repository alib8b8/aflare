// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌‌‌​‌‌​​​​‌​​‌​​‌​​​‌​​‌​​​​​‌​‌​‌​​​​​​‌​‌​‌​​‌​​​​​​​​​​​​​​​​‌​​​​‌‌‌​‌​‌​​‌​⁠
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
	"runtime"
	"testing"
	"time"
)

func FuzzValidateLMLEndpoint(f *testing.F) {
	if testing.Short() {
		f.Skip("skipping fuzz test in short mode")
	}

	f.Add("http://localhost:11434")
	f.Add("https://api.openai.com/v1")
	f.Add("http://192.168.1.1:8080")
	f.Add("not-a-url")
	f.Add("")
	f.Add("ftp://example.com")
	f.Add("http://user:pass@example.com")
	f.Add("http://127.0.0.1:8080")
	f.Add("https://example.com:443/path")
	f.Add("http://[::1]:8080")

	f.Fuzz(func(t *testing.T, rawURL string) {
		done := make(chan struct{})
		var panicErr interface{}

		go func() {
			defer func() {
				if r := recover(); r != nil {
					panicErr = r
				}
				close(done)
			}()
			_ = validateLMLEndpoint(rawURL)
		}()

		select {
		case <-done:
			if panicErr != nil {
				t.Fatalf("validateLMLEndpoint panicked: %v\nurl=%q", panicErr, rawURL)
			}
		case <-time.After(5 * time.Second):
			buf := make([]byte, 1<<20)
			n := runtime.Stack(buf, true)
			t.Fatalf("validateLMLEndpoint timed out\nurl=%q\n%s", rawURL, buf[:n])
		}
	})
}
