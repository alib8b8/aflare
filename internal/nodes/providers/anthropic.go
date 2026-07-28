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

package providers

import (
	"github.com/alib8b8/llm-box/internal/nodes/core"
)

func init() {
	core.Register(core.NewOpenAICompatibleNode(core.LLMNodeConfig{
		Name:            "anthropic",
		DefaultModel:    "claude-3-5-sonnet-latest",
		DefaultEndpoint: "https://api.anthropic.com/v1",
		EnvAPIKey:       "ANTHROPIC_API_KEY", // #nosec G101 -- env var name, not a credential value
		ProviderName:    "Anthropic",
	}))
}
