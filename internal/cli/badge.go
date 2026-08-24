// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​‌​​‌‌‌‌​​​​‌​‌​‌​​‌‌‌​​‌‌​‌​‌‌‌​‌​​‌​‌‌‌​​​‌​‌​​​​​​​​​​​​​​​​​‌​​​‌​​​‌‌​‌‌​‌‌⁠
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

	"github.com/alib8b8/aflare/internal/badge"
)

// osExit is a package-level variable for testing os.Exit calls.
var osExit = os.Exit

// HandleBadge handles the "badge" command.
// Subcommands: list, show, award
func HandleBadge(args []string) {
	if len(args) == 0 {
		printBadgeUsage()
		return
	}

	subCmd := args[0]
	switch subCmd {
	case "list":
		handleBadgeList()
	case "show":
		if len(args) < 2 {
			fmt.Println("Usage: aflare badge show <contributor-id>")
			osExit(1)
		}
		handleBadgeShow(args[1])
	case "award":
		contType := badge.ContributionTemplate
		// Filter out --type flag and its value, then extract positional args.
		var positional []string
		for i := 0; i < len(args); i++ {
			if args[i] == "--type" && i+1 < len(args) {
				contType = badge.ContributionType(args[i+1])
				i++
			} else if !strings.HasPrefix(args[i], "-") {
				positional = append(positional, args[i])
			}
		}
		// First positional is "award", then name, email, reason.
		if len(positional) < 4 {
			fmt.Println("Usage: aflare badge award <name> <email> <reason> [--type template|code|docs|bugfix]")
			osExit(1)
		}
		handleBadgeAward(positional[1], positional[2], positional[3], contType)
	case "--help", "-h", "help":
		printBadgeUsage()
	default:
		fmt.Printf("Unknown badge subcommand: %s\n\n", subCmd)
		printBadgeUsage()
		osExit(1)
	}
}

func printBadgeUsage() {
	fmt.Println("Usage: aflare badge <subcommand> [options]")
	fmt.Println()
	fmt.Println("Subcommands:")
	fmt.Println("  list                  List all contributors and their badges")
	fmt.Println("  show <id>             Show a specific contributor's badges")
	fmt.Println("  award <name> <email> <reason>  Manually award a contribution badge")
	fmt.Println()
	fmt.Println("Badge Tiers:")
	fmt.Printf("  %s Bronze   — 1+ contributions\n", badge.TierEmoji[badge.TierBronze])
	fmt.Printf("  %s Silver   — 3+ contributions\n", badge.TierEmoji[badge.TierSilver])
	fmt.Printf("  %s Gold     — 6+ contributions\n", badge.TierEmoji[badge.TierGold])
	fmt.Printf("  %s Platinum — 10+ contributions\n", badge.TierEmoji[badge.TierPlatinum])
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  aflare badge list")
	fmt.Println("  aflare badge show abc12345")
	fmt.Println("  aflare badge award \"Alice\" \"alice@example.com\" \"Submitted stock-analyzer template\"")
	fmt.Println("  aflare badge award \"Bob\" \"bob@example.com\" \"Fixed critical bug\" --type bugfix")
}

func handleBadgeList() {
	store, err := badge.LoadStore(badge.DefaultStorePath())
	if err != nil {
		fmt.Printf("Error loading badge store: %v\n", err)
		osExit(1)
	}

	contributors := store.ListContributors()
	if len(contributors) == 0 {
		fmt.Println("No badges awarded yet. Be the first contributor!")
		fmt.Println()
		fmt.Println("Submit a template or code contribution to earn badges:")
		fmt.Println("  aflare template submit <workflow.yaml>")
		return
	}

	fmt.Println("aflare Community Contributors")
	fmt.Println(strings.Repeat("─", 60))
	fmt.Printf("%-4s %-20s %-10s %-8s %s\n", "Rank", "Name", "Tier", "Count", "Latest Badge")
	fmt.Println(strings.Repeat("─", 60))

	for i, c := range contributors {
		tier := c.CurrentTier()
		emoji := badge.TierEmoji[tier]
		latestBadge := ""
		if len(c.Badges) > 0 {
			last := c.Badges[len(c.Badges)-1]
			latestBadge = last.Reason
		}
		fmt.Printf("#%-3d %-20s %s%-7s %-8d %s\n",
			i+1, truncate(c.Name, 20), emoji, tier, c.ContributionCount, truncate(latestBadge, 30))
	}
	fmt.Println(strings.Repeat("─", 60))
	fmt.Println()
	fmt.Printf("Total: %d contributors\n", len(contributors))
}

//nolint:staticcheck // osExit is os.Exit wrapper; staticcheck can't track termination
func handleBadgeShow(id string) {
	store, err := badge.LoadStore(badge.DefaultStorePath())
	if err != nil {
		fmt.Printf("Error loading badge store: %v\n", err)
		osExit(1)
	}

	rec := store.GetContributor(id)
	if rec == nil {
		// Try prefix match.
		for _, c := range store.ListContributors() {
			if strings.HasPrefix(c.ID, id) {
				rec = c
				break
			}
		}
	}

	if rec == nil {
		fmt.Printf("No contributor found with ID prefix: %s\n", id)
		fmt.Println("Use 'aflare badge list' to see all contributors.")
		osExit(1)
	}

	tier := rec.CurrentTier()
	emoji := badge.TierEmoji[tier]

	fmt.Printf("Contributor: %s  %s%s\n", rec.Name, emoji, tier)
	fmt.Printf("ID:          %s\n", rec.ID)
	fmt.Println(strings.Repeat("─", 50))
	fmt.Printf("Total Contributions: %d\n", rec.ContributionCount)
	fmt.Printf("First Contribution:  %s\n", rec.FirstContribution.Format("2006-01-02"))
	fmt.Printf("Last Contribution:   %s\n", rec.LastContribution.Format("2006-01-02"))
	fmt.Println()
	fmt.Println("Badges Earned:")
	for _, b := range rec.Badges {
		fmt.Printf("  %s\n", badge.FormatBadge(b))
	}
}

func handleBadgeAward(name, email, reason string, contType badge.ContributionType) {
	store, err := badge.LoadStore(badge.DefaultStorePath())
	if err != nil {
		fmt.Printf("Error loading badge store: %v\n", err)
		osExit(1)
	}

	cid := badge.ContributorID(name, email)
	b, awarded := store.RecordContribution(cid, name, reason, contType)

	if err := store.Save(); err != nil {
		fmt.Printf("Error saving badge store: %v\n", err)
		osExit(1)
	}

	if awarded {
		fmt.Printf("Badge awarded to %s!\n", name)
		fmt.Printf("  %s\n", badge.FormatBadge(*b))
	} else {
		fmt.Printf("Contribution recorded for %s (no new badge tier earned).\n", name)
	}

	rec := store.GetContributor(cid)
	fmt.Printf("  Total contributions: %d  |  Current tier: %s%s\n",
		rec.ContributionCount, badge.TierEmoji[rec.CurrentTier()], rec.CurrentTier())
}
