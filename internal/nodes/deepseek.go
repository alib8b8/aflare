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

package nodes

func init() {
	Register(NewOpenAICompatibleNode(LLMNodeConfig{
		Name:            "deepseek",
		DefaultModel:    "deepseek-chat",
		DefaultEndpoint: "https://api.deepseek.com/v1",
		EnvAPIKey:       "DEEPSEEK_API_KEY",
		ProviderName:    "DeepSeek",
	}))
}
