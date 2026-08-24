// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌​​‌‌​​‌‌​​‌‌​‌​​‌​​‌​‌‌‌​‌‌​​​​​‌‌‌​​​​​​‌​‌​​​​​​​​​​​​​​​​​​​‌​‌‌‌‌‌​‌​​​‌‌​‌⁠
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

package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/alib8b8/aflare/internal/meta"
	"github.com/alib8b8/aflare/internal/nodes/core"
)

// SessionFileName is the name of the file that stores the chat session.
const SessionFileName = "chat_session.json"

// SessionData is the persisted representation of a chat session.
type SessionData struct {
	Messages   []core.LLMMessage `json:"messages"`
	SavedAt    time.Time         `json:"saved_at"`
	MessageCnt int               `json:"message_count"`
}

// DefaultSessionPath returns the default path to the session store file.
func DefaultSessionPath() string {
	return filepath.Join(meta.DataDir(), SessionFileName)
}

// SaveSession writes the conversation history to the session file.
func (cm *ContextManager) SaveSession(path string) error {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	msgs := cm.messages
	// Prepend system prompt as a message if set
	if cm.systemPrompt != "" {
		msgs = append([]core.LLMMessage{{Role: "system", Content: cm.systemPrompt}}, msgs...)
	}

	data := SessionData{
		Messages:   msgs,
		SavedAt:    time.Now(),
		MessageCnt: len(msgs),
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	if err := os.WriteFile(path, jsonData, 0600); err != nil {
		return fmt.Errorf("failed to write session file: %w", err)
	}
	return nil
}

// LoadSession reads the conversation history from the session file.
// Returns the loaded data and true if a session was found, or nil and false.
func LoadSession(path string) (*SessionData, bool) {
	data, err := os.ReadFile(path) // #nosec G304 -- path is internally derived
	if err != nil {
		return nil, false
	}

	var session SessionData
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, false
	}
	if session.Messages == nil {
		session.Messages = []core.LLMMessage{}
	}
	return &session, true
}

// RestoreSession loads the conversation history from the session file
// into the context manager. Returns the number of messages restored.
func (cm *ContextManager) RestoreSession(path string) int {
	session, ok := LoadSession(path)
	if !ok || len(session.Messages) == 0 {
		return 0
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Extract system prompt from the first message if present
	var msgs []core.LLMMessage
	for _, m := range session.Messages {
		if m.Role == "system" {
			cm.systemPrompt = m.Content
		} else {
			msgs = append(msgs, m)
		}
	}
	cm.messages = msgs
	return len(session.Messages)
}

// DeleteSession removes the session file if it exists.
func DeleteSession(path string) {
	_ = os.Remove(path) // best-effort: missing file is not an error
}
