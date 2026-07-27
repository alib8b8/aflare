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

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/alib8b8/llm-box/internal/memory"
)

type PreferenceNode struct{}

func init() {
	Register(&PreferenceNode{})
}

func (n *PreferenceNode) Name() string {
	return "preference"
}

func (n *PreferenceNode) Description() string {
	return "Manage and query user preferences for personalized outputs (MemSlides-inspired)"
}

func (n *PreferenceNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "preference",
		Description: "User preference memory: store, retrieve, and learn user habits across sessions (MemSlides-inspired user profiling)",
		Input:       "string - input depends on operation (value to set, key to get, etc.)",
		Output:      "string - result of the operation",
		Params: []ParamSchema{
			{Name: "operation", Type: "string", Description: "get|set|learn|summary|category|prompt_addon (default: get)", Required: false, Default: "get"},
			{Name: "user_id", Type: "string", Description: "User identifier (default: default)", Required: false, Default: "default"},
			{Name: "category", Type: "string", Description: "coding_style|output_format|model_choice|verbosity|language|safety|workflow|custom", Required: false, Default: "custom"},
			{Name: "key", Type: "string", Description: "Preference key name", Required: false},
			{Name: "value", Type: "string", Description: "Preference value (for set/learn operations)", Required: false},
			{Name: "confidence", Type: "string", Description: "Confidence 0-1, default 0.6 for learn, 1.0 for set", Required: false},
			{Name: "source", Type: "string", Description: "Where this preference came from (explicit|learned|config)", Required: false, Default: "explicit"},
		},
	}
}

func (n *PreferenceNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	operation := getParam(params, "operation", "get")
	userID := getParam(params, "user_id", "default")
	category := getParam(params, "category", "custom")
	key := getParam(params, "key", "")
	value := getParam(params, "value", input)
	source := getParam(params, "source", "explicit")

	if value == "" && input != "" && (operation == "set" || operation == "learn") {
		value = input
	}

	pm := memory.GetProfileManager()
	profile := pm.GetProfile(userID)

	switch operation {
	case "set":
		if key == "" {
			return "", fmt.Errorf("key is required for set operation")
		}
		confidence := 1.0
	if c, ok := params["confidence"]; ok && c != "" {
		if _, err := fmt.Sscanf(c, "%f", &confidence); err != nil {
			// keep default value on parse failure
		}
	}
	profile.SetPreference(memory.PreferenceCategory(category), key, value, source, confidence)
		pm.Save(userID)
		return fmt.Sprintf("Set preference [%s:%s] = %s", category, key, value), nil

	case "learn":
		if key == "" {
			return "", fmt.Errorf("key is required for learn operation")
		}
		confidence := 0.6
	if c, ok := params["confidence"]; ok && c != "" {
		if _, err := fmt.Sscanf(c, "%f", &confidence); err != nil {
			// keep default value on parse failure
		}
	}
	profile.LearnFromInteraction(userID, category, key, value, source)
		_ = confidence
		pm.Save(userID)
		return fmt.Sprintf("Learned preference [%s:%s] = %s", category, key, value), nil

	case "get":
		if key == "" {
			return "", fmt.Errorf("key is required for get operation")
		}
		val, conf, ok := profile.GetPreference(memory.PreferenceCategory(category), key)
		if !ok {
			return "", fmt.Errorf("preference not found: %s:%s", category, key)
		}
		return fmt.Sprintf("%s (confidence: %.0f%%)", val, conf*100), nil

	case "category":
		prefs := profile.GetAllByCategory(memory.PreferenceCategory(category))
		if len(prefs) == 0 {
			return fmt.Sprintf("No preferences in category: %s", category), nil
		}
		var parts []string
		for k, v := range prefs {
			parts = append(parts, fmt.Sprintf("  %s: %s", k, v))
		}
		return fmt.Sprintf("Preferences in [%s]:\n%s", category, strings.Join(parts, "\n")), nil

	case "summary":
		summary := profile.GetSummary()
		data, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return "", fmt.Errorf("failed to marshal summary: %w", err)
		}
		return string(data), nil

	case "prompt_addon":
		addon := profile.BuildSystemPromptAddon()
		if addon == "" {
			return "", nil
		}
		return addon, nil

	default:
		return "", fmt.Errorf("unknown operation: %s (supported: get, set, learn, category, summary, prompt_addon)", operation)
	}
}
