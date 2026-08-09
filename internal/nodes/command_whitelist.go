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

package nodes

// SafeCommandWhitelist defines the shared set of safe commands allowed in
// both execute and sandbox nodes when allowlist mode is enabled.
//
// These are read-only (or read-mostly) commands. sed and awk are included
// for text processing but their -i (in-place edit) flag is explicitly blocked
// in the allowlist enforcement path (execute.go) to prevent file modification.
//
// curl, wget, rm, git, go are intentionally excluded — use:
//   - http_request node for HTTP operations
//   - file_write node for file operations
//   - git node for git operations
var SafeCommandWhitelist = map[string]bool{
	"ls": true, "cat": true, "head": true, "tail": true,
	"wc": true, "grep": true, "awk": true, "sed": true,
	"find": true, "sort": true, "uniq": true, "cut": true,
	"tr": true, "echo": true, "date": true, "pwd": true,
	"whoami": true, "uname": true, "df": true, "du": true,
}
