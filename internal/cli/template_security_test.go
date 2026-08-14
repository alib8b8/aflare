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

import "testing"

func TestValidateTemplateNameComponent(t *testing.T) {
	valid := []string{
		"my-workflow",
		"ssl-cert-checker",
		"devops_infra",
		"name123",
	}
	for _, name := range valid {
		if err := validateTemplateNameComponent(name, "name"); err != nil {
			t.Errorf("validateTemplateNameComponent(%q) = %v, want nil", name, err)
		}
	}

	invalid := []struct {
		name string
		in   string
	}{
		{"empty", ""},
		{"path separator slash", "a/b"},
		{"path separator backslash", "a\\b"},
		{"parent traversal", ".."},
		{"embedded parent", "a/../b"},
		{"leading dot", ".hidden"},
		{"null byte", "name\x00evil"},
		{"windows drive", "C:evil"},
		{"too long", string(make([]byte, 129))},
	}
	for _, c := range invalid {
		// Initialize the "too long" case with valid chars.
		if c.name == "too long" {
			for i := range c.in {
				c.in = c.in[:i] + "a" + c.in[i+1:]
			}
		}
		if err := validateTemplateNameComponent(c.in, "name"); err == nil {
			t.Errorf("validateTemplateNameComponent(%q) = nil, want error (%s)", c.in, c.name)
		}
	}
}
