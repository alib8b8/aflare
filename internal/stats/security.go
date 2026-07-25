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

package stats

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type SecurityBlockType string

const (
	BlockPathTraversal    SecurityBlockType = "path_traversal"
	BlockCommandInjection SecurityBlockType = "command_injection"
	BlockSSRF             SecurityBlockType = "ssrf"
	BlockSensitiveFile    SecurityBlockType = "sensitive_file"
	BlockUnsafeExtension  SecurityBlockType = "unsafe_extension"
	BlockNetwork          SecurityBlockType = "network"
	BlockSafeMode         SecurityBlockType = "safe_mode"
	BlockSymlinkBypass    SecurityBlockType = "symlink_bypass"
	BlockCredentialLeak   SecurityBlockType = "credential_leak"
)

type SecurityEvent struct {
	Type      SecurityBlockType `json:"type"`
	Detail    string            `json:"detail"`
	Node      string            `json:"node,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
}

type SecurityStats struct {
	mu                sync.RWMutex
	TotalBlocks       int64                       `json:"total_blocks"`
	ByType            map[SecurityBlockType]int64 `json:"by_type"`
	ByNode            map[string]int64            `json:"by_node"`
	SecurityLevelUsed map[string]int64            `json:"security_level_used"`
	CodeInterpreter   struct {
		TotalRuns    int64 `json:"total_runs"`
		TimeoutCount int64 `json:"timeout_count"`
		BlockedCount int64 `json:"blocked_count"`
		AvgMs        int64 `json:"avg_duration_ms"`
		totalMs      int64
	} `json:"code_interpreter"`
	HTTPRequests struct {
		TotalRequests  int64 `json:"total_requests"`
		BlockedSSRF    int64 `json:"blocked_ssrf"`
		BlockedNonHTTP int64 `json:"blocked_non_http"`
	} `json:"http_requests"`
	RecentEvents []SecurityEvent `json:"recent_events"`
}

var globalSecurityStats = &SecurityStats{
	ByType:            make(map[SecurityBlockType]int64),
	ByNode:            make(map[string]int64),
	SecurityLevelUsed: make(map[string]int64),
	RecentEvents:      make([]SecurityEvent, 0, 100),
}

func GetSecurityStats() *SecurityStats {
	return globalSecurityStats
}

func (s *SecurityStats) RecordBlock(blockType SecurityBlockType, detail, node string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.TotalBlocks++
	s.ByType[blockType]++
	if node != "" {
		s.ByNode[node]++
	}

	event := SecurityEvent{
		Type:      blockType,
		Detail:    detail,
		Node:      node,
		Timestamp: time.Now(),
	}
	s.RecentEvents = append(s.RecentEvents, event)
	if len(s.RecentEvents) > 100 {
		s.RecentEvents = s.RecentEvents[1:]
	}
}

func (s *SecurityStats) RecordSecurityLevel(level string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.SecurityLevelUsed[level]++
}

func (s *SecurityStats) RecordCodeInterpreterRun(durationMs int64, blocked, timedOut bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.CodeInterpreter.TotalRuns++
	s.CodeInterpreter.totalMs += durationMs
	if s.CodeInterpreter.TotalRuns > 0 {
		s.CodeInterpreter.AvgMs = s.CodeInterpreter.totalMs / s.CodeInterpreter.TotalRuns
	}
	if blocked {
		s.CodeInterpreter.BlockedCount++
	}
	if timedOut {
		s.CodeInterpreter.TimeoutCount++
	}
}

func (s *SecurityStats) RecordHTTPRequest(blockedSSRF, blockedNonHTTP bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.HTTPRequests.TotalRequests++
	if blockedSSRF {
		s.HTTPRequests.BlockedSSRF++
	}
	if blockedNonHTTP {
		s.HTTPRequests.BlockedNonHTTP++
	}
}

func (s *SecurityStats) ToJSON() (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *SecurityStats) FormatReport() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var report string
	report += "🔒 Security Stats Report\n"
	report += "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n"

	report += fmt.Sprintf("Total blocks:       %d\n", s.TotalBlocks)
	report += fmt.Sprintf("Recent events:      %d\n\n", len(s.RecentEvents))

	if len(s.ByType) > 0 {
		report += "Blocks by Type:\n"
		for t, c := range s.ByType {
			report += fmt.Sprintf("  %-25s %d\n", t, c)
		}
		report += "\n"
	}

	if len(s.SecurityLevelUsed) > 0 {
		report += "Security Level Usage:\n"
		for l, c := range s.SecurityLevelUsed {
			report += fmt.Sprintf("  %s: %d\n", l, c)
		}
		report += "\n"
	}

	report += fmt.Sprintf("Code Interpreter:\n")
	report += fmt.Sprintf("  Total runs:       %d\n", s.CodeInterpreter.TotalRuns)
	report += fmt.Sprintf("  Blocked:          %d\n", s.CodeInterpreter.BlockedCount)
	report += fmt.Sprintf("  Timed out:        %d\n", s.CodeInterpreter.TimeoutCount)
	report += fmt.Sprintf("  Avg duration:     %d ms\n\n", s.CodeInterpreter.AvgMs)

	report += fmt.Sprintf("HTTP Requests:\n")
	report += fmt.Sprintf("  Total:            %d\n", s.HTTPRequests.TotalRequests)
	report += fmt.Sprintf("  SSRF blocked:     %d\n", s.HTTPRequests.BlockedSSRF)
	report += fmt.Sprintf("  Non-HTTP blocked: %d\n", s.HTTPRequests.BlockedNonHTTP)

	return report
}
