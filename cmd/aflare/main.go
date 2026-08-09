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
	"fmt"
	"os"

	"github.com/alib8b8/aflare/internal/cli"
	"github.com/alib8b8/aflare/internal/i18n"
	"github.com/alib8b8/aflare/internal/nodes"
	"github.com/alib8b8/aflare/internal/output"
)

func main() {
	if len(os.Args) < 2 {
		i18n.Init("")
		fmt.Println(cli.PrintUsage())
		os.Exit(1)
	}
	command, args, safeMode, dryRun, mcpServer, lang, concise, initMCP, initAgent, updateChannel, serveMode, aiMode := cli.ParseArgs(os.Args[1:])
	if concise {
		output.SetMode(output.ModeConcise)
	}
	i18n.Init(lang)
	if mcpServer {
		cli.HandleMCP()
		return
	}
	if serveMode {
		cli.HandleServe(args)
		return
	}
	if initMCP != "" {
		cli.HandleInitMCPQuick(initMCP)
		return
	}
	if initAgent != "" {
		cli.HandleInitAgentQuick(initAgent)
		return
	}
	if updateChannel != "" {
		cli.HandleChannelQuick(updateChannel)
		return
	}
	if safeMode {
		nodes.SetSafeMode(true)
		fmt.Println(i18n.T("safe_mode.enabled"))
	}
	if dryRun {
		fmt.Println(i18n.T("dry_run.enabled"))
	}
	if command == "" {
		fmt.Println(cli.PrintUsage())
		os.Exit(1)
	}
	if err := cli.ValidateCommand(command); err != nil {
		fmt.Println(cli.PrintUsage())
		os.Exit(1)
	}
	switch command {
	case "create":
		cli.HandleCreate(args, aiMode)
	case "run":
		cli.HandleRun(args, dryRun, safeMode)
	case "install":
		cli.HandleInstall(args)
	case "uninstall":
		cli.HandleUninstall(args)
	case "registry":
		cli.HandleRegistry(args)
	case "list":
		cli.HandleList()
	case "validate":
		cli.HandleValidate(args)
	case "review":
		cli.HandleReview(args)
	case "version", "--version", "-v":
		fmt.Println(cli.PrintVersion())
	case "self-update", "update":
		cli.HandleSelfUpdate()
	case "autoupgrade", "au":
		cli.HandleAutoUpgrade(args)
	case "init":
		cli.HandleInit(args)
	case "skills":
		cli.HandleSkills(args)
	case "-h", "--help", "help":
		fmt.Println(cli.PrintUsage())
	case "webui":
		cli.HandleWebUI(args)
	case "schedule":
		cli.HandleSchedule(args)
	case "audit":
		cli.HandleAudit(args)
	case "serve":
		cli.HandleServe(args)
	case "marketplace":
		cli.HandleMarketplace(args)
	case "secrets":
		cli.HandleSecrets(args)
	case "resume":
		cli.HandleResume(args)
	default:
		cli.HandleRunFile(command, dryRun, false, "", safeMode)
	}
}
