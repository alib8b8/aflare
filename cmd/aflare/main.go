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
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/alib8b8/aflare/internal/cli"
	"github.com/alib8b8/aflare/internal/i18n"
	"github.com/alib8b8/aflare/internal/nodes"
	"github.com/alib8b8/aflare/internal/output"
	"github.com/alib8b8/aflare/internal/plugins"
	"github.com/alib8b8/aflare/internal/telemetry"
)

func main() {
	if len(os.Args) < 2 {
		i18n.Init("")
		fmt.Println(cli.PrintUsage())
		fmt.Println(cli.FirstRunHint())
		os.Exit(1)
	}
	command, args, safeMode, dryRun, mcpServer, lang, concise, initMCP, initAgent, updateChannel, serveMode, aiMode, otelEndpoint, otelServiceName := cli.ParseArgs(os.Args[1:])
	if concise {
		output.SetMode(output.ModeConcise)
	}
	i18n.Init(lang)

	// OpenTelemetry: initialise the global tracer provider if an OTLP endpoint
	// is configured (via --otel-endpoint or OTEL_EXPORTER_OTLP_ENDPOINT env).
	// Tracing is a no-op when no endpoint is set.
	ctx := context.Background()
	otelShutdown, err := telemetry.InitTracer(ctx, telemetry.Config{
		Endpoint:    otelEndpoint,
		ServiceName: otelServiceName,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: OpenTelemetry initialisation failed: %v\n", err)
	}
	defer func() {
		if err := otelShutdown(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: OpenTelemetry shutdown failed: %v\n", err)
		}
	}()

	// Load community plugins from ~/.config/aflare/plugins/*.so.
	// Opt-out via AFLARE_NO_PLUGINS for air-gapped / data-sensitive
	// deployments where auto-loading .so files is an unwanted code-exec
	// surface. We also lock the dir to 0700 on first use so only the owner
	// can drop plugins.
	pluginMgr := plugins.NewPluginManager()
	if os.Getenv("AFLARE_NO_PLUGINS") == "" {
		pluginDir := plugins.DefaultPluginDir()
		if err := plugins.EnsurePluginDirSecure(pluginDir); err != nil {
			log.Printf("[main] plugin dir security: %v", err)
		}
		if n, err := plugins.LoadDir(pluginDir, pluginMgr); err != nil {
			log.Printf("[main] plugin loading: %v", err)
		} else if n > 0 {
			log.Printf("[main] loaded %d plugin(s)", n)
		}
	}

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
		// 断点: 未知命令不能静默回退到 usage，否则用户不知道哪里打错了
		// （例如 `aflare node list` 之前只显示主 help）。打印错误 + did-you-mean。
		fmt.Fprintf(os.Stderr, "❌ %v\n", err)
		if suggestions := cli.SuggestCommand(command); len(suggestions) > 0 {
			fmt.Fprintf(os.Stderr, "你是不是想输入：%s\n", strings.Join(suggestions, ", "))
		}
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, cli.PrintUsage())
		os.Exit(1)
	}
	// 断点17: 启动时检测新版本（非阻塞，提示一行）。仅对交互/信息类命令
	// 生效，避免延迟工作流执行或与输出交错。交互类命令在进入主流程前提示，
	// 信息类命令在输出后提示。
	if wantsUpdateNoticeBefore(command) {
		cli.PrintUpdateNoticeAsync(os.Stderr, 1500*time.Millisecond)
	}
	dispatchCommand(command, args, aiMode, dryRun, safeMode)
	if wantsUpdateNoticeAfter(command) {
		cli.PrintUpdateNoticeAsync(os.Stderr, 1500*time.Millisecond)
	}
}

// wantsUpdateNoticeBefore returns true for interactive commands that should
// receive the "new version available" hint before entering their main loop.
func wantsUpdateNoticeBefore(command string) bool {
	switch command {
	case "chat", "agent", "doctor", "init":
		return true
	}
	return false
}

// wantsUpdateNoticeAfter returns true for one-shot informational commands that
// should receive the "new version available" hint after their output.
func wantsUpdateNoticeAfter(command string) bool {
	switch command {
	case "version", "--version", "-v", "help", "-h", "--help", "list", "config", "skills", "registry":
		return true
	}
	return false
}

func dispatchCommand(command string, args []string, aiMode bool, dryRun bool, safeMode bool) {
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
	case "upgrade":
		cli.HandleUpgrade(args)
	case "doctor":
		cli.HandleDoctor(args)
	case "autoupgrade", "au":
		cli.HandleAutoUpgrade(args)
	case "init":
		cli.HandleInit(args)
	case "config":
		cli.HandleConfig(args)
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
	case "chat":
		cli.HandleChat(args)
	case "agent":
		cli.HandleAgent(args)
	case "template":
		cli.HandleTemplateSubmit(args, dryRun, safeMode)
	case "install-pack":
		cli.HandleInstallPack(args)
	case "watermark":
		cli.HandleWatermark(args)
	case "badge":
		cli.HandleBadge(args)
	default:
		cli.HandleRunFile(command, dryRun, false, "", safeMode, nil)
	}
}
