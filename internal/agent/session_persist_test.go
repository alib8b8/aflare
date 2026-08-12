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

// session_persist_test.go covers P1 test gap:
//   - Session persistence round-trip: write → "restart" → restore → verify consistency

package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/alib8b8/aflare/internal/nodes/core"
)

// TestSessionPersistence_RoundTrip tests the full save → load → restore cycle.
// Verifies that messages, metadata, and state are preserved correctly.
func TestSessionPersistence_RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	sessionPath := filepath.Join(tmpDir, "test_session.json")

	// 1. Create a context manager with messages
	cm := NewContextManager()
	cm.SetProvider("openai")
	cm.SetSystemPrompt("You are a helpful assistant.")

	cm.AddUser("Hello, my name is Alice")
	cm.AddAssistant("Hi Alice! How can I help you today?")
	cm.AddUser("What's the weather like?")
	cm.AddAssistant("I don't have access to real-time weather data.")

	originalCount := cm.MessageCount()
	if originalCount != 4 {
		t.Fatalf("expected 4 messages, got %d", originalCount)
	}

	// 2. Save session
	if err := cm.SaveSession(sessionPath); err != nil {
		t.Fatalf("SaveSession failed: %v", err)
	}

	// 3. Verify file exists
	if _, err := os.Stat(sessionPath); err != nil {
		t.Fatalf("session file not created: %v", err)
	}

	// 4. Load session (simulating a restart)
	session, ok := LoadSession(sessionPath)
	if !ok {
		t.Fatal("LoadSession returned false")
	}
	// MessageCnt includes system prompt (1 extra)
	if session.MessageCnt != originalCount+1 {
		t.Errorf("expected message count %d, got %d", originalCount+1, session.MessageCnt)
	}
	if len(session.Messages) != originalCount+1 {
		t.Errorf("expected %d messages, got %d", originalCount+1, len(session.Messages))
	}

	// 5. Verify message content (first message is system prompt)
	if session.Messages[0].Role != "system" {
		t.Errorf("expected first message role 'system', got %q", session.Messages[0].Role)
	}
	if session.Messages[1].Role != "user" {
		t.Errorf("expected second message role 'user', got %q", session.Messages[1].Role)
	}
	if session.Messages[1].Content != "Hello, my name is Alice" {
		t.Errorf("second message content mismatch: %q", session.Messages[1].Content)
	}

	// 6. Restore into a new context manager (simulating fresh start)
	cm2 := NewContextManager()
	restored := cm2.RestoreSession(sessionPath)
	if restored != originalCount+1 {
		t.Errorf("expected %d restored messages (including system), got %d", originalCount+1, restored)
	}
	restoredMsgs := cm2.Messages()
	if len(restoredMsgs) != originalCount {
		t.Errorf("restored %d messages, expected %d", len(restoredMsgs), originalCount)
	}
}

// TestSessionPersistence_EmptySession tests save/load of empty sessions.
func TestSessionPersistence_EmptySession(t *testing.T) {
	tmpDir := t.TempDir()
	sessionPath := filepath.Join(tmpDir, "empty_session.json")

	cm := NewContextManager()
	if cm.MessageCount() != 0 {
		t.Fatal("expected empty context manager")
	}

	// Save empty session should not create file
	if err := cm.SaveSession(sessionPath); err != nil {
		t.Fatalf("SaveSession should not error on empty: %v", err)
	}

	// Loading non-existent file
	_, ok := LoadSession(filepath.Join(tmpDir, "nonexistent.json"))
	if ok {
		t.Error("LoadSession should return false for non-existent file")
	}
}

// TestSessionPersistence_CorruptedFile verifies loading a corrupted file.
func TestSessionPersistence_CorruptedFile(t *testing.T) {
	tmpDir := t.TempDir()
	sessionPath := filepath.Join(tmpDir, "corrupted.json")

	if err := os.WriteFile(sessionPath, []byte("not valid json"), 0600); err != nil {
		t.Fatalf("failed to write corrupted file: %v", err)
	}

	_, ok := LoadSession(sessionPath)
	if ok {
		t.Error("LoadSession should return false for corrupted file")
	}
}

// TestSessionPersistence_SystemMessagePreserved verifies system prompt persistence.
func TestSessionPersistence_SystemMessagePreserved(t *testing.T) {
	tmpDir := t.TempDir()
	sessionPath := filepath.Join(tmpDir, "system_msg_session.json")

	cm := NewContextManager()
	systemPrompt := "You are a specialized coding assistant."
	cm.SetSystemPrompt(systemPrompt)
	cm.AddUser("hello")
	cm.AddAssistant("hi")

	if err := cm.SaveSession(sessionPath); err != nil {
		t.Fatalf("SaveSession failed: %v", err)
	}

	session, ok := LoadSession(sessionPath)
	if !ok {
		t.Fatal("LoadSession failed")
	}

	hasSystem := false
	for _, msg := range session.Messages {
		if msg.Role == "system" && msg.Content == systemPrompt {
			hasSystem = true
			break
		}
	}
	if !hasSystem {
		t.Error("system prompt not found in persisted session")
	}
}

// TestSessionPersistence_LargeSession tests persistence with many messages.
func TestSessionPersistence_LargeSession(t *testing.T) {
	tmpDir := t.TempDir()
	sessionPath := filepath.Join(tmpDir, "large_session.json")

	cm := NewContextManager()
	cm.SetProvider("openai")

	for i := 0; i < 50; i++ {
		cm.AddUser("User message " + string(rune('0'+i%10)))
		cm.AddAssistant("Assistant response " + string(rune('0'+i%10)))
	}

	if err := cm.SaveSession(sessionPath); err != nil {
		t.Fatalf("SaveSession failed: %v", err)
	}

	session, ok := LoadSession(sessionPath)
	if !ok {
		t.Fatal("LoadSession failed")
	}
	if len(session.Messages) != 100 {
		t.Errorf("expected 100 messages, got %d", len(session.Messages))
	}
}

// TestSessionPersistence_DeleteSession verifies DeleteSession removes the file.
func TestSessionPersistence_DeleteSession(t *testing.T) {
	tmpDir := t.TempDir()
	sessionPath := filepath.Join(tmpDir, "to_delete.json")

	cm := NewContextManager()
	cm.AddUser("test")
	cm.AddAssistant("response")

	if err := cm.SaveSession(sessionPath); err != nil {
		t.Fatalf("SaveSession failed: %v", err)
	}

	DeleteSession(sessionPath)

	if _, err := os.Stat(sessionPath); err == nil {
		t.Error("session file should be deleted")
	}
}

// TestSessionPersistence_LockFile tests lock file cleanup.
func TestSessionPersistence_LockFile(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "test_session.json.lock")

	if err := os.WriteFile(lockPath, []byte("locked"), 0600); err != nil {
		t.Fatalf("failed to create lock file: %v", err)
	}

	DeleteSession(lockPath)

	if _, err := os.Stat(lockPath); err == nil {
		t.Error("lock file should be deleted")
	}
}

// TestSessionPersistence_RestoreEmptySession tests restoring empty session.
func TestSessionPersistence_RestoreEmptySession(t *testing.T) {
	tmpDir := t.TempDir()
	sessionPath := filepath.Join(tmpDir, "empty.json")

	session := SessionData{
		Messages:   []core.LLMMessage{},
		MessageCnt: 0,
	}
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if err := os.WriteFile(sessionPath, data, 0600); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	cm := NewContextManager()
	restored := cm.RestoreSession(sessionPath)
	if restored != 0 {
		t.Errorf("expected 0 restored messages, got %d", restored)
	}
}
