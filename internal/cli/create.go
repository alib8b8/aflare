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

package cli

import (
	"fmt"
	"os"

	"github.com/alib8b8/aflare/internal/agent"
	"github.com/alib8b8/aflare/internal/i18n"
	"github.com/alib8b8/aflare/internal/workflow"
)

// HandleCreate handles the "create" command.
func HandleCreate(args []string, aiMode bool) {
	interactive := false

	// Filter out --interactive flag from args
	filteredArgs := make([]string, 0, len(args))
	for _, a := range args {
		if a == "--interactive" || a == "-i" {
			interactive = true
		} else {
			filteredArgs = append(filteredArgs, a)
		}
	}

	if len(filteredArgs) < 1 {
		fmt.Println(i18n.T("create.usage"))
		fmt.Println("\nExamples:")
		fmt.Println("  aflare create \"fetch example.com and save to file\"")
		fmt.Println("  aflare create \"fetch Hacker News and save to hn.txt\"")
		fmt.Println("  aflare create \"summarize article and write to summary.md\"")
		fmt.Println("  aflare --ai create \"generate a weekly report from github commits\"")
		fmt.Println("  aflare create --interactive \"fetch example.com\"")
		os.Exit(1)
	}

	description := SummarizeCommand("", filteredArgs)
	fmt.Printf("%s\n", i18n.T("create.start", description))

	var filename string
	var err error
	if aiMode {
		filename, err = workflow.CreateWorkflowFromDescriptionWithAI(description, true)
	} else {
		filename, err = workflow.CreateWorkflowFromDescription(description)
	}
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n✅ %s\n", i18n.T("create.success", filename))
	fmt.Printf("\n%s\n", i18n.T("create.run_hint"))
	fmt.Printf("  aflare run %s\n", filename)

	if interactive {
		fmt.Println("\nEntering interactive chat mode to validate your workflow...")
		fmt.Println("Type /quit to exit.")
		EnterChatMode()
	}
}

// EnterChatMode starts an interactive chat session for workflow validation.
// This is called from create --interactive.
func EnterChatMode() {
	cfg := agent.DefaultConfig()
	session := agent.NewChatSession(cfg)
	session.Run()
}
