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

// packs defines scenario-based installation packs that bundle related templates
// with recommended capability configurations. Each pack targets a specific
// domain (e.g., devops, security, finance) and lowers the first-time experience
// barrier by providing a curated, ready-to-use set of tools.
package packs

import "sort"

// ScenarioPack bundles a set of skill templates and recommended capabilities
// for a specific domain.
type ScenarioPack struct {
	Name         string   // human-readable pack name
	Description  string   // one-line description
	Categories   []string // skill categories included
	Capabilities []string // recommended capabilities
}

// AllPacks returns all available scenario packs sorted by name.
func AllPacks() []ScenarioPack {
	packs := []ScenarioPack{
		{
			Name:         "devops",
			Description:  "CI/CD, infrastructure monitoring, deployment automation",
			Categories:   []string{"devops-infra"},
			Capabilities: []string{"reflection", "bdi", "utility"},
		},
		{
			Name:         "security",
			Description:  "Security audits, vulnerability scanning, compliance checks",
			Categories:   []string{"software-engineering", "devops-infra"},
			Capabilities: []string{"reflection", "human-in-loop"},
		},
		{
			Name:         "finance",
			Description:  "Financial analysis, risk assessment, trading automation",
			Categories:   []string{"finance"},
			Capabilities: []string{"reflection", "utility", "bdi"},
		},
		{
			Name:         "development",
			Description:  "Code review, testing, CI/CD, software engineering workflows",
			Categories:   []string{"software-engineering", "devops-infra"},
			Capabilities: []string{"reflection", "planning"},
		},
		{
			Name:         "data",
			Description:  "Data processing, ETL pipelines, AI/ML workflows",
			Categories:   []string{"data-ai"},
			Capabilities: []string{"reflection", "workflow"},
		},
		{
			Name:         "business",
			Description:  "Business planning, contract review, churn analysis",
			Categories:   []string{"business"},
			Capabilities: []string{"reflection", "utility"},
		},
		{
			Name:         "marketing",
			Description:  "Content creation, SEO, social media, campaign management",
			Categories:   []string{"marketing", "content-creative"},
			Capabilities: []string{"reflection", "simulation"},
		},
		{
			Name:         "healthcare",
			Description:  "Medical analysis, patient data, diagnostic workflows",
			Categories:   []string{"healthcare"},
			Capabilities: []string{"human-in-loop", "reflection"},
		},
		{
			Name:         "hr",
			Description:  "Recruitment, onboarding, performance reviews",
			Categories:   []string{"hr"},
			Capabilities: []string{"workflow", "planning"},
		},
		{
			Name:         "legal",
			Description:  "Contract analysis, compliance, legal document review",
			Categories:   []string{"legal"},
			Capabilities: []string{"human-in-loop", "reflection"},
		},
		{
			Name:         "ecommerce",
			Description:  "Product management, inventory, order processing",
			Categories:   []string{"ecommerce"},
			Capabilities: []string{"reflection", "bdi"},
		},
		{
			Name:         "all",
			Description:  "All available templates across every category",
			Categories:   []string{},
			Capabilities: []string{"reflection", "bdi", "utility", "memory", "planning"},
		},
	}

	sort.Slice(packs, func(i, j int) bool {
		return packs[i].Name < packs[j].Name
	})
	return packs
}

// GetPack returns a scenario pack by name, or nil if not found.
func GetPack(name string) *ScenarioPack {
	for _, p := range AllPacks() {
		if p.Name == name {
			return &p
		}
	}
	return nil
}
