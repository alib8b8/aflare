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

package main

import (
	"os"

	"github.com/alib8b8/aflare/internal/agent"
)

// handleChatCommand starts an interactive chat session with the aflare agent.
func handleChatCommand() {
	cfg := agent.DefaultConfig()

	// Allow environment variable overrides
	if v := os.Getenv("AFLARE_PROVIDER"); v != "" {
		cfg.Provider = v
	}
	if v := os.Getenv("AFLARE_MODEL"); v != "" {
		cfg.Model = v
	}
	if v := os.Getenv("AFLARE_API_KEY"); v != "" {
		cfg.APIKey = v
	}
	if v := os.Getenv("AFLARE_ENDPOINT"); v != "" {
		cfg.Endpoint = v
	}

	session := agent.NewChatSession(cfg)
	session.Run()
}