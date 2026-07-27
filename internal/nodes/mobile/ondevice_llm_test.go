// Copyright (c) 2026 llm-box Contributors
//
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

package mobile

import (
	"context"
	"strings"
	"testing"

	"github.com/alib8b8/llm-box/internal/nodes/core"
)

func TestOnDeviceLLMNode_Metadata(t *testing.T) {
	node := &OnDeviceLLMNode{}
	if node.Name() != "ondevice_llm" {
		t.Errorf("name: %s", node.Name())
	}
	if node.Description() == "" {
		t.Error("empty description")
	}
	schema := node.Schema()
	if schema.Name != "ondevice_llm" {
		t.Errorf("schema name: %s", schema.Name)
	}
	if len(schema.Params) == 0 {
		t.Error("expected params")
	}
}

func TestOnDeviceLLMNode_ExecuteErrors(t *testing.T) {
	ctx := context.Background()
	node := &OnDeviceLLMNode{}

	tests := []struct {
		name   string
		params map[string]string
		errSub string
	}{
		{"missing model", map[string]string{}, "model parameter is required"},
		{"unsupported model", map[string]string{"model": "gpt-4"}, "unsupported model"},
		{"unsupported backend", map[string]string{"model": "qwen2-1.5b", "backend": "tensorrt"}, "unsupported backend"},
		{"unsupported quantization", map[string]string{"model": "qwen2-1.5b", "quantization": "fp32"}, "unsupported quantization"},
		{"path traversal model_path", map[string]string{"model": "qwen2-1.5b", "model_path": "../etc/passwd"}, "path traversal not allowed"},
		{"invalid model_path chars", map[string]string{"model": "qwen2-1.5b", "model_path": "bad path!"}, "invalid model_path"},
		{"system_prompt too long", map[string]string{"model": "qwen2-1.5b", "system_prompt": strings.Repeat("a", 4001)}, "system_prompt too long"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := node.Execute(ctx, "hello", tt.params)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.errSub) {
				t.Errorf("err = %q, want substring %q", err.Error(), tt.errSub)
			}
		})
	}
}

func TestOnDeviceLLMNode_ExecuteSuccess(t *testing.T) {
	ctx := context.Background()
	node := &OnDeviceLLMNode{}

	// Basic successful execution
	out, err := node.Execute(ctx, "translate this", map[string]string{
		"model":         "qwen2.5-1.5b",
		"backend":       "mlc-llm",
		"quantization":  "int8",
		"max_tokens":    "100",
		"temperature":   "0.5",
		"context_size":  "2048",
		"threads":       "4",
		"use_gpu":       "false",
		"system_prompt": "You are a helpful assistant",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "ondevice_llm") {
		t.Errorf("unexpected output: %s", out)
	}
	if !strings.Contains(out, "Translation") {
		t.Errorf("expected translation response, got: %s", out)
	}
}

func TestOnDeviceLLMNode_ExecuteParamClamping(t *testing.T) {
	ctx := context.Background()
	node := &OnDeviceLLMNode{}

	// Out-of-range values should be clamped to defaults, not error
	out, err := node.Execute(ctx, "hello", map[string]string{
		"model":        "qwen2-1.5b",
		"max_tokens":   "99999", // > 4096, falls back to 512
		"temperature":  "5.0",   // > 2.0, falls back to 0.7
		"context_size": "99999", // > 32768, falls back to 4096
		"threads":      "999",   // > 64, falls back to 0
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "\"max_tokens\": 512") {
		t.Errorf("expected max_tokens 512 in output: %s", out)
	}
	if !strings.Contains(out, "\"temperature\": 0.7") {
		t.Errorf("expected temperature 0.7 in output: %s", out)
	}
}

func TestOnDeviceLLMNode_ExecuteModelPathUnderHome(t *testing.T) {
	// Set HOME to a temp dir so model_path validation succeeds
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	ctx := context.Background()
	node := &OnDeviceLLMNode{}

	out, err := node.Execute(ctx, "hello", map[string]string{
		"model":      "qwen2-1.5b",
		"model_path": tmpDir + "/models/qwen",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, tmpDir) {
		t.Errorf("expected model_path in output: %s", out)
	}
}

func TestOnDeviceLLMNode_ExecuteModelPathOutsideHome(t *testing.T) {
	// Set HOME to a temp dir, then provide a model_path outside home and not under /opt or /usr/local
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	ctx := context.Background()
	node := &OnDeviceLLMNode{}

	_, err := node.Execute(ctx, "hello", map[string]string{
		"model":      "qwen2-1.5b",
		"model_path": "/etc/some/path",
	})
	if err == nil {
		t.Fatal("expected error for model_path outside home")
	}
	if !strings.Contains(err.Error(), "must be under home directory") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestEstimateModelSize(t *testing.T) {
	tests := []struct {
		model        string
		quantization string
		wantPositive bool
	}{
		{"qwen2-0.5b", "int4", true},
		{"qwen2-1.5b", "int8", true},
		{"phi-3-mini", "fp16", true},
		{"unknown-model", "int4", true}, // falls back to 2000 base
		{"qwen2-1.5b", "unknown-quant", true},
	}
	for _, tt := range tests {
		got := estimateModelSize(tt.model, tt.quantization)
		if tt.wantPositive && got <= 0 {
			t.Errorf("estimateModelSize(%q, %q) = %d, want positive", tt.model, tt.quantization, got)
		}
	}

	// Sanity: int4 should be smaller than fp16 for the same model
	int4 := estimateModelSize("qwen2-1.5b", "int4")
	fp16 := estimateModelSize("qwen2-1.5b", "fp16")
	if int4 >= fp16 {
		t.Errorf("expected int4 (%d) < fp16 (%d)", int4, fp16)
	}
}

func TestEstimateMemoryRequired(t *testing.T) {
	mem := estimateMemoryRequired("qwen2-1.5b", "int4", 4096)
	if mem <= 0 {
		t.Errorf("expected positive memory, got %d", mem)
	}

	// Larger context should require more memory
	small := estimateMemoryRequired("qwen2-1.5b", "int4", 1024)
	large := estimateMemoryRequired("qwen2-1.5b", "int4", 8192)
	if small >= large {
		t.Errorf("expected larger context to need more memory: small=%d, large=%d", small, large)
	}
}

func TestOnDeviceModelRegistry(t *testing.T) {
	r := &OnDeviceModelRegistry{
		models: make(map[string]*OnDeviceModelInfo),
	}

	// Initially empty
	if ready := r.ListReady(); len(ready) != 0 {
		t.Errorf("expected 0 ready models, got %d", len(ready))
	}

	if _, ok := r.Get("nonexistent"); ok {
		t.Error("expected Get to return false for nonexistent model")
	}

	// Register a not-downloaded model
	r.Register(&OnDeviceModelInfo{
		Name:           "qwen2-1.5b",
		Path:           "/models/qwen",
		SizeMB:         1500,
		Quantization:   "int4",
		Backend:        "llama.cpp",
		ContextSize:    4096,
		DownloadStatus: "not_downloaded",
	})

	m, ok := r.Get("qwen2-1.5b")
	if !ok {
		t.Fatal("expected Get to find registered model")
	}
	if m.SizeMB != 1500 {
		t.Errorf("SizeMB: got %d, want 1500", m.SizeMB)
	}
	if ready := r.ListReady(); len(ready) != 0 {
		t.Errorf("expected 0 ready models, got %d", len(ready))
	}

	// Register a ready model
	r.Register(&OnDeviceModelInfo{
		Name:           "phi-3-mini",
		Path:           "/models/phi",
		SizeMB:         3800,
		Quantization:   "int8",
		Backend:        "mlc-llm",
		ContextSize:    4096,
		DownloadStatus: "ready",
	})

	ready := r.ListReady()
	if len(ready) != 1 {
		t.Errorf("expected 1 ready model, got %d", len(ready))
	}
	if ready[0] != "phi-3-mini" {
		t.Errorf("ready[0]: got %q, want phi-3-mini", ready[0])
	}

	// Re-register to overwrite
	r.Register(&OnDeviceModelInfo{
		Name:           "qwen2-1.5b",
		Path:           "/models/qwen2",
		SizeMB:         2000,
		Quantization:   "int8",
		Backend:        "llama.cpp",
		ContextSize:    2048,
		DownloadStatus: "ready",
	})
	m, _ = r.Get("qwen2-1.5b")
	if m.SizeMB != 2000 {
		t.Errorf("after re-register, SizeMB: got %d, want 2000", m.SizeMB)
	}
	ready = r.ListReady()
	if len(ready) != 2 {
		t.Errorf("expected 2 ready models, got %d", len(ready))
	}
}

// Ensure ondevice_llm node was registered.
func TestOnDeviceLLMNode_Registered(t *testing.T) {
	if _, ok := core.Get("ondevice_llm"); !ok {
		t.Error("ondevice_llm not registered")
	}
}
