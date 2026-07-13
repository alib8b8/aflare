package nodes

import (
	"runtime"
	"testing"
	"time"
)

func FuzzValidateLMLEndpoint(f *testing.F) {
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
