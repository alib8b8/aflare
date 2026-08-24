// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​​​​‌​‌​‌‌​‌​​​​‌​‌​‌‌​‌‌‌‌‌‌‌​​​‌‌‌​‌​​‌​​‌​‌‌‌​‌​‌​​‌​‌​‌‌​‌‌‌‌‌​​​​​​​​​​​​​​​​​‌​‌‌​‌‌​‌‌‌​‌‌‌⁠
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
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alib8b8/aflare/internal/autoupgrade"
)

// autoupgradeConfigFile returns the path where autoupgrade.SaveConfig writes
// the config for the currently set HOME.
func autoupgradeConfigFile(t *testing.T) string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}
	return filepath.Join(home, ".config", "aflare", "autoupgrade.yaml")
}

func TestUpdateAutoUpgradeNoArgs(t *testing.T) {
	var err error
	output := captureOutput(func() {
		err = HandleAutoUpgrade(nil)
	})
	if code := exitCodeForErr(err); code != 1 {
		t.Errorf("HandleAutoUpgrade(nil) exit code = %d, want 1 (err=%v)", code, err)
	}
	if !strings.Contains(output, "Usage: aflare autoupgrade") {
		t.Errorf("expected usage output, got: %s", output)
	}
}

func TestUpdateAutoUpgradeUnknownSubcommand(t *testing.T) {
	// handleAutoUpgradeSubCmd has no dedicated "help" case, so help-style
	// flags fall through to the unknown-command default and exit 1.
	for _, sub := range []string{"frobnicate", "help", "-h", "--help"} {
		t.Run(sub, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			var err error
			output := captureOutput(func() {
				err = HandleAutoUpgrade([]string{sub})
			})
			if code := exitCodeForErr(err); code != 1 {
				t.Errorf("HandleAutoUpgrade([%q]) exit code = %d, want 1 (err=%v)", sub, code, err)
			}
			if !strings.Contains(output, "Unknown command") {
				t.Errorf("expected unknown-command message for %q, got: %s", sub, output)
			}
		})
	}
}

func TestUpdateAutoUpgradeStatus(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	var err error
	output := captureOutput(func() {
		err = HandleAutoUpgrade([]string{"status"})
	})
	if err != nil {
		t.Fatalf("HandleAutoUpgrade([status]) = %v, want nil", err)
	}
	for _, want := range []string{
		"Auto-upgrade Status:",
		"Mode: manual", // default config is local-first manual mode
		"Auto-update Enabled: false",
		"Auto-merge Enabled: false",
		"Check Interval: 24h",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("status output missing %q, got: %s", want, output)
		}
	}
}

func TestUpdateAutoUpgradeEnableDisableMonitor(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfgPath := filepath.Join(home, ".config", "aflare", "autoupgrade.yaml")

	steps := []struct {
		sub    string
		wantIn string
	}{
		{"enable", "mode: auto"},
		{"disable", "mode: manual"},
		{"monitor", "mode: monitor"},
	}
	for _, step := range steps {
		var err error
		captureOutput(func() {
			err = HandleAutoUpgrade([]string{step.sub})
		})
		if err != nil {
			t.Fatalf("HandleAutoUpgrade([%s]) = %v, want nil", step.sub, err)
		}
		data, rerr := os.ReadFile(cfgPath)
		if rerr != nil {
			t.Fatalf("config file not written after %s: %v", step.sub, rerr)
		}
		if !strings.Contains(string(data), step.wantIn) {
			t.Errorf("config after %s = %s, want it to contain %q", step.sub, data, step.wantIn)
		}
	}
}

func TestUpdateAutoUpgradeConfigUsage(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	var err error
	output := captureOutput(func() {
		err = HandleAutoUpgrade([]string{"config"})
	})
	if code := exitCodeForErr(err); code != 1 {
		t.Errorf("HandleAutoUpgrade([config]) exit code = %d, want 1 (err=%v)", code, err)
	}
	if !strings.Contains(output, "Usage: aflare autoupgrade config <key>=<value>") {
		t.Errorf("expected config usage output, got: %s", output)
	}
}

func TestUpdateAutoUpgradeConfigSet(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	var err error
	output := captureOutput(func() {
		err = HandleAutoUpgrade([]string{"config", "mode=auto", "interval=6h", "auto_merge=true"})
	})
	if err != nil {
		t.Fatalf("HandleAutoUpgrade([config ...]) = %v, want nil", err)
	}
	if !strings.Contains(output, "Config updated") {
		t.Errorf("expected 'Config updated' message, got: %s", output)
	}

	data, rerr := os.ReadFile(autoupgradeConfigFile(t))
	if rerr != nil {
		t.Fatalf("config file not written: %v", rerr)
	}
	for _, want := range []string{"mode: auto", "interval: 6h", "auto_merge_enabled: true"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("config = %s, want it to contain %q", data, want)
		}
	}
}

func TestUpdateAutoUpgradeConfigInvalidPair(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	// A key=value pair without '=' prints a warning but still succeeds
	// (the remaining config is saved unchanged).
	var err error
	output := captureOutput(func() {
		err = HandleAutoUpgrade([]string{"config", "not-a-pair"})
	})
	if err != nil {
		t.Fatalf("HandleAutoUpgrade([config not-a-pair]) = %v, want nil", err)
	}
	if !strings.Contains(output, "Invalid config format") {
		t.Errorf("expected invalid-format warning, got: %s", output)
	}
}

func TestUpdateAutoUpgradeAutoMergeDisabled(t *testing.T) {
	// Default config has auto-merge disabled, so the subcommand fails fast
	// without touching git or the network.
	t.Setenv("HOME", t.TempDir())

	var err error
	output := captureOutput(func() {
		err = HandleAutoUpgrade([]string{"auto-merge"})
	})
	if code := exitCodeForErr(err); code != 1 {
		t.Errorf("HandleAutoUpgrade([auto-merge]) exit code = %d, want 1 (err=%v)", code, err)
	}
	if !strings.Contains(output, "Auto-merge failed") {
		t.Errorf("expected auto-merge failure message, got: %s", output)
	}
}

func TestUpdateConfigKey(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
		apply func(cfg *autoupgrade.UpgradeConfig) // expected mutation
	}{
		{"mode auto", "mode", "auto", func(c *autoupgrade.UpgradeConfig) { c.Mode = autoupgrade.ModeAuto }},
		{"mode monitor", "mode", "monitor", func(c *autoupgrade.UpgradeConfig) { c.Mode = autoupgrade.ModeMonitor }},
		{"mode manual", "mode", "manual", func(c *autoupgrade.UpgradeConfig) { c.Mode = autoupgrade.ModeManual }},
		{"mode unknown value is a no-op", "mode", "bogus", func(*autoupgrade.UpgradeConfig) {}},
		{"mode key is case-insensitive", "MODE", "AUTO", func(c *autoupgrade.UpgradeConfig) { c.Mode = autoupgrade.ModeAuto }},
		{"auto_update true", "auto_update", "true", func(c *autoupgrade.UpgradeConfig) { c.AutoUpdateEnabled = true }},
		{"auto_update false", "auto_update", "false", func(*autoupgrade.UpgradeConfig) {}},
		{"auto-update alias", "auto-update", "true", func(c *autoupgrade.UpgradeConfig) { c.AutoUpdateEnabled = true }},
		{"autoupdate alias", "autoupdate", "true", func(c *autoupgrade.UpgradeConfig) { c.AutoUpdateEnabled = true }},
		{"auto_merge true", "auto_merge", "true", func(c *autoupgrade.UpgradeConfig) { c.AutoMergeEnabled = true }},
		{"auto-merge alias", "auto-merge", "true", func(c *autoupgrade.UpgradeConfig) { c.AutoMergeEnabled = true }},
		{"automerge alias", "automerge", "true", func(c *autoupgrade.UpgradeConfig) { c.AutoMergeEnabled = true }},
		{"interval", "interval", "6h", func(c *autoupgrade.UpgradeConfig) { c.CheckInterval = "6h" }},
		{"check_interval alias", "check_interval", "12h", func(c *autoupgrade.UpgradeConfig) { c.CheckInterval = "12h" }},
		{"backup false", "backup", "false", func(c *autoupgrade.UpgradeConfig) { c.BackupBeforeUpgrade = false }},
		{"backup_before_upgrade alias", "backup_before_upgrade", "false", func(c *autoupgrade.UpgradeConfig) { c.BackupBeforeUpgrade = false }},
		{"rollback false", "rollback", "false", func(c *autoupgrade.UpgradeConfig) { c.RollbackOnFailure = false }},
		{"rollback_on_failure alias", "rollback_on_failure", "false", func(c *autoupgrade.UpgradeConfig) { c.RollbackOnFailure = false }},
		{"unknown key is a no-op", "nope", "x", func(*autoupgrade.UpgradeConfig) {}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := autoupgrade.UpgradeConfig{
				Mode:                autoupgrade.ModeManual,
				CheckInterval:       "24h",
				BackupBeforeUpgrade: true,
				RollbackOnFailure:   true,
			}
			want := autoupgrade.UpgradeConfig{
				Mode:                autoupgrade.ModeManual,
				CheckInterval:       "24h",
				BackupBeforeUpgrade: true,
				RollbackOnFailure:   true,
			}
			updateConfigKey(&got, tt.key, tt.value)
			tt.apply(&want)
			if got != want {
				t.Errorf("updateConfigKey(%q, %q) = %+v, want %+v", tt.key, tt.value, got, want)
			}
		})
	}
}

func TestUpdateNetworkHint(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"private address rejection", errors.New(`Get "https://github.com": private address 10.0.0.1 is not allowed`), "AFLARE_SELF_UPDATE_ALLOW_PRIVATE"},
		{"loopback address rejection", errors.New("dial tcp: loopback address not allowed"), "AFLARE_SELF_UPDATE_ALLOW_PRIVATE"},
		{"generic not-allowed rejection", errors.New("connect: request is not allowed"), "AFLARE_SELF_UPDATE_ALLOW_PRIVATE"},
		{"generic dial failure", errors.New("dial tcp: connection refused"), "HTTPS_PROXY"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := updateNetworkHint(tt.err)
			if !strings.Contains(got, tt.want) {
				t.Errorf("updateNetworkHint(%v) = %q, want it to contain %q", tt.err, got, tt.want)
			}
		})
	}
}

func TestUpdatePrintAutoUpgradeUsage(t *testing.T) {
	output := captureOutput(func() {
		PrintAutoUpgradeUsage()
	})
	if !strings.Contains(output, "Usage: aflare autoupgrade <command>") {
		t.Errorf("expected usage header, got: %s", output)
	}
	for _, sub := range []string{"status", "enable", "disable", "monitor", "run", "config", "auto-merge"} {
		if !strings.Contains(output, sub) {
			t.Errorf("usage output missing %q, got: %s", sub, output)
		}
	}
}

func TestUpdatePrintUpdateNoticeAsyncOff(t *testing.T) {
	t.Run("non-positive timeout returns immediately", func(t *testing.T) {
		var buf bytes.Buffer
		PrintUpdateNoticeAsync(&buf, 0)
		PrintUpdateNoticeAsync(&buf, -time.Second)
		if buf.Len() != 0 {
			t.Errorf("expected no output, got %q", buf.String())
		}
	})
	t.Run("legacy opt-out env", func(t *testing.T) {
		t.Setenv("AFLARE_NO_UPDATE_CHECK", "1")
		t.Setenv("AFLARE_UPDATE_CHECK", "")
		var buf bytes.Buffer
		PrintUpdateNoticeAsync(&buf, time.Second)
		if buf.Len() != 0 {
			t.Errorf("expected no output, got %q", buf.String())
		}
	})
	t.Run("opt-in unset means default off", func(t *testing.T) {
		t.Setenv("AFLARE_NO_UPDATE_CHECK", "")
		t.Setenv("AFLARE_UPDATE_CHECK", "")
		var buf bytes.Buffer
		PrintUpdateNoticeAsync(&buf, time.Second)
		if buf.Len() != 0 {
			t.Errorf("expected no output, got %q", buf.String())
		}
	})
}
