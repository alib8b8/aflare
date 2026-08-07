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
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// AgentCapability represents a capability that an agent exposes at runtime.
type AgentCapability struct {
	AgentID      string   `json:"agent_id"`
	AgentName    string   `json:"agent_name"`
	Capabilities []string `json:"capabilities"`
	Confidence   float64  `json:"confidence"`
	RegisteredAt string   `json:"registered_at"`
	LastSeen     string   `json:"last_seen"`
	Status       string   `json:"status"` // active, idle, busy, offline
}

// agentDiscovery stores the runtime agent registry.
var (
	agentDiscoveryRegistry   = make(map[string]*AgentCapability)
	agentDiscoveryMu         sync.RWMutex
	agentDiscoveryCleanupAt  time.Time
)

var (
	validConsensusStrategies = map[string]bool{
		"majority":   true,
		"weighted":   true,
		"unanimous":  true,
		"round_robin": true,
		"ranked":     true,
	}
	validOrchestratorActions = map[string]bool{
		"discover":  true,
		"consensus": true,
		"register":  true,
		"deregister": true,
		"list":      true,
		"status":    true,
	}
)

// AgentOrchestratorNode provides runtime agent discovery and consensus
// mechanisms for multi-agent collaboration. It extends the supervisor
// with Avernet-style agent discovery, voting, and governance.
type AgentOrchestratorNode struct{}

func (n *AgentOrchestratorNode) Name() string { return "agent_orchestrator" }

func (n *AgentOrchestratorNode) Description() string {
	return "多智能体编排节点。提供运行时 Agent 发现注册、共识投票（多数/加权/全票/轮询/排序）、跨团队协作治理能力。"
}

func (n *AgentOrchestratorNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        n.Name(),
		Description: n.Description(),
		Input:       "string - 任务描述或共识议题",
		Output:      "string - JSON格式的编排结果",
		Params: []ParamSchema{
			{Name: "action", Type: "string", Description: "操作：discover（发现）、consensus（共识）、register（注册）、deregister（注销）、list（列表）、status（状态）", Required: false, Default: "discover"},
			{Name: "consensus_strategy", Type: "string", Description: "共识策略：majority（多数决）、weighted（加权）、unanimous（全票）、round_robin（轮询）、ranked（排序）", Required: false, Default: "majority"},
			{Name: "agent_id", Type: "string", Description: "Agent ID（注册/注销时必填）", Required: false},
			{Name: "agent_name", Type: "string", Description: "Agent 名称", Required: false},
			{Name: "capabilities", Type: "string", Description: "Agent 能力列表（逗号分隔）", Required: false},
			{Name: "min_agents", Type: "int", Description: "最少参与 Agent 数（默认 2）", Required: false, Default: "2"},
			{Name: "timeout_ms", Type: "int", Description: "共识超时（毫秒，默认 30000）", Required: false, Default: "30000"},
			{Name: "governance", Type: "string", Description: "治理模式：open（开放注册）、approval（需审批）、restricted（白名单）", Required: false, Default: "open"},
		},
	}
}

func (n *AgentOrchestratorNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	action := getParam(params, "action", "discover")
	if !validOrchestratorActions[action] {
		return "", fmt.Errorf("invalid action: %s (valid: discover, consensus, register, deregister, list, status)", action)
	}

	startTime := time.Now()

	// Periodic cleanup of stale agents
	cleanupStaleAgents()

	var result map[string]interface{}

	switch action {
	case "register":
		result = n.handleRegister(params)
	case "deregister":
		result = n.handleDeregister(params)
	case "discover":
		result = n.handleDiscover(input, params)
	case "consensus":
		result = n.handleConsensus(input, params)
	case "list":
		result = n.handleList(params)
	case "status":
		result = n.handleStatus(params)
	}

	result["action"] = action
	result["latency_ms"] = time.Since(startTime).Milliseconds()
	result["timestamp"] = time.Now().UTC().Format(time.RFC3339)

	output, _ := json.MarshalIndent(result, "", "  ")
	return string(output), nil
}

func (n *AgentOrchestratorNode) handleRegister(params map[string]string) map[string]interface{} {
	agentID := getParam(params, "agent_id", "")
	if agentID == "" {
		return map[string]interface{}{"status": "error", "error": "agent_id is required"}
	}

	agentName := getParam(params, "agent_name", agentID)
	capabilitiesStr := getParam(params, "capabilities", "")
	capabilities := []string{}
	if capabilitiesStr != "" {
		capabilities = strings.Split(capabilitiesStr, ",")
		for i := range capabilities {
			capabilities[i] = strings.TrimSpace(capabilities[i])
		}
	}

	governance := getParam(params, "governance", "open")

	agentDiscoveryMu.Lock()
	defer agentDiscoveryMu.Unlock()

	now := time.Now().UTC().Format(time.RFC3339)

	if existing, ok := agentDiscoveryRegistry[agentID]; ok {
		existing.Capabilities = capabilities
		existing.LastSeen = now
		existing.AgentName = agentName
		return map[string]interface{}{
			"status":  "updated",
			"agent":   existing,
			"message": fmt.Sprintf("Agent %s capabilities updated", agentID),
		}
	}

	agent := &AgentCapability{
		AgentID:      agentID,
		AgentName:    agentName,
		Capabilities: capabilities,
		Confidence:   1.0,
		RegisteredAt: now,
		LastSeen:     now,
		Status:       "active",
	}

	agentDiscoveryRegistry[agentID] = agent

	return map[string]interface{}{
		"status":     "registered",
		"agent":      agent,
		"governance": governance,
		"message":    fmt.Sprintf("Agent %s registered with %d capabilities", agentID, len(capabilities)),
	}
}

func (n *AgentOrchestratorNode) handleDeregister(params map[string]string) map[string]interface{} {
	agentID := getParam(params, "agent_id", "")
	if agentID == "" {
		return map[string]interface{}{"status": "error", "error": "agent_id is required"}
	}

	agentDiscoveryMu.Lock()
	defer agentDiscoveryMu.Unlock()

	if _, ok := agentDiscoveryRegistry[agentID]; !ok {
		return map[string]interface{}{"status": "error", "error": fmt.Sprintf("agent %s not found", agentID)}
	}

	delete(agentDiscoveryRegistry, agentID)
	return map[string]interface{}{
		"status":  "deregistered",
		"message": fmt.Sprintf("Agent %s removed from registry", agentID),
	}
}

func (n *AgentOrchestratorNode) handleDiscover(task string, params map[string]string) map[string]interface{} {
	agentDiscoveryMu.RLock()
	defer agentDiscoveryMu.RUnlock()

	lowerTask := strings.ToLower(task)

	var matched []*AgentCapability
	for _, agent := range agentDiscoveryRegistry {
		if agent.Status == "offline" {
			continue
		}
		// Match by capability
		for _, cap := range agent.Capabilities {
			if strings.Contains(lowerTask, strings.ToLower(cap)) {
				matched = append(matched, agent)
				break
			}
		}
	}

	// If no match by capability, return all active agents
	if len(matched) == 0 {
		for _, agent := range agentDiscoveryRegistry {
			if agent.Status != "offline" {
				matched = append(matched, agent)
			}
		}
	}

	return map[string]interface{}{
		"status":        "completed",
		"task":          task,
		"matched_agents": len(matched),
		"total_agents":  len(agentDiscoveryRegistry),
		"agents":        matched,
	}
}

func (n *AgentOrchestratorNode) handleConsensus(topic string, params map[string]string) map[string]interface{} {
	strategy := getParam(params, "consensus_strategy", "majority")
	if !validConsensusStrategies[strategy] {
		return map[string]interface{}{"status": "error", "error": fmt.Sprintf("invalid consensus strategy: %s", strategy)}
	}

	minAgents := parseIntSafe(getParam(params, "min_agents", "2"), 2)
	if minAgents < 2 {
		minAgents = 2
	}

	agentDiscoveryMu.RLock()
	activeAgents := make([]*AgentCapability, 0)
	for _, agent := range agentDiscoveryRegistry {
		if agent.Status != "offline" {
			activeAgents = append(activeAgents, agent)
		}
	}
	agentDiscoveryMu.RUnlock()

	if len(activeAgents) < minAgents {
		return map[string]interface{}{
			"status": "insufficient",
			"error":  fmt.Sprintf("need at least %d agents, only %d active", minAgents, len(activeAgents)),
		}
	}

	result := map[string]interface{}{
		"status":    "completed",
		"topic":     topic,
		"strategy":  strategy,
		"agent_count": len(activeAgents),
	}

	switch strategy {
	case "majority":
		result["result"] = simulateMajorityConsensus(activeAgents, topic)
	case "weighted":
		result["result"] = simulateWeightedConsensus(activeAgents, topic)
	case "unanimous":
		result["result"] = simulateUnanimousConsensus(activeAgents, topic)
	case "round_robin":
		result["result"] = simulateRoundRobinConsensus(activeAgents, topic)
	case "ranked":
		result["result"] = simulateRankedConsensus(activeAgents, topic)
	}

	return result
}

func (n *AgentOrchestratorNode) handleList(params map[string]string) map[string]interface{} {
	agentDiscoveryMu.RLock()
	defer agentDiscoveryMu.RUnlock()

	agents := make([]*AgentCapability, 0, len(agentDiscoveryRegistry))
	for _, agent := range agentDiscoveryRegistry {
		agents = append(agents, agent)
	}

	return map[string]interface{}{
		"status":       "completed",
		"total_agents": len(agents),
		"agents":       agents,
	}
}

func (n *AgentOrchestratorNode) handleStatus(params map[string]string) map[string]interface{} {
	agentID := getParam(params, "agent_id", "")
	if agentID == "" {
		return map[string]interface{}{"status": "error", "error": "agent_id is required"}
	}

	agentDiscoveryMu.RLock()
	defer agentDiscoveryMu.RUnlock()

	agent, ok := agentDiscoveryRegistry[agentID]
	if !ok {
		return map[string]interface{}{"status": "error", "error": fmt.Sprintf("agent %s not found", agentID)}
	}

	return map[string]interface{}{
		"status": "completed",
		"agent":  agent,
	}
}

// Consensus simulation functions

func simulateMajorityConsensus(agents []*AgentCapability, topic string) map[string]interface{} {
	votes := make(map[string]int)
	for _, agent := range agents {
		// Simulate vote based on capability match
		vote := simulateAgentVote(agent, topic)
		votes[vote]++
	}

	winner := ""
	maxVotes := 0
	for option, count := range votes {
		if count > maxVotes {
			maxVotes = count
			winner = option
		}
	}

	return map[string]interface{}{
		"method":  "majority",
		"winner":  winner,
		"votes":   votes,
		"total":   len(agents),
		"quorum":  maxVotes > len(agents)/2,
	}
}

func simulateWeightedConsensus(agents []*AgentCapability, topic string) map[string]interface{} {
	type weightedVote struct {
		AgentID string  `json:"agent_id"`
		Vote    string  `json:"vote"`
		Weight  float64 `json:"weight"`
	}

	weightedVotes := make([]weightedVote, 0, len(agents))
	scores := make(map[string]float64)

	for _, agent := range agents {
		vote := simulateAgentVote(agent, topic)
		weight := agent.Confidence
		if weight <= 0 {
			weight = 0.5
		}
		weightedVotes = append(weightedVotes, weightedVote{
			AgentID: agent.AgentID,
			Vote:    vote,
			Weight:  weight,
		})
		scores[vote] += weight
	}

	winner := ""
	maxScore := 0.0
	for option, score := range scores {
		if score > maxScore {
			maxScore = score
			winner = option
		}
	}

	return map[string]interface{}{
		"method":   "weighted",
		"winner":   winner,
		"votes":    weightedVotes,
		"scores":   scores,
		"total":    len(agents),
	}
}

func simulateUnanimousConsensus(agents []*AgentCapability, topic string) map[string]interface{} {
	votes := make(map[string]int)
	for _, agent := range agents {
		vote := simulateAgentVote(agent, topic)
		votes[vote]++
	}

	unanimous := len(votes) == 1
	result := "deadlock"
	if unanimous {
		for option := range votes {
			result = option
			break
		}
	}

	return map[string]interface{}{
		"method":    "unanimous",
		"result":    result,
		"unanimous": unanimous,
		"votes":     votes,
		"total":     len(agents),
	}
}

func simulateRoundRobinConsensus(agents []*AgentCapability, topic string) map[string]interface{} {
	rounds := make([]map[string]interface{}, 0, len(agents))
	iterations := 0

	for _, agent := range agents {
		vote := simulateAgentVote(agent, topic)
		iterations++
		rounds = append(rounds, map[string]interface{}{
			"agent":  agent.AgentID,
			"round":  iterations,
			"vote":   vote,
		})
	}

	// Final round: aggregate
	votes := make(map[string]int)
	for _, r := range rounds {
		votes[r["vote"].(string)]++
	}
	winner := ""
	maxVotes := 0
	for option, count := range votes {
		if count > maxVotes {
			maxVotes = count
			winner = option
		}
	}

	return map[string]interface{}{
		"method":     "round_robin",
		"winner":     winner,
		"rounds":     rounds,
		"iterations": iterations,
		"total":      len(agents),
	}
}

func simulateRankedConsensus(agents []*AgentCapability, topic string) map[string]interface{} {
	type rankedVote struct {
		AgentID string   `json:"agent_id"`
		Ranking []string `json:"ranking"`
	}

	rankedVotes := make([]rankedVote, 0, len(agents))
	scores := make(map[string]float64)

	for _, agent := range agents {
		options := []string{"approve", "reject", "defer"}
		// Score: first choice gets 3, second 2, third 1
		for i, option := range options {
			score := float64(len(options) - i)
			// Apply confidence to score
			confidence := agent.Confidence
			if confidence <= 0 {
				confidence = 0.5
			}
			scores[option] += score * confidence
		}

		rankedVotes = append(rankedVotes, rankedVote{
			AgentID: agent.AgentID,
			Ranking: options,
		})
	}

	// Find winner by Borda count
	type scorePair struct {
		option string
		score  float64
	}
	pairs := make([]scorePair, 0, len(scores))
	for option, score := range scores {
		pairs = append(pairs, scorePair{option, score})
	}
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].score > pairs[j].score
	})

	return map[string]interface{}{
		"method":  "ranked",
		"winner":  pairs[0].option,
		"ranking": pairs,
		"votes":   rankedVotes,
		"total":   len(agents),
	}
}

func simulateAgentVote(agent *AgentCapability, topic string) string {
	lowerTopic := strings.ToLower(topic)

	// Check if agent's capabilities match the topic
	relevance := 0
	for _, cap := range agent.Capabilities {
		if strings.Contains(lowerTopic, strings.ToLower(cap)) {
			relevance++
		}
	}

	// High relevance -> approve, low -> reject, medium -> defer
	switch {
	case relevance >= 2:
		return "approve"
	case relevance == 1:
		return "defer"
	default:
		// If no capability match, randomly approve/reject based on confidence
		if agent.Confidence > 0.7 {
			return "approve"
		}
		return "reject"
	}
}

func cleanupStaleAgents() {
	agentDiscoveryMu.Lock()
	defer agentDiscoveryMu.Unlock()

	now := time.Now()
	cutoff := now.Add(-30 * time.Minute)

	for id, agent := range agentDiscoveryRegistry {
		lastSeen, err := time.Parse(time.RFC3339, agent.LastSeen)
		if err != nil || lastSeen.Before(cutoff) {
			agent.Status = "offline"
			agentDiscoveryRegistry[id] = agent
		}
	}

	// Full cleanup of long-offline agents (1 hour)
	cutoffFull := now.Add(-1 * time.Hour)
	for id, agent := range agentDiscoveryRegistry {
		lastSeen, err := time.Parse(time.RFC3339, agent.LastSeen)
		if err != nil || lastSeen.Before(cutoffFull) {
			delete(agentDiscoveryRegistry, id)
		}
	}
}

func init() {
	// Pre-register built-in agents
	agentDiscoveryMu.Lock()
	now := time.Now().UTC().Format(time.RFC3339)
	builtinAgents := []*AgentCapability{
		{AgentID: "planner", AgentName: "Planner Agent", Capabilities: []string{"planning", "decomposition", "strategy"}, Confidence: 0.95, RegisteredAt: now, LastSeen: now, Status: "active"},
		{AgentID: "researcher", AgentName: "Researcher Agent", Capabilities: []string{"research", "search", "analysis", "information"}, Confidence: 0.90, RegisteredAt: now, LastSeen: now, Status: "active"},
		{AgentID: "critic", AgentName: "Critic Agent", Capabilities: []string{"review", "critique", "evaluation", "quality"}, Confidence: 0.85, RegisteredAt: now, LastSeen: now, Status: "active"},
		{AgentID: "code_review", AgentName: "Code Review Agent", Capabilities: []string{"code", "review", "security", "quality"}, Confidence: 0.90, RegisteredAt: now, LastSeen: now, Status: "active"},
		{AgentID: "evaluator", AgentName: "Evaluator Agent", Capabilities: []string{"evaluation", "scoring", "benchmark", "metrics"}, Confidence: 0.88, RegisteredAt: now, LastSeen: now, Status: "active"},
		{AgentID: "reflector", AgentName: "Reflector Agent", Capabilities: []string{"reflection", "improvement", "learning", "feedback"}, Confidence: 0.82, RegisteredAt: now, LastSeen: now, Status: "active"},
		{AgentID: "financial_expert", AgentName: "Financial Expert", Capabilities: []string{"finance", "risk", "compliance", "audit", "trading"}, Confidence: 0.92, RegisteredAt: now, LastSeen: now, Status: "active"},
		{AgentID: "legal_expert", AgentName: "Legal Expert", Capabilities: []string{"legal", "compliance", "regulation", "contract", "policy"}, Confidence: 0.93, RegisteredAt: now, LastSeen: now, Status: "active"},
	}
	for _, agent := range builtinAgents {
		agentDiscoveryRegistry[agent.AgentID] = agent
	}
	agentDiscoveryMu.Unlock()
}