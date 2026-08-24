// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​​‌​​‌‌‌​‌‌​​‌​​‌​​​​​‌‌‌‌‌​​‌‌‌​​​‌​​‌‌​​‌​‌‌‌‌​​​​​​​​​​​​​​​​‌​​‌‌​​‌‌​​​‌​​​⁠
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
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alib8b8/aflare/internal/badge"
)

// captureOutput captures stdout during a function call.
func captureOutput(fn func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestHandleBadge_NoArgs(t *testing.T) {
	output := captureOutput(func() {
		HandleBadge(nil)
	})
	if !strings.Contains(output, "Usage: aflare badge") {
		t.Errorf("expected usage output, got: %s", output)
	}
}

func TestHandleBadge_Help(t *testing.T) {
	for _, arg := range []string{"--help", "-h", "help"} {
		output := captureOutput(func() {
			HandleBadge([]string{arg})
		})
		if !strings.Contains(output, "Usage: aflare badge") {
			t.Errorf("expected usage for %s, got: %s", arg, output)
		}
	}
}

func TestHandleBadge_UnknownSubcommand(t *testing.T) {
	// HandleBadge calls os.Exit(1) for unknown subcommands.
	// We use a recover-based approach to catch the exit.
	exited := false
	origExit := osExit
	osExit = func(code int) {
		exited = true
		panic("exit")
	}
	defer func() { osExit = origExit }()

	func() {
		defer func() { recover() }()
		HandleBadge([]string{"unknown"})
	}()

	if !exited {
		t.Error("expected os.Exit(1) for unknown subcommand")
	}
}

func TestHandleBadge_ShowMissingID(t *testing.T) {
	exited := false
	origExit := osExit
	osExit = func(code int) { exited = true; panic("exit") }
	defer func() { osExit = origExit }()

	func() {
		defer func() { recover() }()
		HandleBadge([]string{"show"})
	}()

	if !exited {
		t.Error("expected os.Exit(1) for 'show' without ID")
	}
}

func TestHandleBadge_ListEmptyStore(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	output := captureOutput(func() {
		handleBadgeList()
	})

	if !strings.Contains(output, "No badges awarded yet") {
		t.Errorf("expected 'No badges awarded yet', got: %s", output)
	}
}

func TestHandleBadge_AwardAndList(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Award a badge.
	awardOutput := captureOutput(func() {
		handleBadgeAward("Alice", "alice@example.com", "Submitted stock-analyzer template", badge.ContributionTemplate)
	})

	if !strings.Contains(awardOutput, "Badge awarded to Alice!") {
		t.Errorf("expected 'Badge awarded to Alice!', got: %s", awardOutput)
	}
	if !strings.Contains(awardOutput, "bronze") {
		t.Errorf("expected bronze badge, got: %s", awardOutput)
	}

	// List should show Alice.
	listOutput := captureOutput(func() {
		handleBadgeList()
	})

	if !strings.Contains(listOutput, "Alice") {
		t.Errorf("expected 'Alice' in list, got: %s", listOutput)
	}
}

func TestHandleBadge_AwardMultiple(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// 3 contributions → Silver.
	for i := 0; i < 3; i++ {
		captureOutput(func() {
			handleBadgeAward("Bob", "bob@example.com", "Contribution", badge.ContributionCode)
		})
	}

	// Show Bob.
	output := captureOutput(func() {
		handleBadgeShow(badge.ContributorID("Bob", "bob@example.com"))
	})

	if !strings.Contains(output, "Bob") {
		t.Errorf("expected 'Bob' in show, got: %s", output)
	}
	if !strings.Contains(output, "silver") {
		t.Errorf("expected silver tier, got: %s", output)
	}
}

func TestHandleBadge_ShowPrefixMatch(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	captureOutput(func() {
		handleBadgeAward("Charlie", "charlie@example.com", "Bug fix", badge.ContributionBugFix)
	})

	cid := badge.ContributorID("Charlie", "charlie@example.com")
	prefix := cid[:8]

	output := captureOutput(func() {
		handleBadgeShow(prefix)
	})

	if !strings.Contains(output, "Charlie") {
		t.Errorf("expected 'Charlie' with prefix match, got: %s", output)
	}
}

func TestHandleBadge_ShowNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	exited := false
	origExit := osExit
	osExit = func(code int) { exited = true; panic("exit") }
	defer func() { osExit = origExit }()

	func() {
		defer func() { recover() }()
		handleBadgeShow("nonexistent")
	}()

	if !exited {
		t.Error("expected os.Exit(1) for nonexistent contributor")
	}
}

func TestHandleBadge_AwardWithTypeFlag(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	output := captureOutput(func() {
		HandleBadge([]string{"award", "Dave", "dave@x.com", "Fixed bug", "--type", "bugfix"})
	})

	if !strings.Contains(output, "Badge awarded to Dave!") {
		t.Errorf("expected 'Badge awarded to Dave!', got: %s", output)
	}
}

func TestHandleBadge_AwardNoNewBadge(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// First contribution → Bronze.
	captureOutput(func() {
		handleBadgeAward("Eve", "eve@example.com", "First template", badge.ContributionTemplate)
	})

	// Second contribution → still Bronze, no new badge.
	output := captureOutput(func() {
		handleBadgeAward("Eve", "eve@example.com", "Second template", badge.ContributionTemplate)
	})

	if !strings.Contains(output, "no new badge tier earned") {
		t.Errorf("expected 'no new badge tier earned', got: %s", output)
	}
}

func TestHandleBadge_ListSorted(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// A: 3 contributions (Silver), B: 1 contribution (Bronze).
	captureOutput(func() { handleBadgeAward("A", "a@x.com", "1", badge.ContributionTemplate) })
	captureOutput(func() { handleBadgeAward("B", "b@x.com", "1", badge.ContributionTemplate) })
	captureOutput(func() { handleBadgeAward("B", "b@x.com", "2", badge.ContributionTemplate) })
	captureOutput(func() { handleBadgeAward("B", "b@x.com", "3", badge.ContributionTemplate) })

	output := captureOutput(func() {
		handleBadgeList()
	})

	// B (3 contributions) should appear before A (1 contribution).
	bIdx := strings.Index(output, "B")
	aIdx := strings.Index(output, "A")
	if bIdx == -1 || aIdx == -1 || bIdx > aIdx {
		t.Errorf("B should appear before A in list, got B at %d, A at %d", bIdx, aIdx)
	}
}

func TestPrintBadgeUsage(t *testing.T) {
	output := captureOutput(func() {
		printBadgeUsage()
	})

	if !strings.Contains(output, "Usage: aflare badge") {
		t.Error("expected usage header")
	}
	if !strings.Contains(output, "Bronze") {
		t.Error("expected Bronze tier info")
	}
	if !strings.Contains(output, "Platinum") {
		t.Error("expected Platinum tier info")
	}
}

func TestHandleBadge_ListWithContributors(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	captureOutput(func() {
		handleBadgeAward("Zoe", "zoe@example.com", "Awesome template", badge.ContributionTemplate)
	})

	output := captureOutput(func() {
		handleBadgeList()
	})

	if !strings.Contains(output, "aflare Community Contributors") {
		t.Errorf("expected header, got: %s", output)
	}
	if !strings.Contains(output, "Zoe") {
		t.Errorf("expected Zoe in list, got: %s", output)
	}
	if !strings.Contains(output, "Total: 1 contributors") {
		t.Errorf("expected total count, got: %s", output)
	}
}

func TestHandleBadge_ShowFullOutput(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	captureOutput(func() {
		handleBadgeAward("Frank", "frank@example.com", "Doc improvement", badge.ContributionDocs)
	})

	cid := badge.ContributorID("Frank", "frank@example.com")
	output := captureOutput(func() {
		handleBadgeShow(cid)
	})

	if !strings.Contains(output, "Frank") {
		t.Error("expected contributor name")
	}
	if !strings.Contains(output, "Badges Earned:") {
		t.Error("expected badges section")
	}
	if !strings.Contains(output, "Total Contributions:") {
		t.Error("expected total contributions")
	}
}

func TestHandleBadge_AwardInsufficientArgs(t *testing.T) {
	exited := false
	origExit := osExit
	osExit = func(code int) { exited = true; panic("exit") }
	defer func() { osExit = origExit }()

	func() {
		defer func() { recover() }()
		HandleBadge([]string{"award", "name"})
	}()

	if !exited {
		t.Error("expected os.Exit(1) for insufficient award args")
	}
}

func TestHandleBadge_ErrorLoadingStore(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a directory where badges.json should be, preventing file creation.
	badgeDir := filepath.Join(tmpDir, ".aflare")
	if err := os.MkdirAll(badgeDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Create badges.json as a directory, which will cause LoadStore to fail.
	if err := os.MkdirAll(filepath.Join(badgeDir, "badges.json"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", tmpDir)

	exited := false
	origExit := osExit
	osExit = func(code int) { exited = true; panic("exit") }
	defer func() { osExit = origExit }()

	func() {
		defer func() { recover() }()
		handleBadgeList()
	}()

	if !exited {
		t.Error("expected os.Exit(1) when badge store cannot be loaded")
	}
}
