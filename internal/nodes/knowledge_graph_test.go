// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌‌​​​​​‌‌​‌​‌‌‌‌​​​​​​‌‌‌​​​‌‌​​​​‌​‌‌​​‌‌​‌‌​​‌‌​‌​‌​‌‌​​​‌‌‌‌​​‌​‌​​​​​​​​​​​​​​​​​​​‌​​​​‌​​​‌‌‌​⁠
// aflare
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

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTraverseGraphPathLimit verifies that BFS traversal rejects paths
// longer than maxTraversalPathLen (the overflow-safe allocation guard)
// while normal depths traverse fine.
func TestTraverseGraphPathLimit(t *testing.T) {
	oldWorkDir := workDir
	workDir = t.TempDir()
	defer func() { workDir = oldWorkDir }()

	// Chain graph long enough for a BFS path to exceed the cap.
	chainLen := maxTraversalPathLen + 2
	kg := NewKnowledgeGraph()
	for i := 0; i <= chainLen; i++ {
		kg.AddEntity(fmt.Sprintf("n%d", i), "node", nil)
	}
	for i := 0; i < chainLen; i++ {
		kg.AddRelation(fmt.Sprintf("n%d", i), fmt.Sprintf("n%d", i+1), "next", 1.0)
	}
	data, err := json.Marshal(kg)
	if err != nil {
		t.Fatalf("marshal graph: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workDir, "chain.json"), data, 0644); err != nil {
		t.Fatalf("write graph: %v", err)
	}

	tests := []struct {
		name     string
		maxDepth int
		topK     int
		wantErr  bool
	}{
		{name: "normal depth", maxDepth: 2, topK: 10},
		{name: "path over limit", maxDepth: maxTraversalPathLen + 2, topK: maxTraversalPathLen + 2, wantErr: true},
	}

	n := &KnowledgeGraphNode{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := n.traverseGraph("chain.json", "n0", tt.maxDepth, tt.topK, "markdown")
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected path-length error, got nil")
				}
				if !strings.Contains(err.Error(), "traversal path") {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(out, "n0") {
				t.Errorf("expected start entity in output, got:\n%s", out)
			}
		})
	}
}
