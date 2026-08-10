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

// register_all.go centralizes blank imports of all nodes subpackages so
// that their init() functions run and register their nodes with the
// global core.Registry. When adding a new subpackage under
// internal/nodes/, add a corresponding blank import line here.
package nodes

import (
	// Register drone nodes (MAVLink-compatible drone control via PX4/ArduPilot).
	_ "github.com/alib8b8/aflare/internal/nodes/drone"

	// Register mobile/hardware nodes (OnDeviceLLM, PowerManager, Robot,
	// Voice I/O, Screen, Video, AgentBrowser, BlockchainAudit, etc.).
	_ "github.com/alib8b8/aflare/internal/nodes/mobile"

	// Register LLM provider nodes (OpenAI, Anthropic, Gemini, GLM, Qwen,
	// Kimi, DeepSeek, Ollama, Mistral, MiniMax, Baichuan, InternLM, Yi,
	// XVERSE, MiMo, Coze, FastGPT, IMA, SenseNova, AntLing, AndesGPT).
	_ "github.com/alib8b8/aflare/internal/nodes/providers"
)
