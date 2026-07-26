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
	"strings"
	"testing"
)

// removeTestAgent schedules cleanup of a test agent from the global swarmAgents
// map so tests using shared swarm state do not leak agents into other tests.
func removeTestAgent(t *testing.T, agentID string) {
	t.Helper()
	t.Cleanup(func() {
		swarmMu.Lock()
		delete(swarmAgents, agentID)
		swarmMu.Unlock()
	})
}

// removeTestChannel schedules cleanup of a test channel from the global
// swarmChannels map.
func removeTestChannel(t *testing.T, name string) {
	t.Helper()
	t.Cleanup(func() {
		swarmMu.Lock()
		delete(swarmChannels, name)
		swarmMu.Unlock()
	})
}

func TestSwarmComm_ListChannels(t *testing.T) {
	node, ok := Get("swarm_comm")
	if !ok {
		t.Fatal("swarm_comm not found in registry")
	}

	ctx := context.Background()

	output, err := node.Execute(ctx, "", map[string]string{"action": "list_channels"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "Swarm Channels") {
		t.Errorf("expected swarm channels header, got: %s", output)
	}
	// The default channels created by initSwarm should be listed.
	for _, ch := range []string{"general", "tasks", "research", "announcements"} {
		if !strings.Contains(output, ch) {
			t.Errorf("expected channel %q in output, got: %s", ch, output)
		}
	}
}

func TestSwarmComm_JoinAndListAgents(t *testing.T) {
	node, ok := Get("swarm_comm")
	if !ok {
		t.Fatal("swarm_comm not found in registry")
	}

	ctx := context.Background()

	agentID := "test-join-agent"
	output, err := node.Execute(ctx, "", map[string]string{
		"action":     "join",
		"agent_id":   agentID,
		"agent_name": "TestJoinAgent",
		"agent_role": "developer",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "Agent Joined Swarm") {
		t.Errorf("expected join confirmation, got: %s", output)
	}
	removeTestAgent(t, agentID)

	// list_agents should now include our agent.
	listOutput, err := node.Execute(ctx, "", map[string]string{"action": "list_agents"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(listOutput, "Swarm Agents") {
		t.Errorf("expected swarm agents header, got: %s", listOutput)
	}
	if !strings.Contains(listOutput, "TestJoinAgent") {
		t.Errorf("expected agent name in list, got: %s", listOutput)
	}
}

func TestSwarmComm_JoinEmptyAgentID(t *testing.T) {
	node, ok := Get("swarm_comm")
	if !ok {
		t.Fatal("swarm_comm not found in registry")
	}

	ctx := context.Background()

	_, err := node.Execute(ctx, "", map[string]string{
		"action": "join",
		// agent_id intentionally omitted
	})
	if err == nil {
		t.Error("expected error for empty agent_id on join")
	}
}

func TestSwarmComm_SendAfterJoin(t *testing.T) {
	node, ok := Get("swarm_comm")
	if !ok {
		t.Fatal("swarm_comm not found in registry")
	}

	ctx := context.Background()

	agentID := "test-send-agent"
	_, err := node.Execute(ctx, "", map[string]string{
		"action":     "join",
		"agent_id":   agentID,
		"agent_name": "TestSendAgent",
	})
	if err != nil {
		t.Fatalf("join failed: %v", err)
	}
	removeTestAgent(t, agentID)

	output, err := node.Execute(ctx, "hello world message", map[string]string{
		"action":   "send",
		"agent_id": agentID,
		"channel":  "general",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "Message Sent") {
		t.Errorf("expected 'Message Sent' header, got: %s", output)
	}
	if !strings.Contains(output, "hello world message") {
		t.Errorf("expected message content in output, got: %s", output)
	}
}

func TestSwarmComm_SendWithoutJoin(t *testing.T) {
	node, ok := Get("swarm_comm")
	if !ok {
		t.Fatal("swarm_comm not found in registry")
	}

	ctx := context.Background()

	_, err := node.Execute(ctx, "hello", map[string]string{
		"action":   "send",
		"agent_id": "nonexistent-agent-xyz-12345",
		"channel":  "general",
	})
	if err == nil {
		t.Error("expected error for send without prior join")
	}
}

func TestSwarmComm_SendEmptyContent(t *testing.T) {
	node, ok := Get("swarm_comm")
	if !ok {
		t.Fatal("swarm_comm not found in registry")
	}

	ctx := context.Background()

	agentID := "test-send-empty-agent"
	_, _ = node.Execute(ctx, "", map[string]string{
		"action":     "join",
		"agent_id":   agentID,
		"agent_name": "TestSendEmptyAgent",
	})
	removeTestAgent(t, agentID)

	_, err := node.Execute(ctx, "   ", map[string]string{
		"action":   "send",
		"agent_id": agentID,
		"channel":  "general",
	})
	if err == nil {
		t.Error("expected error for empty message content")
	}
}

func TestSwarmComm_Read(t *testing.T) {
	node, ok := Get("swarm_comm")
	if !ok {
		t.Fatal("swarm_comm not found in registry")
	}

	ctx := context.Background()

	output, err := node.Execute(ctx, "", map[string]string{
		"action":  "read",
		"channel": "general",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "Channel: #general") {
		t.Errorf("expected channel header, got: %s", output)
	}
}

func TestSwarmComm_Broadcast(t *testing.T) {
	node, ok := Get("swarm_comm")
	if !ok {
		t.Fatal("swarm_comm not found in registry")
	}

	ctx := context.Background()

	agentID := "test-broadcast-agent"
	_, err := node.Execute(ctx, "", map[string]string{
		"action":     "join",
		"agent_id":   agentID,
		"agent_name": "TestBroadcastAgent",
	})
	if err != nil {
		t.Fatalf("join failed: %v", err)
	}
	removeTestAgent(t, agentID)

	output, err := node.Execute(ctx, "important announcement text", map[string]string{
		"action":   "broadcast",
		"agent_id": agentID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "Broadcast Sent") {
		t.Errorf("expected 'Broadcast Sent' header, got: %s", output)
	}
	if !strings.Contains(output, "important announcement text") {
		t.Errorf("expected broadcast content, got: %s", output)
	}
}

func TestSwarmComm_BroadcastEmptyAgentID(t *testing.T) {
	node, ok := Get("swarm_comm")
	if !ok {
		t.Fatal("swarm_comm not found in registry")
	}

	ctx := context.Background()

	_, err := node.Execute(ctx, "msg", map[string]string{
		"action": "broadcast",
		// agent_id intentionally omitted
	})
	if err == nil {
		t.Error("expected error for broadcast without agent_id")
	}
}

func TestSwarmComm_ListAgents(t *testing.T) {
	node, ok := Get("swarm_comm")
	if !ok {
		t.Fatal("swarm_comm not found in registry")
	}

	ctx := context.Background()

	output, err := node.Execute(ctx, "", map[string]string{"action": "list_agents"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "Swarm Agents") {
		t.Errorf("expected swarm agents header, got: %s", output)
	}
}

func TestSwarmComm_CreateChannel(t *testing.T) {
	node, ok := Get("swarm_comm")
	if !ok {
		t.Fatal("swarm_comm not found in registry")
	}

	ctx := context.Background()

	channelName := "test-channel-xyz-12345"
	output, err := node.Execute(ctx, "test channel description", map[string]string{
		"action":  "create_channel",
		"channel": channelName,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "Channel Created") {
		t.Errorf("expected 'Channel Created' header, got: %s", output)
	}
	removeTestChannel(t, channelName)

	// Verify the new channel now appears in list_channels.
	listOutput, err := node.Execute(ctx, "", map[string]string{"action": "list_channels"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(listOutput, channelName) {
		t.Errorf("expected new channel in list, got: %s", listOutput)
	}
}

func TestSwarmComm_CreateChannelEmptyName(t *testing.T) {
	node, ok := Get("swarm_comm")
	if !ok {
		t.Fatal("swarm_comm not found in registry")
	}

	ctx := context.Background()

	// With no channel param the default is "general", which already exists,
	// so the action returns a soft "already exists" message rather than an
	// error. Use action=create_channel with an empty channel override by
	// passing channel="" explicitly: getParam treats empty as default, so we
	// must check the action's own validation path differently. Instead, pass
	// an explicit action that hits the create_channel code path with the
	// default channel name to confirm the "already exists" branch.
	output, err := node.Execute(ctx, "description", map[string]string{
		"action":  "create_channel",
		"channel": "general",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "already exists") {
		t.Errorf("expected 'already exists' for general channel, got: %s", output)
	}
}

func TestSwarmComm_Leave(t *testing.T) {
	node, ok := Get("swarm_comm")
	if !ok {
		t.Fatal("swarm_comm not found in registry")
	}

	ctx := context.Background()

	agentID := "test-leave-agent"
	_, err := node.Execute(ctx, "", map[string]string{
		"action":     "join",
		"agent_id":   agentID,
		"agent_name": "TestLeaveAgent",
	})
	if err != nil {
		t.Fatalf("join failed: %v", err)
	}
	removeTestAgent(t, agentID)

	output, err := node.Execute(ctx, "", map[string]string{
		"action":   "leave",
		"agent_id": agentID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "Left Swarm") {
		t.Errorf("expected 'Left Swarm' header, got: %s", output)
	}
}

func TestSwarmComm_LeaveUnknownAgent(t *testing.T) {
	node, ok := Get("swarm_comm")
	if !ok {
		t.Fatal("swarm_comm not found in registry")
	}

	ctx := context.Background()

	// Leaving with an unregistered agent should not error; it returns a
	// soft "not found" message instead.
	output, err := node.Execute(ctx, "", map[string]string{
		"action":   "leave",
		"agent_id": "never-joined-agent-xyz-98765",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "not found") {
		t.Errorf("expected 'not found' message for unknown agent, got: %s", output)
	}
}

func TestSwarmComm_UnknownAction(t *testing.T) {
	node, ok := Get("swarm_comm")
	if !ok {
		t.Fatal("swarm_comm not found in registry")
	}

	ctx := context.Background()

	_, err := node.Execute(ctx, "", map[string]string{"action": "unknown_action"})
	if err == nil {
		t.Error("expected error for unknown swarm action")
	}
}
