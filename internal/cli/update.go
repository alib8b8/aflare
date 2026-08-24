// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​​​​​‌​​​​‌‌​‌​‌‌‌‌‌‌‌​​‌‌‌‌‌‌‌‌​​‌​​​‌‌​‌‌​‌​‌​​​​​​​​​​​​​​​​​​​‌​‌‌​​‌‌​‌‌​‌‌⁠
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
	"io"
	"os"
	"strings"
	"time"

	"github.com/alib8b8/aflare/internal/autoupgrade"
	"github.com/alib8b8/aflare/internal/meta"
)

// HandleSelfUpdate handles the "self-update" / "update" command.
func HandleSelfUpdate() error {
	repo := "alib8b8/aflare"
	fmt.Println("Checking for updates...")
	result, err := SelfUpdate(repo)
	if err != nil {
		fmt.Printf("❌ Update failed: %v\n", err)
		fmt.Print(updateNetworkHint(err))
		return exitErr(1)
	}
	fmt.Printf("✅ %s\n", result)
	return nil
}

// updateNetworkHint returns a actionable hint when an update error looks like
// a network/SSRF rejection (private/loopback address, or a generic dial
// failure). Empty string when the error is unrelated.
func updateNetworkHint(err error) string {
	msg := err.Error()
	if strings.Contains(msg, "private address") ||
		strings.Contains(msg, "loopback address") ||
		strings.Contains(msg, "is not allowed") {
		return "\n提示：你的网络环境可能把 github.com 解析到了内网地址（企业镜像/VPN/零信任网关）。\n" +
			"      可设置 export AFLARE_SELF_UPDATE_ALLOW_PRIVATE=1 后重试（仍只允许访问 GitHub 官方域名），\n" +
			"      或设置 HTTPS_PROXY 指向可出公网的代理。\n"
	}
	return "\n提示：检查网络连接，或设置 HTTPS_PROXY 后重试。\n"
}

// HandleUpgrade handles the "upgrade" command (断点17: 没有 aflare upgrade
// 一键升级). It wraps SelfUpdate with a friendlier, step-by-step progress
// output so users get clear feedback during the update process.
func HandleUpgrade(args []string) error {
	repo := "alib8b8/aflare"
	current := meta.GetVersion()

	fmt.Println("检查最新版本...")
	latestTag, hasUpdate, err := CheckUpdate(repo)
	if err != nil {
		fmt.Printf("❌ 检查更新失败: %v\n", err)
		fmt.Println("提示：检查网络连接，或设置 HTTPS_PROXY 后重试。")
		return exitErr(1)
	}

	fmt.Printf("当前版本：%s\n", current)
	fmt.Printf("最新版本：%s\n", latestTag)

	if !hasUpdate {
		fmt.Println("已是最新版本。")
		return nil
	}

	fmt.Print("下载中... ")
	result, err := SelfUpdate(repo)
	if err != nil {
		fmt.Println()
		fmt.Printf("❌ 更新失败: %v\n", err)
		fmt.Print(updateNetworkHint(err))
		return exitErr(1)
	}
	// SelfUpdate verifies the SHA256 checksum internally before replacing the
	// binary; reaching here means verification passed.
	fmt.Println("████████████████████ 100%")
	fmt.Println("校验 SHA256... ✓")
	fmt.Printf("更新完成。%s\n", result)
	return nil
}

// CheckUpdateNotice performs a short, non-blocking check for a newer aflare
// release and returns a one-line notice string when an update is available.
// Returns "" when no update is available or the check fails (callers should
// treat failure as "no notice" to avoid noisy startup output).
func CheckUpdateNotice() string {
	repo := "alib8b8/aflare"
	current := meta.GetVersion()
	latestTag, hasUpdate, err := CheckUpdate(repo)
	if err != nil || !hasUpdate {
		return ""
	}
	return fmt.Sprintf("aflare %s (有新版本 %s，运行 aflare upgrade 更新)", current, latestTag)
}

// PrintUpdateNoticeAsync checks for a newer aflare release bounded by timeout
// and writes a one-line notice to w when an update is available. It blocks
// for at most timeout, so callers that want truly non-blocking startup output
// should invoke it in a goroutine. On timeout or failure it writes nothing.
//
// This implements the 断点17 startup hint: "aflare vX.Y.Z (有新版本 vX.Y.Z，
// 运行 aflare upgrade 更新)".
//
// Default is OFF. aflare's audience is local-first / data-sensitive users
// who should not phone-home to api.github.com on every launch. Opt in by
// setting AFLARE_UPDATE_CHECK=1 (any non-empty value). The legacy
// AFLARE_NO_UPDATE_CHECK is still respected as a redundant opt-out.
// Manual `aflare self-update` always works regardless of these flags.
func PrintUpdateNoticeAsync(w io.Writer, timeout time.Duration) {
	if timeout <= 0 {
		return
	}
	if os.Getenv("AFLARE_NO_UPDATE_CHECK") != "" {
		return
	}
	if os.Getenv("AFLARE_UPDATE_CHECK") == "" {
		return // default off for local-first / data-sensitive users
	}
	type result struct{ notice string }
	ch := make(chan result, 1)
	go func() {
		ch <- result{notice: CheckUpdateNotice()}
	}()
	select {
	case r := <-ch:
		if r.notice != "" {
			fmt.Fprintln(w, r.notice)
		}
	case <-time.After(timeout):
	}
}

// HandleAutoUpgrade handles the "autoupgrade" / "au" command.
func HandleAutoUpgrade(args []string) error {
	if len(args) == 0 {
		PrintAutoUpgradeUsage()
		return exitErr(1)
	}

	config, err := autoupgrade.LoadConfig()
	if err != nil {
		fmt.Printf("❌ Failed to load config: %v\n", err)
		return exitErr(1)
	}

	engine := autoupgrade.NewUpgradeEngine(config)

	if err := handleAutoUpgradeSubCmd(args, config, engine); err != nil {
		return err
	}
	return nil
}

// handleAutoUpgradeSubCmd dispatches the autoupgrade subcommand to the appropriate handler.
func handleAutoUpgradeSubCmd(args []string, config *autoupgrade.UpgradeConfig, engine *autoupgrade.UpgradeEngine) error {
	subCmd := args[0]
	switch subCmd {
	case "status":
		state := engine.GetState()
		fmt.Println("Auto-upgrade Status:")
		fmt.Printf("  Mode: %s\n", config.Mode)
		fmt.Printf("  Current Version: %s\n", state.CurrentVersion)
		fmt.Printf("  Latest Version: %s\n", state.LatestVersion)
		fmt.Printf("  Auto-update Enabled: %t\n", config.AutoUpdateEnabled)
		fmt.Printf("  Auto-merge Enabled: %t\n", config.AutoMergeEnabled)
		fmt.Printf("  Check Interval: %s\n", config.CheckInterval)
		fmt.Printf("  Backup Before Upgrade: %t\n", config.BackupBeforeUpgrade)
		fmt.Printf("  Rollback On Failure: %t\n", config.RollbackOnFailure)
		fmt.Printf("  Last Check: %s\n", state.LastCheck.Format("2006-01-02 15:04:05"))
		fmt.Printf("  Last Upgrade: %s\n", state.LastUpgrade.Format("2006-01-02 15:04:05"))
		fmt.Printf("  Upgrade In Progress: %t\n", state.UpgradeInProgress)
		fmt.Printf("  Upgrade Status: %s\n", state.UpgradeStatus)

	case "enable":
		config.Mode = autoupgrade.ModeAuto
		config.AutoUpdateEnabled = true
		if err := autoupgrade.SaveConfig(config); err != nil {
			fmt.Printf("❌ Failed to save config: %v\n", err)
			return exitErr(1)
		}
		fmt.Println("✅ Auto-upgrade enabled")
		fmt.Println("   Mode: auto (will automatically download and install updates)")

	case "disable":
		config.Mode = autoupgrade.ModeManual
		config.AutoUpdateEnabled = false
		if err := autoupgrade.SaveConfig(config); err != nil {
			fmt.Printf("❌ Failed to save config: %v\n", err)
			return exitErr(1)
		}
		fmt.Println("✅ Auto-upgrade disabled")
		fmt.Println("   Mode: manual (you need to run 'aflare self-update' manually)")

	case "monitor":
		config.Mode = autoupgrade.ModeMonitor
		if err := autoupgrade.SaveConfig(config); err != nil {
			fmt.Printf("❌ Failed to save config: %v\n", err)
			return exitErr(1)
		}
		fmt.Println("✅ Auto-upgrade set to monitor mode")
		fmt.Println("   Mode: monitor (checks for updates but does not install)")

	case "run":
		fmt.Println("Running manual upgrade check...")
		result, err := engine.RunSelfUpdate()
		if err != nil {
			fmt.Printf("❌ Upgrade failed: %v\n", err)
			return exitErr(1)
		}
		fmt.Printf("✅ %s\n", result)

	case "config":
		if len(args) < 2 {
			fmt.Println("Usage: aflare autoupgrade config <key>=<value>")
			fmt.Println("\nAvailable keys:")
			fmt.Println("  mode           - auto, monitor, manual")
			fmt.Println("  auto_update    - true, false")
			fmt.Println("  auto_merge     - true, false")
			fmt.Println("  interval       - 1h, 6h, 24h, etc.")
			fmt.Println("  backup         - true, false")
			fmt.Println("  rollback       - true, false")
			return exitErr(1)
		}

		for _, arg := range args[1:] {
			parts := strings.SplitN(arg, "=", 2)
			if len(parts) != 2 {
				fmt.Printf("⚠️ Invalid config format: %s\n", arg)
				continue
			}
			key, value := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
			updateConfigKey(config, key, value)
		}

		if err := autoupgrade.SaveConfig(config); err != nil {
			fmt.Printf("❌ Failed to save config: %v\n", err)
			return exitErr(1)
		}
		fmt.Println("✅ Config updated")

	case "auto-merge":
		result, err := engine.RunAutoMerge()
		if err != nil {
			fmt.Printf("❌ Auto-merge failed: %v\n", err)
			return exitErr(1)
		}
		fmt.Printf("✅ %s\n", result)

	default:
		fmt.Printf("❌ Unknown command: %s\n", subCmd)
		PrintAutoUpgradeUsage()
		return exitErr(1)
	}
	return nil
}

// PrintAutoUpgradeUsage prints usage information for the autoupgrade command.
func PrintAutoUpgradeUsage() {
	fmt.Println("Usage: aflare autoupgrade <command>")
	fmt.Println("\nCommands:")
	fmt.Println("  status       - Show current auto-upgrade status")
	fmt.Println("  enable       - Enable automatic updates")
	fmt.Println("  disable      - Disable automatic updates")
	fmt.Println("  monitor      - Enable monitor mode (notify only)")
	fmt.Println("  run          - Manually trigger upgrade check")
	fmt.Println("  config       - Configure auto-upgrade settings")
	fmt.Println("  auto-merge   - Run automatic branch merge")
	fmt.Println("\nExamples:")
	fmt.Println("  aflare autoupgrade enable")
	fmt.Println("  aflare autoupgrade config mode=auto interval=6h")
	fmt.Println("  aflare autoupgrade run")
}

func updateConfigKey(config *autoupgrade.UpgradeConfig, key, value string) {
	switch strings.ToLower(key) {
	case "mode":
		switch strings.ToLower(value) {
		case "auto":
			config.Mode = autoupgrade.ModeAuto
		case "monitor":
			config.Mode = autoupgrade.ModeMonitor
		case "manual":
			config.Mode = autoupgrade.ModeManual
		}
	case "auto_update", "auto-update", "autoupdate":
		config.AutoUpdateEnabled = strings.ToLower(value) == "true"
	case "auto_merge", "auto-merge", "automerge":
		config.AutoMergeEnabled = strings.ToLower(value) == "true"
	case "interval", "check_interval":
		config.CheckInterval = value
	case "backup", "backup_before_upgrade":
		config.BackupBeforeUpgrade = strings.ToLower(value) == "true"
	case "rollback", "rollback_on_failure":
		config.RollbackOnFailure = strings.ToLower(value) == "true"
	}
}
