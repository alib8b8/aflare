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

package nodes

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"
)

type SwarmMessage struct {
	ID        string
	From      string
	To        string
	Channel   string
	Content   string
	Type      string
	Timestamp time.Time
	Signature string
	ParentID  string
}

type SwarmAgent struct {
	ID         string
	Name       string
	Role       string
	Status     string
	JoinedAt   time.Time
	LastActive time.Time
	Skills     []string
}

type SwarmChannel struct {
	Name        string
	Description string
	Members     []string
	Messages    []SwarmMessage
	CreatedAt   time.Time
}

type SwarmCommNode struct{}

func init() {
	Register(&SwarmCommNode{})
}

var (
	swarmAgents   = map[string]*SwarmAgent{}
	swarmChannels = map[string]*SwarmChannel{}
	swarmMessages = []SwarmMessage{}
	swarmMu       sync.RWMutex
	swarmInited   bool
)

func initSwarm() {
	swarmMu.Lock()
	defer swarmMu.Unlock()

	if swarmInited {
		return
	}
	swarmInited = true

	swarmChannels["general"] = &SwarmChannel{
		Name:        "general",
		Description: "General discussion and coordination channel",
		CreatedAt:   time.Now(),
	}
	swarmChannels["tasks"] = &SwarmChannel{
		Name:        "tasks",
		Description: "Task assignment and status updates",
		CreatedAt:   time.Now(),
	}
	swarmChannels["research"] = &SwarmChannel{
		Name:        "research",
		Description: "Research findings and knowledge sharing",
		CreatedAt:   time.Now(),
	}
	swarmChannels["announcements"] = &SwarmChannel{
		Name:        "announcements",
		Description: "Important announcements and broadcasts",
		CreatedAt:   time.Now(),
	}
}

func (n *SwarmCommNode) Name() string {
	return "swarm_comm"
}

func (n *SwarmCommNode) Description() string {
	return "Multi-agent swarm communication: Nostr-style protocol, shared collaboration spaces (block/buzz inspired)"
}

func (n *SwarmCommNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "swarm_comm",
		Description: "Decentralized multi-agent swarm communication system with channels, agent registration, and message broadcasting. Inspired by block/buzz (Nostr protocol) for human-AI collaborative spaces.",
		Input:       "string - message content or command parameters",
		Output:      "string - communication results, channel messages, or agent status",
		Params: []ParamSchema{
			{Name: "action", Type: "string", Description: "Action: join|leave|send|read|list_channels|list_agents|create_channel|broadcast (default: read)", Required: false, Default: "read"},
			{Name: "agent_id", Type: "string", Description: "Agent identifier for join/send actions", Required: false},
			{Name: "agent_name", Type: "string", Description: "Agent display name for registration", Required: false},
			{Name: "agent_role", Type: "string", Description: "Agent role: researcher|developer|coordinator|reviewer|custom (default: agent)", Required: false, Default: "agent"},
			{Name: "channel", Type: "string", Description: "Target channel: general|tasks|research|announcements (default: general)", Required: false, Default: "general"},
			{Name: "message_type", Type: "string", Description: "Message type: text|task|result|status|emergency (default: text)", Required: false, Default: "text"},
			{Name: "to_agent", Type: "string", Description: "Direct message target agent ID (optional)", Required: false},
			{Name: "limit", Type: "string", Description: "Maximum messages to retrieve (default: 50)", Required: false, Default: "50"},
		},
	}
}

func (n *SwarmCommNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	initSwarm()

	action := getParam(params, "action", "read")
	agentID := getParam(params, "agent_id", "")
	channel := getParam(params, "channel", "general")
	limitStr := getParam(params, "limit", "50")

	limit := 50
	if _, err := fmt.Sscanf(limitStr, "%d", &limit); err != nil {
		limit = 50
	}
	if limit < 1 {
		limit = 50
	}

	switch action {
	case "join":
		return n.actionJoin(agentID, params)
	case "leave":
		return n.actionLeave(agentID)
	case "send":
		return n.actionSend(agentID, channel, input, params)
	case "broadcast":
		return n.actionBroadcast(agentID, input, params)
	case "read":
		return n.actionRead(channel, agentID, limit)
	case "list_channels":
		return n.actionListChannels()
	case "list_agents":
		return n.actionListAgents()
	case "create_channel":
		return n.actionCreateChannel(channel, input)
	default:
		return "", fmt.Errorf("unknown swarm action: %s", action)
	}
}

func (n *SwarmCommNode) actionJoin(agentID string, params map[string]string) (string, error) {
	if agentID == "" {
		return "", fmt.Errorf("agent_id is required to join the swarm")
	}

	agentName := getParam(params, "agent_name", agentID)
	agentRole := getParam(params, "agent_role", "agent")

	swarmMu.Lock()
	defer swarmMu.Unlock()

	if _, exists := swarmAgents[agentID]; exists {
		swarmAgents[agentID].LastActive = time.Now()
		swarmAgents[agentID].Status = "active"
		return fmt.Sprintf("## 🐝 Agent Rejoined\n\nAgent **%s** (%s) is now active in the swarm.\n", agentName, agentID), nil
	}

	swarmAgents[agentID] = &SwarmAgent{
		ID:         agentID,
		Name:       agentName,
		Role:       agentRole,
		Status:     "active",
		JoinedAt:   time.Now(),
		LastActive: time.Now(),
	}

	for _, ch := range swarmChannels {
		ch.Members = append(ch.Members, agentID)
	}

	return fmt.Sprintf("## 🐝 Agent Joined Swarm\n\nWelcome, **%s** (%s)!\n\n- **Role**: %s\n- **Joined**: %s\n- **Channels**: general, tasks, research, announcements\n\nTotal agents in swarm: %d\n",
		agentName, agentID, agentRole, time.Now().Format("2006-01-02 15:04:05"), len(swarmAgents)), nil
}

func (n *SwarmCommNode) actionLeave(agentID string) (string, error) {
	if agentID == "" {
		return "", fmt.Errorf("agent_id is required to leave the swarm")
	}

	swarmMu.Lock()
	defer swarmMu.Unlock()

	agent, exists := swarmAgents[agentID]
	if !exists {
		return fmt.Sprintf("Agent %s not found in swarm", agentID), nil
	}

	agent.Status = "offline"
	agent.LastActive = time.Now()

	return fmt.Sprintf("## 👋 Agent Left Swarm\n\n**%s** (%s) has left the swarm.\n", agent.Name, agentID), nil
}

func (n *SwarmCommNode) actionSend(agentID, channel, content string, params map[string]string) (string, error) {
	if agentID == "" {
		return "", fmt.Errorf("agent_id is required to send messages")
	}
	if strings.TrimSpace(content) == "" {
		return "", fmt.Errorf("message content cannot be empty")
	}

	toAgent := getParam(params, "to_agent", "")
	msgType := getParam(params, "message_type", "text")

	swarmMu.Lock()
	defer swarmMu.Unlock()

	if _, exists := swarmAgents[agentID]; !exists {
		return "", fmt.Errorf("agent %s not registered. Join the swarm first with action=join", agentID)
	}

	ch, exists := swarmChannels[channel]
	if !exists {
		return "", fmt.Errorf("channel %s does not exist", channel)
	}

	msgID := generateMessageID(agentID, content)
	msg := SwarmMessage{
		ID:        msgID,
		From:      agentID,
		To:        toAgent,
		Channel:   channel,
		Content:   content,
		Type:      msgType,
		Timestamp: time.Now(),
		Signature: signMessage(msgID, content),
	}

	ch.Messages = append(ch.Messages, msg)
	swarmMessages = append(swarmMessages, msg)
	swarmAgents[agentID].LastActive = time.Now()

	direct := ""
	if toAgent != "" {
		direct = fmt.Sprintf(" → @%s", toAgent)
	}
	return fmt.Sprintf("## ✉️ Message Sent\n\n**From**: %s\n**Channel**: #%s\n**Type**: %s\n%s\n**Content**: %s\n",
		agentID, channel, msgType, direct, content), nil
}

func (n *SwarmCommNode) actionBroadcast(agentID, content string, params map[string]string) (string, error) {
	if agentID == "" {
		return "", fmt.Errorf("agent_id is required for broadcasting")
	}

	msgType := getParam(params, "message_type", "announcement")

	swarmMu.Lock()
	defer swarmMu.Unlock()

	ch := swarmChannels["announcements"]
	msgID := generateMessageID(agentID, content)
	msg := SwarmMessage{
		ID:        msgID,
		From:      agentID,
		Channel:   "announcements",
		Content:   content,
		Type:      msgType,
		Timestamp: time.Now(),
		Signature: signMessage(msgID, content),
	}
	ch.Messages = append(ch.Messages, msg)
	swarmMessages = append(swarmMessages, msg)

	return fmt.Sprintf("## 📢 Broadcast Sent\n\n**From**: %s\n**Channel**: #announcements\n**Type**: %s\n\n%s\n\nDelivered to %d active agents.\n",
		agentID, msgType, content, countActiveAgents()), nil
}

func (n *SwarmCommNode) actionRead(channel, agentID string, limit int) (string, error) {
	swarmMu.RLock()
	defer swarmMu.RUnlock()

	ch, exists := swarmChannels[channel]
	if !exists {
		return "", fmt.Errorf("channel %s does not exist", channel)
	}

	msgs := ch.Messages
	if len(msgs) > limit {
		msgs = msgs[len(msgs)-limit:]
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## 📬 Channel: #%s\n\n", channel))
	sb.WriteString(fmt.Sprintf("%s\n\n", ch.Description))
	sb.WriteString(fmt.Sprintf("**Members**: %d | **Messages**: %d\n\n", len(ch.Members), len(ch.Messages)))

	if len(msgs) == 0 {
		sb.WriteString("_No messages yet. Start the conversation!_\n")
		return sb.String(), nil
	}

	for i, msg := range msgs {
		fromName := msg.From
		if agent, ok := swarmAgents[msg.From]; ok {
			fromName = agent.Name
		}
		sb.WriteString(fmt.Sprintf("%d. **%s** [%s] _(%s)_\n", i+1, fromName, msg.Type, msg.Timestamp.Format("15:04:05")))
		if msg.To != "" {
			sb.WriteString(fmt.Sprintf("   _DM to @%s_\n", msg.To))
		}
		sb.WriteString(fmt.Sprintf("   %s\n\n", msg.Content))
	}

	return sb.String(), nil
}

func (n *SwarmCommNode) actionListChannels() (string, error) {
	swarmMu.RLock()
	defer swarmMu.RUnlock()

	var sb strings.Builder
	sb.WriteString("## 🏷️ Swarm Channels\n\n")
	for name, ch := range swarmChannels {
		sb.WriteString(fmt.Sprintf("- **#%s** - %s\n", name, ch.Description))
		sb.WriteString(fmt.Sprintf("  Members: %d | Messages: %d\n", len(ch.Members), len(ch.Messages)))
	}
	sb.WriteString(fmt.Sprintf("\nTotal: %d channels\n", len(swarmChannels)))
	return sb.String(), nil
}

func (n *SwarmCommNode) actionListAgents() (string, error) {
	swarmMu.RLock()
	defer swarmMu.RUnlock()

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## 👥 Swarm Agents (%d total)\n\n", len(swarmAgents)))

	i := 1
	for _, agent := range swarmAgents {
		statusEmoji := "🟢"
		if agent.Status == "offline" {
			statusEmoji = "🔴"
		} else if agent.Status == "busy" {
			statusEmoji = "🟡"
		}
		sb.WriteString(fmt.Sprintf("%d. %s **%s** _(%s)_\n", i, statusEmoji, agent.Name, agent.ID))
		sb.WriteString(fmt.Sprintf("   Role: %s | Status: %s\n", agent.Role, agent.Status))
		sb.WriteString(fmt.Sprintf("   Last active: %s\n\n", agent.LastActive.Format("2006-01-02 15:04:05")))
		i++
	}
	return sb.String(), nil
}

func (n *SwarmCommNode) actionCreateChannel(name, description string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("channel name is required")
	}

	swarmMu.Lock()
	defer swarmMu.Unlock()

	if _, exists := swarmChannels[name]; exists {
		return fmt.Sprintf("Channel #%s already exists", name), nil
	}

	swarmChannels[name] = &SwarmChannel{
		Name:        name,
		Description: description,
		CreatedAt:   time.Now(),
	}

	return fmt.Sprintf("## ✅ Channel Created\n\n**#%s** is now available.\n\n%s\n", name, description), nil
}

func generateMessageID(agentID, content string) string {
	h := sha256.New()
	h.Write([]byte(agentID + content + time.Now().String()))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func signMessage(msgID, content string) string {
	h := sha256.New()
	h.Write([]byte(msgID + ":" + content))
	return hex.EncodeToString(h.Sum(nil))[:12]
}

func countActiveAgents() int {
	count := 0
	for _, a := range swarmAgents {
		if a.Status == "active" {
			count++
		}
	}
	return count
}
