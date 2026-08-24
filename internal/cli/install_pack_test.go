// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​​​​‌​‌​‌‌​‌​​​​‌​​‌​​‌‌​​‌‌​‌‌‌​‌‌​​‌‌‌‌​​‌​‌​‌‌‌​​‌‌‌‌‌​​‌​​​‌‌‌​​​​​​​​​​​​​​​​‌​‌​​‌​​​​​​‌‌​​⁠
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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// snapshotInstalledPacks preserves the contents of the installed-packs
// manifest directory for the duration of the test: manifests created by the
// code under test are removed on cleanup, pre-existing ones are restored.
//
// The directory cannot be redirected via AFLARE_DATA because meta caches
// DataDir with sync.Once (it may already point at the real user home by the
// time these tests run), so the resolved directory is snapshotted instead.
func snapshotInstalledPacks(t *testing.T) {
	t.Helper()
	dir := installedPacksDir()
	prev := map[string][]byte{}
	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if data, err := os.ReadFile(filepath.Join(dir, e.Name())); err == nil {
				prev[e.Name()] = data
			}
		}
	}
	t.Cleanup(func() {
		if entries, err := os.ReadDir(dir); err == nil {
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				if _, ok := prev[e.Name()]; !ok {
					_ = os.Remove(filepath.Join(dir, e.Name()))
				}
			}
		}
		for name, data := range prev {
			_ = os.WriteFile(filepath.Join(dir, name), data, 0o644)
		}
	})
}

// seedInstalledPackManifest writes a pack installation manifest as if the
// given pack had been installed previously.
func seedInstalledPackManifest(t *testing.T, m *installedPackManifest) {
	t.Helper()
	snapshotInstalledPacks(t)
	if err := saveInstalledPack(m); err != nil {
		t.Fatalf("seedInstalledPackManifest: %v", err)
	}
}

func TestInstallPackNoArgs(t *testing.T) {
	var err error
	out := captureOutput(func() {
		err = HandleInstallPack(nil)
	})
	if code := exitCodeForErr(err); code != 1 {
		t.Errorf("HandleInstallPack(nil) exit code = %d, want 1 (err=%v)", code, err)
	}
	if !strings.Contains(out, "Usage: aflare install-pack") {
		t.Errorf("expected usage output, got:\n%s", out)
	}
	if !strings.Contains(out, "Available scenario packs") {
		t.Errorf("expected pack list in usage output, got:\n%s", out)
	}
}

func TestInstallPackHelp(t *testing.T) {
	for _, arg := range []string{"--help", "-h"} {
		var err error
		out := captureOutput(func() {
			err = HandleInstallPack([]string{arg})
		})
		if code := exitCodeForErr(err); code != 0 {
			t.Errorf("HandleInstallPack(%s) exit code = %d, want 0 (err=%v)", arg, code, err)
		}
		if !strings.Contains(out, "Usage: aflare install-pack") {
			t.Errorf("expected usage output for %s, got:\n%s", arg, out)
		}
	}
}

func TestInstallPackList(t *testing.T) {
	// Mark the devops pack as installed so the "*" marker branch runs too.
	seedInstalledPackManifest(t, &installedPackManifest{
		Pack:        "devops",
		Description: "CI/CD, infrastructure monitoring, deployment automation",
		InstalledAt: "2026-01-02T03:04:05Z",
	})

	for _, arg := range []string{"--list", "-l"} {
		var err error
		out := captureOutput(func() {
			err = HandleInstallPack([]string{arg})
		})
		if code := exitCodeForErr(err); code != 0 {
			t.Errorf("HandleInstallPack(%s) exit code = %d, want 0 (err=%v)", arg, code, err)
		}
		if !strings.Contains(out, "Available scenario packs") {
			t.Errorf("expected pack list header for %s, got:\n%s", arg, out)
		}
		// Pack definitions come from the in-memory packs registry.
		for _, pack := range []string{"devops", "security", "finance", "all"} {
			if !strings.Contains(out, pack) {
				t.Errorf("expected pack %q in list output for %s, got:\n%s", pack, arg, out)
			}
		}
		if !strings.Contains(out, "* devops") {
			t.Errorf("expected installed marker next to devops for %s, got:\n%s", arg, out)
		}
		if !strings.Contains(out, "* = installed") {
			t.Errorf("expected installed-marker legend for %s, got:\n%s", arg, out)
		}
	}
}

func TestInstallPackUnknown(t *testing.T) {
	var err error
	out := captureOutput(func() {
		err = HandleInstallPack([]string{"zz-no-such-pack"})
	})
	if code := exitCodeForErr(err); code != 1 {
		t.Errorf("HandleInstallPack(unknown) exit code = %d, want 1 (err=%v)", code, err)
	}
	if !strings.Contains(out, `pack "zz-no-such-pack" not found`) {
		t.Errorf("expected not-found message, got:\n%s", out)
	}
	// The error path also prints the available packs as a hint.
	if !strings.Contains(out, "Available packs:") {
		t.Errorf("expected available-packs hint, got:\n%s", out)
	}
}

func TestInstallPackNoTemplates(t *testing.T) {
	// An empty registry (the seeded index suppresses the embedded
	// catalog) makes every pack match zero templates.
	seedTemplatesRegistry(t)
	snapshotInstalledPacks(t)
	if err := os.Remove(filepath.Join(installedPacksDir(), "devops.json")); err != nil && !os.IsNotExist(err) {
		t.Fatalf("remove stale devops manifest: %v", err)
	}

	var err error
	out := captureOutput(func() {
		err = HandleInstallPack([]string{"devops"})
	})
	if code := exitCodeForErr(err); code != 1 {
		t.Errorf("HandleInstallPack(no templates) exit code = %d, want 1 (err=%v)", code, err)
	}
	if !strings.Contains(out, `No templates found for pack "devops"`) {
		t.Errorf("expected no-templates message, got:\n%s", out)
	}
}

func TestInstallPackCorruptManifest(t *testing.T) {
	// A corrupt installed-pack manifest makes loadInstalledPack fail; the
	// handler then falls through and reinstalls the pack.
	seedTemplatesRegistry(t,
		seedTemplate{ID: "devops-infra/zz-seed-easy", Workflow: seedEasyWorkflow},
	)
	snapshotInstalledPacks(t)
	corrupt := filepath.Join(installedPacksDir(), "devops.json")
	if err := os.WriteFile(corrupt, []byte("{not valid json"), 0o644); err != nil {
		t.Fatalf("write corrupt manifest: %v", err)
	}

	var err error
	out := captureOutput(func() {
		err = HandleInstallPack([]string{"devops"})
	})
	if code := exitCodeForErr(err); code != 0 {
		t.Fatalf("HandleInstallPack(corrupt manifest) exit code = %d, want 0 (err=%v)", code, err)
	}
	if !strings.Contains(out, "Installing pack: devops") {
		t.Errorf("expected fresh install after corrupt manifest, got:\n%s", out)
	}
	// The install rewrites the manifest with valid JSON.
	data, readErr := os.ReadFile(corrupt)
	if readErr != nil {
		t.Fatalf("read rewritten manifest: %v", readErr)
	}
	if strings.Contains(string(data), "not valid json") {
		t.Errorf("manifest was not rewritten, still corrupt:\n%s", data)
	}
}

func TestInstallPackAlreadyInstalled(t *testing.T) {
	// A devops-infra template so the registry has content, plus a manifest
	// claiming the pack was already installed.
	seedTemplatesRegistry(t,
		seedTemplate{ID: "devops-infra/zz-seed-easy", Workflow: seedEasyWorkflow},
	)
	seedInstalledPackManifest(t, &installedPackManifest{
		Pack:         "devops",
		Description:  "CI/CD, infrastructure monitoring, deployment automation",
		InstalledAt:  "2026-01-02T03:04:05Z",
		Templates:    3,
		Capabilities: []string{"reflection", "bdi", "utility"},
		Categories:   []string{"devops-infra"},
	})

	var err error
	out := captureOutput(func() {
		err = HandleInstallPack([]string{"devops"})
	})
	if code := exitCodeForErr(err); code != 0 {
		t.Fatalf("HandleInstallPack(already installed) exit code = %d, want 0 (err=%v)", code, err)
	}
	if !strings.Contains(out, `Pack "devops" is already installed`) {
		t.Errorf("expected already-installed message, got:\n%s", out)
	}
	if !strings.Contains(out, "Use --force to reinstall") {
		t.Errorf("expected reinstall hint, got:\n%s", out)
	}
}

func TestInstallPackSeeded(t *testing.T) {
	// Two devops-infra templates: exactly what the "devops" pack bundles.
	seedTemplatesRegistry(t,
		seedTemplate{ID: "devops-infra/zz-seed-easy", Workflow: seedEasyWorkflow},
		seedTemplate{ID: "devops-infra/zz-seed-static", Difficulty: "easy"},
	)
	snapshotInstalledPacks(t)
	// Start from a clean slate for the packs this test installs (any
	// pre-existing manifests are restored by the snapshot on cleanup).
	for _, pack := range []string{"devops", "all"} {
		if err := os.Remove(filepath.Join(installedPacksDir(), pack+".json")); err != nil && !os.IsNotExist(err) {
			t.Fatalf("remove stale %s manifest: %v", pack, err)
		}
	}

	// Fresh install.
	var err error
	out := captureOutput(func() {
		err = HandleInstallPack([]string{"devops"})
	})
	if code := exitCodeForErr(err); code != 0 {
		t.Fatalf("HandleInstallPack(devops) exit code = %d, want 0 (err=%v)", code, err)
	}
	for _, want := range []string{
		"Installing pack: devops",
		"Templates:    2",
		"Included templates:",
		"devops-infra/zz-seed-easy",
		`Pack "devops" installed successfully. 2 templates ready to use.`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in install output, got:\n%s", want, out)
		}
	}

	// Second run without --force is idempotent.
	var errSecond error
	outSecond := captureOutput(func() {
		errSecond = HandleInstallPack([]string{"devops"})
	})
	if code := exitCodeForErr(errSecond); code != 0 {
		t.Fatalf("HandleInstallPack(devops, again) exit code = %d, want 0 (err=%v)", code, errSecond)
	}
	if !strings.Contains(outSecond, "is already installed") {
		t.Errorf("expected already-installed message, got:\n%s", outSecond)
	}

	// --force reinstalls even though the manifest exists.
	var errForce error
	outForce := captureOutput(func() {
		errForce = HandleInstallPack([]string{"devops", "--force"})
	})
	if code := exitCodeForErr(errForce); code != 0 {
		t.Fatalf("HandleInstallPack(devops, --force) exit code = %d, want 0 (err=%v)", code, errForce)
	}
	if !strings.Contains(outForce, "Reinstalling pack: devops") {
		t.Errorf("expected reinstall header, got:\n%s", outForce)
	}

	// The "all" pack bundles every category (empty Categories list).
	var errAll error
	outAll := captureOutput(func() {
		errAll = HandleInstallPack([]string{"all", "--force"})
	})
	if code := exitCodeForErr(errAll); code != 0 {
		t.Fatalf("HandleInstallPack(all, --force) exit code = %d, want 0 (err=%v)", code, errAll)
	}
	if !strings.Contains(outAll, `Pack "all" installed successfully. 2 templates ready to use.`) {
		t.Errorf("expected all-pack success message, got:\n%s", outAll)
	}
}
