// Copyright (c) 2026 aflare Contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation; either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

// Package templates embeds the builtin skill-template library (323 workflow
// templates under templates/) into the binary so that a bare aflare binary —
// without the source tree's templates/ directory on disk — can still list,
// clone, and run templates. This is essential for the local-first / offline
// story: users install a single binary and immediately get the full template
// catalog without a network fetch.
package templates

import "embed"

// Embedded is the embedded templates/ directory tree. It contains, for each
// template, a workflow.yaml, skill.json, and README.md, plus a top-level
// skills-registry.json index. Consumers read from it with fs.ReadFile using
// paths relative to "templates/" (e.g. "templates/business/business-plan/workflow.yaml").
//
//go:embed templates
var Embedded embed.FS
