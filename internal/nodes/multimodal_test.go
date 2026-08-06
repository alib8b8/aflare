// Copyright (c) 2026 llm-box Contributors
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

package nodes

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestFallbackVisionCall_SendsImageContent is the regression test for the bug
// where the vision path stripped image_url content parts and only sent the
// prompt text to the LLM. It spins up a fake OpenAI-compatible endpoint,
// captures the request body, and asserts the image_url part (with its data
// URL) is present on the wire.
func TestFallbackVisionCall_SendsImageContent(t *testing.T) {
	var (
		capturedBody  []byte
		capturedAuthz string
	)
	mux := http.NewServeMux()
	mux.HandleFunc("/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		capturedBody = b
		capturedAuthz = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"image-seen"}}]}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	msgs := []visionMessage{
		{Role: "user", Content: []visionMessageContent{
			{Type: "text", Text: "describe this image"},
			{Type: "image_url"},
		}},
	}
	msgs[0].Content[1].ImageURL.URL = "data:image/png;base64,iVBORw0KGgo="
	msgs[0].Content[1].ImageURL.Detail = "auto"

	out, err := fallbackVisionCall(context.Background(), "openai", "gpt-4o", "test-key", server.URL, msgs)
	if err != nil {
		t.Fatalf("fallbackVisionCall failed: %v", err)
	}
	if out != "image-seen" {
		t.Errorf("expected response content 'image-seen', got %q", out)
	}
	if capturedAuthz != "Bearer test-key" {
		t.Errorf("expected Authorization 'Bearer test-key', got %q", capturedAuthz)
	}

	// The core regression assertion: the image must be on the wire.
	bodyStr := string(capturedBody)
	if !strings.Contains(bodyStr, "image_url") {
		t.Errorf("request body must contain an image_url content part; got: %s", bodyStr)
	}
	if !strings.Contains(bodyStr, "data:image/png;base64,") {
		t.Errorf("request body must carry the image data URL; got: %s", bodyStr)
	}
	// And the prompt text must still be present too.
	if !strings.Contains(bodyStr, "describe this image") {
		t.Errorf("request body must still carry the text prompt; got: %s", bodyStr)
	}

	// Structural check: the request deserializes into a visionRequest with the
	// expected model + a single user message carrying text + image_url parts.
	var req visionRequest
	if err := json.Unmarshal(capturedBody, &req); err != nil {
		t.Fatalf("request body did not deserialize as visionRequest: %v", err)
	}
	if req.Model != "gpt-4o" {
		t.Errorf("expected model 'gpt-4o', got %q", req.Model)
	}
	if len(req.Messages) != 1 || req.Messages[0].Role != "user" {
		t.Fatalf("expected one user message, got %+v", req.Messages)
	}
	parts := req.Messages[0].Content
	if len(parts) != 2 {
		t.Fatalf("expected 2 content parts (text + image_url), got %d", len(parts))
	}
	if parts[0].Type != "text" || parts[0].Text != "describe this image" {
		t.Errorf("unexpected first content part: %+v", parts[0])
	}
	if parts[1].Type != "image_url" {
		t.Errorf("expected second content part type 'image_url', got %q", parts[1].Type)
	}
	if parts[1].ImageURL.URL != "data:image/png;base64,iVBORw0KGgo=" {
		t.Errorf("image data URL not preserved: %q", parts[1].ImageURL.URL)
	}
	if parts[1].ImageURL.Detail != "auto" {
		t.Errorf("expected detail 'auto', got %q", parts[1].ImageURL.Detail)
	}
}

// TestFallbackVisionCall_MissingAPIKey ensures the call fails fast (no network
// attempt) when no API key is configured, rather than sending an unauthenticated
// request.
func TestFallbackVisionCall_MissingAPIKey(t *testing.T) {
	msgs := []visionMessage{
		{Role: "user", Content: []visionMessageContent{{Type: "text", Text: "hi"}}},
	}
	_, err := fallbackVisionCall(context.Background(), "openai", "gpt-4o", "", "https://api.openai.com/v1", msgs)
	if err == nil {
		t.Fatal("expected an error when no API key is provided")
	}
	if !strings.Contains(err.Error(), "no API key provided") {
		t.Errorf("expected 'no API key provided' error, got: %v", err)
	}
}

// TestFallbackVisionCall_PropagatesAPIError verifies that a non-200 response
// from the provider is surfaced as an error carrying the status code (and the
// provider's error message when present), not silently swallowed.
func TestFallbackVisionCall_PropagatesAPIError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid model"}}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	msgs := []visionMessage{
		{Role: "user", Content: []visionMessageContent{{Type: "text", Text: "hi"}}},
	}
	_, err := fallbackVisionCall(context.Background(), "openai", "gpt-4o", "test-key", server.URL, msgs)
	if err == nil {
		t.Fatal("expected an error for a 400 response")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("error should mention status 400, got: %v", err)
	}
	if !strings.Contains(err.Error(), "invalid model") {
		t.Errorf("error should carry provider message 'invalid model', got: %v", err)
	}
}
