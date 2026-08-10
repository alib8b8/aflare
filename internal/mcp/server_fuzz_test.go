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

package mcp

import (
	"encoding/json"
	"runtime"
	"testing"
	"time"
)

// FuzzMCPHandleRequest fuzzes the MCP server's handleRequest method with
// arbitrary JSON-RPC requests. This tests the JSON deserialization and
// method dispatch against malformed inputs.
func FuzzMCPHandleRequest(f *testing.F) {
	// Seed corpus: valid requests, edge cases, and malformed JSON.
	seeds := []string{
		// Valid initialize
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		// Valid tools/list
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		// Valid tools/call
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"create_workflow","arguments":{"description":"test"}}}`,
		// Unknown method
		`{"jsonrpc":"2.0","id":4,"method":"nonexistent"}`,
		// Empty
		``,
		`{}`,
		// Malformed JSON
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":`,
		`{{{{{{{{{{`,
		`{"jsonrpc"`,
		// Missing fields
		`{"id":1}`,
		`{"method":"initialize"}`,
		// Null bytes
		"{\"method\":\"\x00\"}",
		// Very long strings
		`{"jsonrpc":"2.0","id":1,"method":"` + repeat("a", 10000) + `"}`,
		// Deeply nested params
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"create_workflow","arguments":{"description":"test","nested":{"a":{"b":{"c":{"d":{"e":"deep"}}}}}}}}`,
		// Invalid ID types
		`{"jsonrpc":"2.0","id":"string-id","method":"initialize"}`,
		`{"jsonrpc":"2.0","id":null,"method":"initialize"}`,
		// Invalid method
		`{"jsonrpc":"2.0","id":1,"method":123}`,
	}

	for _, s := range seeds {
		f.Add(s)
	}

	srv := NewServer()

	f.Fuzz(func(t *testing.T, rawJSON string) {
		done := make(chan struct{})
		var panicErr interface{}

		go func() {
			defer func() {
				if r := recover(); r != nil {
					panicErr = r
				}
				close(done)
			}()

			var req rpcRequest
			if err := json.Unmarshal([]byte(rawJSON), &req); err != nil {
				return // malformed JSON is expected to fail
			}

			resp := srv.handleRequest(&req)
			if resp != nil {
				// Verify the response can be serialized
				_, err := json.Marshal(resp)
				if err != nil {
					t.Errorf("handleRequest returned unserializable response: %v", err)
				}
			}
		}()

		select {
		case <-done:
			if panicErr != nil {
				t.Fatalf("handleRequest panicked: %v\nrawJSON=%q", panicErr, rawJSON)
			}
		case <-time.After(5 * time.Second):
			buf := make([]byte, 1<<20)
			n := runtime.Stack(buf, true)
			t.Fatalf("handleRequest timed out\nrawJSON=%q\n%s", rawJSON, buf[:n])
		}
	})
}

func repeat(s string, n int) string {
	result := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		result = append(result, s...)
	}
	return string(result)
}
