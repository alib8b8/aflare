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
	"strings"

	"github.com/alib8b8/aflare/internal/autoupgrade"
)

// HandleSelfUpdate handles the "self-update" / "update" command.
func HandleSelfUpdate() {
	repo := "alib8b8/aflare"
	fmt.Println("Checking for updates...")
	result, err := SelfUpdate(repo)
	if err != nil {
		fmt.Printf("❌ Update failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ %s\n", result)
}

// HandleAutoUpgrade handles the "autoupgrade" / "au" command.
func HandleAutoUpgrade(args []string) {
	if len(args) == 0 {
		PrintAutoUpgradeUsage()
		os.Exit(1)
	}

	config, err := autoupgrade.LoadConfig()
	if err != nil {
		fmt.Printf("❌ Failed to load config: %v\n", err)
		os.Exit(1)
	}

	engine := autoupgrade.NewUpgradeEngine(config)

	handleAutoUpgradeSubCmd(args, config, engine)
}

// handleAutoUpgradeSubCmd dispatches the autoupgrade subcommand to the appropriate handler.
func handleAutoUpgradeSubCmd(args []string, config *autoupgrade.UpgradeConfig, engine *autoupgrade.UpgradeEngine) {
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
			os.Exit(1)
		}
		fmt.Println("✅ Auto-upgrade enabled")
		fmt.Println("   Mode: auto (will automatically download and install updates)")

	case "disable":
		config.Mode = autoupgrade.ModeManual
		config.AutoUpdateEnabled = false
		if err := autoupgrade.SaveConfig(config); err != nil {
			fmt.Printf("❌ Failed to save config: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✅ Auto-upgrade disabled")
		fmt.Println("   Mode: manual (you need to run 'aflare self-update' manually)")

	case "monitor":
		config.Mode = autoupgrade.ModeMonitor
		if err := autoupgrade.SaveConfig(config); err != nil {
			fmt.Printf("❌ Failed to save config: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✅ Auto-upgrade set to monitor mode")
		fmt.Println("   Mode: monitor (checks for updates but does not install)")

	case "run":
		fmt.Println("Running manual upgrade check...")
		result, err := engine.RunSelfUpdate()
		if err != nil {
			fmt.Printf("❌ Upgrade failed: %v\n", err)
			os.Exit(1)
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
			os.Exit(1)
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
			os.Exit(1)
		}
		fmt.Println("✅ Config updated")

	case "auto-merge":
		result, err := engine.RunAutoMerge()
		if err != nil {
			fmt.Printf("❌ Auto-merge failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ %s\n", result)

	default:
		fmt.Printf("❌ Unknown command: %s\n", subCmd)
		PrintAutoUpgradeUsage()
		os.Exit(1)
	}
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
