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

package packs

import "testing"

func TestAllPacks_SortedByName(t *testing.T) {
	all := AllPacks()
	if len(all) < 2 {
		t.Fatalf("expected multiple packs, got %d", len(all))
	}
	for i := 1; i < len(all); i++ {
		if all[i-1].Name >= all[i].Name {
			t.Errorf("packs not sorted: %q >= %q", all[i-1].Name, all[i].Name)
		}
	}
}

func TestAllPacks_FieldsComplete(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range AllPacks() {
		if p.Name == "" {
			t.Error("pack with empty Name")
		}
		if seen[p.Name] {
			t.Errorf("duplicate pack name: %q", p.Name)
		}
		seen[p.Name] = true
		if p.Description == "" {
			t.Errorf("pack %q has empty Description", p.Name)
		}
		if len(p.Capabilities) == 0 {
			t.Errorf("pack %q has no Capabilities", p.Name)
		}
	}
}

func TestAllPacks_ContainsAllPack(t *testing.T) {
	if GetPack("all") == nil {
		t.Error("expected a pack named \"all\" for the install-everything path")
	}
}

func TestGetPack_Found(t *testing.T) {
	p := GetPack("devops")
	if p == nil {
		t.Fatal("GetPack(devops) = nil, want the devops pack")
	}
	if p.Name != "devops" {
		t.Errorf("Name = %q, want %q", p.Name, "devops")
	}
	if len(p.Categories) == 0 {
		t.Error("devops pack should list at least one category")
	}
}

func TestGetPack_NotFound(t *testing.T) {
	if GetPack("does-not-exist") != nil {
		t.Error("GetPack for unknown name should return nil")
	}
	if GetPack("") != nil {
		t.Error("GetPack(\"\") should return nil")
	}
}
