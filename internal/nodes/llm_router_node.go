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

package nodes

import (
	"context"
	"fmt"
	"strings"
)

type LLMRouterNode struct{}

func init() {
	Register(&LLMRouterNode{})
}

func (n *LLMRouterNode) Name() string {
	return "llm_router"
}

func (n *LLMRouterNode) Description() string {
	return "Multi-model intelligent router with automatic fallback, quota awareness, and cost optimization"
}

func (n *LLMRouterNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "llm_router",
		Description: "Smart LLM router that automatically selects the best provider with fallback, quota tracking, and cost optimization",
		Input:       "string - user message content to send to LLM",
		Output:      "string - AI response from the selected provider",
		Params: []ParamSchema{
			{Name: "system", Type: "string", Description: "System prompt for the LLM", Required: false},
			{Name: "strategy", Type: "string", Description: "Routing strategy: priority, cost, latency, pareto, round_robin, random (default: priority)", Required: false, Default: "priority"},
			{Name: "max_retries", Type: "string", Description: "Maximum number of fallback attempts (default: 3)", Required: false, Default: "3"},
			{Name: "show_provider", Type: "string", Description: "Show which provider was used in output (default: false)", Required: false, Default: "false"},
			{Name: "show_stats", Type: "string", Description: "Show router statistics in output (default: false)", Required: false, Default: "false"},
		},
	}
}

func (n *LLMRouterNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	showProvider := getParam(params, "show_provider", "false") == "true"
	showStats := getParam(params, "show_stats", "false") == "true"

	// strategy is forwarded via params; the router resolves it per call so
	// a workflow's strategy override never mutates the shared global router.
	router := GetGlobalRouter()

	response, providerName, err := router.Execute(ctx, input, params)
	if err != nil {
		return "", err
	}

	if showStats {
		stats := formatRouterStats(router)
		return fmt.Sprintf("%s\n\n---\n\n%s", response, stats), nil
	}

	if showProvider {
		return fmt.Sprintf("[%s]\n\n%s", providerName, response), nil
	}

	return response, nil
}

func formatRouterStats(router *LLMRouter) string {
	var sb strings.Builder
	sb.WriteString("[LLM Router Stats]\n")
	sb.WriteString(fmt.Sprintf("Strategy: %s\n", router.GetStrategy()))
	sb.WriteString(fmt.Sprintf("Active providers: %d\n", len(router.GetProviders())))

	stats := router.GetProviderStats()
	for _, p := range router.GetProviders() {
		s, ok := stats[p.Name]
		status := "idle"
		switch {
		case !p.Enabled:
			status = "disabled"
		case ok && s.TotalCalls > 0:
			// Use observed success rate (SuccessCalls/TotalCalls), not the
			// static config SuccessRate, so stats reflect real behavior.
			successPct := float64(s.SuccessCalls) / float64(s.TotalCalls) * 100
			status = fmt.Sprintf("%d calls, %.0f%% success", s.TotalCalls, successPct)
			status += fmt.Sprintf(", avg %dms", s.TotalLatency/s.TotalCalls)
		case ok:
			status = fmt.Sprintf("%d calls", s.TotalCalls)
		}
		sb.WriteString(fmt.Sprintf("  - %s (%s): %s\n", p.Name, p.Model, status))
	}

	return sb.String()
}
