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

package edge

import (
	"context"
	"strings"
	"testing"
)

// benchRouter builds a balanced router with an available local model and a
// cloud model, mirroring the test setup in edge_router_test.go.
func benchRouter(b *testing.B, level PrivacyLevel) *EdgeRouter {
	b.Helper()
	cfg := validConfig()
	cfg.PrivacyLevel = level
	cfg.LocalThreshold = 5
	r, err := NewEdgeRouter(cfg)
	if err != nil {
		b.Fatalf("NewEdgeRouter: %v", err)
	}
	r.localModel = &mockLocalModel{available: true}
	r.cloudModels["openai"] = &mockCloudModel{provider: "openai"}
	return r
}

// BenchmarkRoute measures the routing decision path, which includes task
// validation, the analyzeComplexity keyword scan, and provider selection.
// It covers the local (low complexity), cloud (high complexity), and
// sensitive-data branches.
func BenchmarkRoute(b *testing.B) {
	r := benchRouter(b, PrivacyBalanced)
	ctx := context.Background()

	cases := []struct {
		name string
		task TaskRequest
	}{
		{"simple_local", TaskRequest{ID: "t1", Prompt: "总结这段文字"}},
		{"complex_cloud", TaskRequest{ID: "t2", Prompt: "深度分析并比较两个方案的优劣，给出评估"}},
		{"sensitive_local", TaskRequest{ID: "t3", Prompt: "请处理这个密码和身份证号", ContainsSensitiveData: true}},
		{"long_prompt", TaskRequest{ID: "t4", Prompt: strings.Repeat("analyze this complex research topic deeply. ", 50)}},
	}

	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := r.Route(ctx, c.task); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkRoute_PrivacyStrict measures the fast early-return path where
// strict privacy forces every task to the local model without complexity
// analysis.
func BenchmarkRoute_PrivacyStrict(b *testing.B) {
	r := benchRouter(b, PrivacyStrict)
	ctx := context.Background()
	task := TaskRequest{ID: "t1", Prompt: "any prompt"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := r.Route(ctx, task); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkAnalyzeComplexity measures the keyword-based complexity scorer in
// isolation. It runs strings.ToLower plus two keyword-list scans over the
// prompt, which is the dominant per-Route cost for non-strict configs.
func BenchmarkAnalyzeComplexity(b *testing.B) {
	r := benchRouter(b, PrivacyBalanced)
	cases := []struct {
		name   string
		prompt string
	}{
		{"simple", "总结这段文字"},
		{"complex", "深度分析并比较两个方案的优劣"},
		{"long_clean", strings.Repeat("the quick brown fox. ", 50)},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			task := TaskRequest{ID: "t1", Prompt: c.prompt}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = r.analyzeComplexity(task)
			}
		})
	}
}

// BenchmarkPrivacyAnalyzer measures the regex-based sensitive-data detector.
// Analyze runs ToLower then matches against ~30 compiled patterns, so it is
// the heaviest part of privacy-aware routing.
func BenchmarkPrivacyAnalyzer(b *testing.B) {
	analyzer := NewPrivacyAnalyzer()
	cases := []struct {
		name string
		text string
	}{
		{"clean", "The quick brown fox jumps over the lazy dog."},
		{"with_password", "My password is hunter2 and my token is abc123def456"},
		{"with_email", "Contact alice@example.com for details about the project"},
		{"with_apikey", "Use api_key sk-ABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890 for access"},
		{"chinese_sensitive", "请提供你的身份证号和银行卡号用于验证"},
		{"long_clean", strings.Repeat("The quick brown fox jumps over the lazy dog. ", 20)},
	}

	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = analyzer.Analyze(c.text)
			}
		})
	}
}
