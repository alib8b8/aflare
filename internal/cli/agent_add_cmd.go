// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​‌‌​​‌​‌​​‌‌​​​‌​​‌​​​​‌‌‌‌​‌​​‌‌‌‌​‌​‌​​‌‌​‌‌​​‌​‌‌‌​​​‌‌‌‌‌​‌‌​‌​​​​​​​​​​​​​​​​​​​​‌‌​‌​​​‌​​​‌‌​‌​⁠
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

package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/alib8b8/aflare/internal/agentx"
)

// addAgent registers an external A2A agent from its agent card:
//
//	aflare agent add <url> [--name N] [--api-key-env VAR] [--description D]
//
// It fetches the remote agent card, derives a registry name (the card
// name, slugified) and fills the AgentDef — description plus advertised
// skills, so the supervisor's planning prompt knows what to delegate
// where. Credentials are never stored: --api-key-env records only the
// environment variable NAME the agent's bearer token is read from.
func addAgent(args []string) error {
	rawURL, flagArgs := splitAgentAddArgs(args)

	fs := flag.NewFlagSet("agent add", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var name, apiKeyEnv, description string
	fs.StringVar(&name, "name", "", "registry name (default: derived from the agent card name)")
	fs.StringVar(&apiKeyEnv, "api-key-env", "", "environment variable holding the bearer token (the value is never stored)")
	fs.StringVar(&description, "description", "", "override the agent description (default: card description + skills)")
	if err := fs.Parse(flagArgs); err != nil {
		return exitErr(1)
	}
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" || fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "Usage: aflare agent add <url> [--name N] [--api-key-env VAR] [--description D]")
		return exitErr(1)
	}

	probe := agentx.AgentDef{Name: "agent-card-probe", Driver: agentx.DriverA2A, URL: rawURL, APIKeyEnv: apiKeyEnv}
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	card, err := agentx.FetchAgentCard(ctx, probe)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to fetch an agent card from %s: %v\n", rawURL, err)
		fmt.Fprintln(os.Stderr, "The URL must point at an A2A agent (http/https) that serves /.well-known/agent-card.json.")
		return exitErr(1)
	}

	if name == "" {
		name = slugifyAgentName(card.Name)
	}
	if name == "" {
		fmt.Fprintf(os.Stderr, "The agent card has no usable name (%q). Re-run with --name.\n", card.Name)
		return exitErr(1)
	}
	if agentx.IsBuiltinAgentName(name) {
		fmt.Fprintf(os.Stderr, "Name %q collides with a built-in CLI preset; choose another with --name.\n", name)
		return exitErr(1)
	}

	if description == "" {
		description = describeFromCard(card)
	}
	def := agentx.AgentDef{
		Name:        name,
		Driver:      agentx.DriverA2A,
		Description: description,
		URL:         strings.TrimRight(rawURL, "/"),
		APIKeyEnv:   apiKeyEnv,
	}
	if _, err := def.Resolve(); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid agent definition: %v\n", err)
		return exitErr(1)
	}

	storePath := agentx.DefaultAgentStorePath()
	stored, err := agentx.LoadAgentStore(storePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot read the agent store: %v\n", err)
		return exitErr(1)
	}
	if _, exists := stored[name]; exists {
		fmt.Fprintf(os.Stderr, "Agent %q is already registered (remove it from %s first, or pick another --name).\n", name, storePath)
		return exitErr(1)
	}
	stored[name] = def
	if err := agentx.SaveAgentStore(storePath, stored); err != nil {
		fmt.Fprintf(os.Stderr, "Cannot save the agent store: %v\n", err)
		return exitErr(1)
	}

	fmt.Printf("Registered agent %q (a2a → %s)\n", name, def.URL)
	if def.APIKeyEnv != "" {
		fmt.Printf("  Auth: bearer token read from $%s\n", def.APIKeyEnv)
	}
	fmt.Printf("  Saved to %s\n", storePath)
	fmt.Println()
	fmt.Println("Use it from a workflow:")
	fmt.Printf("  supervisor:  specialists: \"@%s,...\"\n", name)
	fmt.Printf("  single step: node: a2a_agent  params: { agent: %s }\n", name)
	fmt.Printf("  verify:      aflare agent probe %s\n", name)
	return nil
}

// splitAgentAddArgs extracts the positional URL from an argument list
// where flags may appear before or after it (Go's flag package stops at
// the first positional, so `agent add <url> --name x` would otherwise
// swallow the flags — same dance as connector add). Value-taking flags
// consume their value token; a second positional is left for fs.Parse
// to reject.
func splitAgentAddArgs(args []string) (url string, rest []string) {
	valueFlags := map[string]bool{"name": true, "api-key-env": true, "description": true}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") {
			if url == "" {
				url = a
			} else {
				rest = append(rest, a)
			}
			continue
		}
		rest = append(rest, a)
		if strings.Contains(a, "=") {
			continue
		}
		if valueFlags[strings.TrimLeft(a, "-")] && i+1 < len(args) {
			i++
			rest = append(rest, args[i])
		}
	}
	return url, rest
}

// slugifyAgentName maps a card name onto a registry name: lowercase,
// runs of non-alphanumerics become single dashes, leading/trailing
// dashes are trimmed. Registry names are referenced as "@name" in the
// specialists list, so they must not contain commas or spaces.
func slugifyAgentName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var sb strings.Builder
	dash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			sb.WriteRune(r)
			dash = true
		default:
			if dash && sb.Len() > 0 {
				sb.WriteByte('-')
				dash = false
			}
		}
	}
	return strings.Trim(sb.String(), "-")
}

// describeFromCard renders the supervisor-facing description from the
// card: its own description plus the advertised skills. The delegation
// planner prompt embeds this text, so skills listed here are what the
// LLM routes by.
func describeFromCard(card *agentx.AgentCard) string {
	var sb strings.Builder
	if d := strings.TrimSpace(card.Description); d != "" {
		sb.WriteString(d)
	} else if card.Name != "" {
		fmt.Fprintf(&sb, "A2A agent %q", card.Name)
	} else {
		sb.WriteString("Remote A2A agent")
	}
	if len(card.Skills) > 0 {
		fmt.Fprintf(&sb, " Skills: ")
		for i, skill := range card.Skills {
			if i > 0 {
				sb.WriteString(", ")
			}
			label := skill.Name
			if label == "" {
				label = skill.ID
			}
			if d := strings.TrimSpace(skill.Description); d != "" {
				label += " (" + d + ")"
			}
			sb.WriteString(label)
		}
	}
	return sb.String()
}
