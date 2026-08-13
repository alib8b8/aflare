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

// Package providers contains the LLM provider node implementations
// (OpenAI, Anthropic, Gemini, GLM, Qwen, Kimi, DeepSeek, Ollama, Mistral,
// MiniMax, Baichuan, InternLM, Yi, XVERSE, MiMo, Coze, FastGPT, IMA).
// Each provider registers itself with the global core.Registry via an
// init() function; subpackage registration is centralized by a blank
// import in internal/nodes/register_all.go.
//
// The shared infrastructure (Node interface, Registry, security helpers,
// LLM base client, parameter helpers) lives in internal/nodes/core and is
// referenced directly from here. Use core.GetParam / core.ParamInt at call
// sites rather than per-package wrappers.
package providers
