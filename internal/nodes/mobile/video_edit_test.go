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

func TestVideoEditNode_Metadata(t *testing.T) {
	node := &VideoEditNode{}
	if node.Name() != "video_edit" {
		t.Errorf("name: %s", node.Name())
	}
	if node.Description() == "" {
		t.Error("empty description")
	}
	schema := node.Schema()
	if schema.Name != "video_edit" {
		t.Errorf("schema name: %s", schema.Name)
	}
}

func TestVideoEditNode_ExecuteErrors(t *testing.T) {
	ctx := context.Background()
	node := &VideoEditNode{}

	tests := []struct {
		name   string
		input  string
		params map[string]string
		errSub string
	}{
		{
			"invalid operation",
			"",
			map[string]string{"operation": "invalid"},
			"invalid operation",
		},
		{
			"missing input_files",
			"",
			map[string]string{"operation": "smart_cut"},
			"input_files is required",
		},
		{
			"invalid output_path with ..",
			"vid.mp4",
			map[string]string{"operation": "smart_cut", "output_path": "../bad.mp4"},
			"output_path cannot contain '..'",
		},
		{
			"invalid style",
			"vid.mp4",
			map[string]string{"operation": "smart_cut", "style": "fancy"},
			"invalid style",
		},
		{
			"invalid resolution",
			"vid.mp4",
			map[string]string{"operation": "smart_cut", "resolution": "8k"},
			"invalid resolution",
		},
		{
			"invalid language",
			"vid.mp4",
			map[string]string{"operation": "smart_cut", "language": "klingon"},
			"invalid language",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := node.Execute(ctx, tt.input, tt.params)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.errSub) {
				t.Errorf("err = %q, want substring %q", err.Error(), tt.errSub)
			}
		})
	}
}

func TestVideoEditNode_ExecuteSuccess(t *testing.T) {
	ctx := context.Background()
	node := &VideoEditNode{}

	operations := []string{"smart_cut", "merge", "effects", "subtitle", "storyboard", "upscale"}
	for _, op := range operations {
		t.Run(op, func(t *testing.T) {
			out, err := node.Execute(ctx, "input1.mp4,input2.mp4", map[string]string{
				"operation":  op,
				"style":      "cinematic",
				"resolution": "1080p",
				"language":   "中文",
				"duration":   "30.0",
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(out, op) {
				t.Errorf("expected operation %q in output: %s", op, out)
			}
			if !strings.Contains(out, "\"status\": \"success\"") {
				t.Errorf("expected status success: %s", out)
			}
		})
	}
}

func TestVideoEditNode_ExecuteInputFromInput(t *testing.T) {
	ctx := context.Background()
	node := &VideoEditNode{}

	// input_files not set, falls back to input
	out, err := node.Execute(ctx, "clip1.mp4,clip2.mp4", map[string]string{
		"operation": "merge",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "clip1.mp4") {
		t.Error("expected clip1.mp4 in output")
	}
}

func TestVideoEditNode_ExecuteSubtitleAddsLanguage(t *testing.T) {
	ctx := context.Background()
	node := &VideoEditNode{}

	out, err := node.Execute(ctx, "vid.mp4", map[string]string{
		"operation": "subtitle",
		"language":  "英文",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "\"language\": \"英文\"") {
		t.Errorf("expected language field for subtitle op: %s", out)
	}
}

func TestParseInputFiles(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"empty", "", 0},
		{"single file", "vid.mp4", 1},
		{"multiple files", "a.mp4,b.mp4,c.mp4", 3},
		{"trims whitespace", " a.mp4 , b.mp4 ", 2},
		{"skips empty parts", "a.mp4,,b.mp4,", 2},
		{"rejects path traversal", "../etc/passwd,vid.mp4", 1},
		{"rejects shell metacharacters", "a.mp4;rm -rf,b.mp4&c,d.mp4|e", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseInputFiles(tt.input)
			if len(got) != tt.want {
				t.Errorf("got %d files (%v), want %d", len(got), got, tt.want)
			}
		})
	}
}

func TestValidateFilePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"empty", "", true},
		{"valid relative", "./output.mp4", false},
		{"valid absolute", "/tmp/output.mp4", false},
		{"path traversal", "../output.mp4", true},
		{"nested path traversal", "dir/../../etc/passwd", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFilePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateFilePath(%q) err = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

// Ensure video_edit node was registered.
func TestVideoEditNode_Registered(t *testing.T) {
	if _, ok := core.Get("video_edit"); !ok {
		t.Error("video_edit not registered")
	}
}
