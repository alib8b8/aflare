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

// These tests exercise the unexported helper functions defined alongside
// the simulated LLM provider nodes (sensenova/antling/andesgpt). They
// were migrated from internal/nodes/nodes_coverage_boost_test.go when the
// provider files moved into the providers subpackage, because the helpers
// themselves moved and are no longer accessible from the top-level nodes
// package.

package providers

import (
	"strings"
	"testing"
)

func TestDetermineExecLocation(t *testing.T) {
	tests := []struct {
		modelSize    string
		endCloudMode string
		input        string
		expected     string
	}{
		{"tiny", "auto", "test", "on_device"},
		{"titan", "auto", "test", "cloud"},
		{"turbo", "auto", "short", "on_device"},
		{"turbo", "auto", strings.Repeat("a", 150), "end_cloud_collaboration"},
		{"any", "force_end", "test", "on_device"},
		{"any", "force_cloud", "test", "cloud"},
		{"unknown", "auto", "test", "cloud"},
	}
	for _, tt := range tests {
		got := determineExecLocation(tt.modelSize, tt.endCloudMode, tt.input)
		if got != tt.expected {
			t.Errorf("determineExecLocation(%q, %q, %q) = %q, want %q",
				tt.modelSize, tt.endCloudMode, tt.input, got, tt.expected)
		}
	}
}

func TestSimulateAndesResponse(t *testing.T) {
	tests := []struct {
		scene   string
		wantSub string
	}{
		{"life", "生活助手"},
		{"imaging", "影像AI"},
		{"productivity", "效率助手"},
		{"creative", "创作助手"},
		{"knowledge", "知识问答"},
		{"unknown", "AndesGPT"},
	}
	for _, tt := range tests {
		got := simulateAndesResponse("test input", "turbo", tt.scene, "", false, "", 100)
		if !strings.Contains(got, tt.wantSub) {
			t.Errorf("simulateAndesResponse(scene=%q) = %q, want substring %q", tt.scene, got, tt.wantSub)
		}
	}

	got := simulateAndesResponse("推荐餐厅", "turbo", "life", "user123", true, "", 100)
	if !strings.Contains(got, "个人偏好") {
		t.Errorf("expected memory+persona response, got %q", got)
	}
}

func TestSimulateAntLingResponse(t *testing.T) {
	tests := []struct {
		scene   string
		wantSub string
	}{
		{"chat", "蚂蚁百灵"},
		{"code", "```"},
		{"analysis", "深度分析"},
		{"creative", "创意内容"},
		{"unknown", "蚂蚁百灵"},
	}
	for _, tt := range tests {
		got := simulateAntLingResponse("test", "ling-max", tt.scene, "", 100)
		if !strings.Contains(got, tt.wantSub) {
			t.Errorf("simulateAntLingResponse(scene=%q) = %q, want substring %q", tt.scene, got, tt.wantSub)
		}
	}

	got := simulateAntLingResponse("image task", "ming-flash-omni-2.0", "multimodal", "", 100)
	if !strings.Contains(got, "多模态") {
		t.Errorf("expected multimodal response, got %q", got)
	}

	got = simulateAntLingResponse("image task", "ling-max", "multimodal", "", 100)
	if !strings.Contains(got, "不支持原生多模态") {
		t.Errorf("expected non-multimodal response, got %q", got)
	}
}

func TestSimulateSenseNovaResponse(t *testing.T) {
	scenes := []string{"chat", "code", "image", "document", "data", "workflow", "unknown"}
	for _, s := range scenes {
		got := simulateSenseNovaResponse("test", "sense-max", s, "", 100)
		if got == "" {
			t.Errorf("empty response for scene %s", s)
		}
	}

	got := simulateSenseNovaResponse("test", "u1-max", "image", "", 100)
	if !strings.Contains(got, "U1") {
		t.Errorf("expected U1 in image response, got %q", got)
	}
}
