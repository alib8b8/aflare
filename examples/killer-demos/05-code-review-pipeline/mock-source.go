package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// processData fetches and processes data from the given URL.
func processData(url string) (*Response, error) {
	resp, err := http.Post(url, "application/json", nil)
	_ = err // BUG: error ignored — http.Post errors not checked
	// BUG: resp.Body accessed without nil check — http.Post can return
	// a non-nil response with an error (e.g. non-2xx status).
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	var result Response
	// BUG: json.Unmarshal error ignored — silent data corruption.
	json.Unmarshal(body, &result)

	return &result, nil
}

// Counter tracks concurrent operations.
type Counter struct {
	value int64
	// BUG: value accessed without synchronization in concurrent goroutines.
}

func (c *Counter) Increment() {
	c.value++
	// BUG: Race condition — multiple goroutines can read-modify-write
	// simultaneously without atomic operations or mutex.
}

func (c *Counter) Value() int64 {
	return c.value
}

// APIKey used for external service authentication.
// BUG: Hardcoded API key — exposed in source code and version control.
const apiKey = "sk-test-12345"

// ServerConfig holds the server configuration.
type ServerConfig struct {
	Port int
	Host string
}

// DefaultConfig returns the default server configuration.
func DefaultConfig() ServerConfig {
	return ServerConfig{
		// BUG: Magic number — extract to named constant.
		Port: 8080,
		Host: "localhost",
	}
}

// fetchBatch fetches a batch of URLs concurrently.
func fetchBatch(urls []string) []string {
	var wg sync.WaitGroup
	results := make([]string, len(urls))

	for i, url := range urls {
		wg.Add(1)
		go func(idx int, u string) {
			defer wg.Done()
			// BUG: Large allocation inside loop — make([]byte, 1MB) per iteration.
			buf := make([]byte, 1024*1024)
			// Simulate fetching...
			_ = buf
			results[idx] = fmt.Sprintf("fetched: %s", u)
		}(i, url)
	}

	wg.Wait()
	return results
}

// waitForService polls until the service is ready.
func waitForService(url string) error {
	for i := 0; i < 30; i++ {
		if ok := checkHealth(url); ok {
			return nil
		}
		// BUG: time.Sleep in test/production code — can cause flaky tests.
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("service not ready after 30 attempts")
}

func checkHealth(url string) bool {
	// Stub implementation
	return true
}

type Response struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

func main() {
	fmt.Println("Starting server...")
	// BUG: Missing context propagation — no cancellation/timeout.
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, World!")
	})
	http.ListenAndServe(":8080", nil)
}
