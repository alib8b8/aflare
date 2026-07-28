// Copyright (c) 2026 llm-box Contributors
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

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// makeZip writes a zip archive to path whose entries are (name -> content).
func makeZip(t *testing.T, path string, entries map[string]string) {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range entries {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

// minimalDocxXML builds a word/document.xml with a heading, a paragraph,
// a tab/br, and a 2x2 table.
const minimalDocxXML = `<?xml version="1.0"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p><w:pPr><w:pStyle w:val="Heading1"/></w:pPr><w:r><w:t>Title</w:t></w:r></w:p>
    <w:p><w:r><w:t>Hello</w:t></w:r><w:r><w:tab/><w:t>tabbed</w:t></w:r></w:p>
    <w:p><w:r><w:t>Line1</w:t><w:br/><w:t>Line2</w:t></w:r></w:p>
    <w:tbl>
      <w:tr><w:tc><w:p><w:r><w:t>A</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>B</w:t></w:r></w:p></w:tc></w:tr>
      <w:tr><w:tc><w:p><w:r><w:t>1</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>2</w:t></w:r></w:p></w:tc></w:tr>
    </w:tbl>
  </w:body>
</w:document>`

func TestOfficeNode_DocxMarkdown(t *testing.T) {
	oldWorkDir := workDir
	workDir = t.TempDir()
	defer func() { workDir = oldWorkDir }()
	path := filepath.Join(workDir, "test.docx")
	makeZip(t, path, map[string]string{"word/document.xml": minimalDocxXML})

	n := &OfficeNode{}
	out, err := n.Execute(context.Background(), "", map[string]string{"path": "test.docx"})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !strings.Contains(out, "# Title") {
		t.Errorf("expected heading, got:\n%s", out)
	}
	if !strings.Contains(out, "Hello") {
		t.Errorf("expected paragraph text, got:\n%s", out)
	}
	if !strings.Contains(out, "tabbed") {
		t.Errorf("expected text after tab, got:\n%s", out)
	}
	if !strings.Contains(out, "Line1") || !strings.Contains(out, "Line2") {
		t.Errorf("expected br-separated text, got:\n%s", out)
	}
	if !strings.Contains(out, "| A | B |") {
		t.Errorf("expected table header row, got:\n%s", out)
	}
	if !strings.Contains(out, "| 1 | 2 |") {
		t.Errorf("expected table data row, got:\n%s", out)
	}
}

func TestOfficeNode_DocxText(t *testing.T) {
	oldWorkDir := workDir
	workDir = t.TempDir()
	defer func() { workDir = oldWorkDir }()
	path := filepath.Join(workDir, "test.docx")
	makeZip(t, path, map[string]string{"word/document.xml": minimalDocxXML})

	n := &OfficeNode{}
	out, err := n.Execute(context.Background(), "", map[string]string{"path": "test.docx", "output": "text"})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	// text mode should NOT contain markdown heading marker
	if strings.Contains(out, "# Title") {
		t.Errorf("text output should not contain markdown heading, got:\n%s", out)
	}
	if !strings.Contains(out, "A\tB") {
		t.Errorf("expected tab-separated table row, got:\n%s", out)
	}
}

func TestOfficeNode_DocxJSON(t *testing.T) {
	oldWorkDir := workDir
	workDir = t.TempDir()
	defer func() { workDir = oldWorkDir }()
	path := filepath.Join(workDir, "test.docx")
	makeZip(t, path, map[string]string{"word/document.xml": minimalDocxXML})

	n := &OfficeNode{}
	out, err := n.Execute(context.Background(), "", map[string]string{"path": "test.docx", "output": "json"})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !strings.HasPrefix(out, "{\"blocks\":[") {
		t.Errorf("expected JSON start, got:\n%s", out[:40])
	}
	if !strings.Contains(out, `"kind":"para"`) {
		t.Errorf("expected para block, got:\n%s", out)
	}
	if !strings.Contains(out, `"kind":"table"`) {
		t.Errorf("expected table block, got:\n%s", out)
	}
}

// minimalXlsxFiles builds a workbook with one sheet, a shared-strings
// table, and a couple of numeric + shared-string cells.
var minimalXlsxFiles = map[string]string{
	"xl/workbook.xml": `<?xml version="1.0"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheets>
    <sheet name="Sheet1" sheetId="1" r:id="rId1" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"/>
  </sheets>
</workbook>`,
	"xl/sharedStrings.xml": `<?xml version="1.0"?>
<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <si><t>Name</t></si>
  <si><t>Alice</t></si>
</sst>`,
	"xl/worksheets/sheet1.xml": `<?xml version="1.0"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheetData>
    <row>
      <c t="s"><v>0</v></c>
      <c><v>42</v></c>
    </row>
    <row>
      <c t="s"><v>1</v></c>
      <c><v>30</v></c>
    </row>
  </sheetData>
</worksheet>`,
}

func TestOfficeNode_XlsxMarkdown(t *testing.T) {
	oldWorkDir := workDir
	workDir = t.TempDir()
	defer func() { workDir = oldWorkDir }()
	path := filepath.Join(workDir, "test.xlsx")
	makeZip(t, path, minimalXlsxFiles)

	n := &OfficeNode{}
	out, err := n.Execute(context.Background(), "", map[string]string{"path": "test.xlsx"})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !strings.Contains(out, "## Sheet1") {
		t.Errorf("expected sheet heading, got:\n%s", out)
	}
	// shared string "Name" should be resolved, not index 0
	if !strings.Contains(out, "| Name | 42 |") {
		t.Errorf("expected resolved shared string + number, got:\n%s", out)
	}
	if !strings.Contains(out, "| Alice | 30 |") {
		t.Errorf("expected second row, got:\n%s", out)
	}
}

func TestOfficeNode_XlsxJSON(t *testing.T) {
	oldWorkDir := workDir
	workDir = t.TempDir()
	defer func() { workDir = oldWorkDir }()
	path := filepath.Join(workDir, "test.xlsx")
	makeZip(t, path, minimalXlsxFiles)

	n := &OfficeNode{}
	out, err := n.Execute(context.Background(), "", map[string]string{"path": "test.xlsx", "output": "json"})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !strings.Contains(out, `"name":"Sheet1"`) {
		t.Errorf("expected sheet name, got:\n%s", out)
	}
	if !strings.Contains(out, `"Name"`) || !strings.Contains(out, `"Alice"`) {
		t.Errorf("expected shared-string values in JSON, got:\n%s", out)
	}
}

func TestOfficeNode_XlsxSheetNotFound(t *testing.T) {
	oldWorkDir := workDir
	workDir = t.TempDir()
	defer func() { workDir = oldWorkDir }()
	path := filepath.Join(workDir, "test.xlsx")
	makeZip(t, path, minimalXlsxFiles)

	n := &OfficeNode{}
	_, err := n.Execute(context.Background(), "", map[string]string{"path": "test.xlsx", "sheet": "NoSheet"})
	if err == nil {
		t.Fatal("expected error for missing sheet")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

// minimalPptxFiles builds a deck with two slides, each with text runs.
var minimalPptxFiles = map[string]string{
	"ppt/slides/slide1.xml": `<?xml version="1.0"?>
<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:cSld><p:spTree>
    <p:sp><p:txBody><a:p><a:r><a:t>Slide One Title</a:t></a:r></a:p></p:txBody></p:sp>
  </p:spTree></p:cSld>
</p:sld>`,
	"ppt/slides/slide2.xml": `<?xml version="1.0"?>
<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:cSld><p:spTree>
    <p:sp><p:txBody><a:p><a:r><a:t>Slide Two</a:t></a:r></a:p></p:txBody></p:sp>
  </p:spTree></p:cSld>
</p:sld>`,
}

func TestOfficeNode_PptxMarkdown(t *testing.T) {
	oldWorkDir := workDir
	workDir = t.TempDir()
	defer func() { workDir = oldWorkDir }()
	path := filepath.Join(workDir, "test.pptx")
	makeZip(t, path, minimalPptxFiles)

	n := &OfficeNode{}
	out, err := n.Execute(context.Background(), "", map[string]string{"path": "test.pptx"})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !strings.Contains(out, "## Slide 1") || !strings.Contains(out, "Slide One Title") {
		t.Errorf("expected slide 1 content, got:\n%s", out)
	}
	if !strings.Contains(out, "## Slide 2") || !strings.Contains(out, "Slide Two") {
		t.Errorf("expected slide 2 content, got:\n%s", out)
	}
}

func TestOfficeNode_PptxJSON(t *testing.T) {
	oldWorkDir := workDir
	workDir = t.TempDir()
	defer func() { workDir = oldWorkDir }()
	path := filepath.Join(workDir, "test.pptx")
	makeZip(t, path, minimalPptxFiles)

	n := &OfficeNode{}
	out, err := n.Execute(context.Background(), "", map[string]string{"path": "test.pptx", "output": "json"})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !strings.Contains(out, `"slide":1`) || !strings.Contains(out, "Slide One Title") {
		t.Errorf("expected slide 1 in JSON, got:\n%s", out)
	}
}

func TestOfficeNode_InvalidFormat(t *testing.T) {
	oldWorkDir := workDir
	workDir = t.TempDir()
	defer func() { workDir = oldWorkDir }()
	path := filepath.Join(workDir, "test.txt")
	if err := os.WriteFile(path, []byte("plain text"), 0o644); err != nil {
		t.Fatal(err)
	}
	n := &OfficeNode{}
	_, err := n.Execute(context.Background(), "", map[string]string{"path": "test.txt"})
	if err == nil {
		t.Fatal("expected error for unsupported format")
	}
}

func TestOfficeNode_MissingPath(t *testing.T) {
	n := &OfficeNode{}
	_, err := n.Execute(context.Background(), "", map[string]string{})
	if err == nil {
		t.Fatal("expected error for missing path")
	}
}

func TestOfficeNode_PathFromInput(t *testing.T) {
	oldWorkDir := workDir
	workDir = t.TempDir()
	defer func() { workDir = oldWorkDir }()
	path := filepath.Join(workDir, "test.docx")
	makeZip(t, path, map[string]string{"word/document.xml": minimalDocxXML})

	n := &OfficeNode{}
	out, err := n.Execute(context.Background(), "test.docx", map[string]string{})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !strings.Contains(out, "Title") {
		t.Errorf("expected content from input path, got:\n%s", out)
	}
}

func TestOfficeNode_NotZip(t *testing.T) {
	oldWorkDir := workDir
	workDir = t.TempDir()
	defer func() { workDir = oldWorkDir }()
	path := filepath.Join(workDir, "fake.docx")
	if err := os.WriteFile(path, []byte("not a zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	n := &OfficeNode{}
	_, err := n.Execute(context.Background(), "", map[string]string{"path": "fake.docx"})
	if err == nil {
		t.Fatal("expected error for non-zip file")
	}
}

func TestOfficeNode_DocxMissingDocumentXML(t *testing.T) {
	oldWorkDir := workDir
	workDir = t.TempDir()
	defer func() { workDir = oldWorkDir }()
	path := filepath.Join(workDir, "bad.docx")
	makeZip(t, path, map[string]string{"other.xml": "<x/>"})

	n := &OfficeNode{}
	_, err := n.Execute(context.Background(), "", map[string]string{"path": "bad.docx"})
	if err == nil {
		t.Fatal("expected error for missing document.xml")
	}
}

func TestOfficeNode_InvalidOutput(t *testing.T) {
	oldWorkDir := workDir
	workDir = t.TempDir()
	defer func() { workDir = oldWorkDir }()
	path := filepath.Join(workDir, "test.docx")
	makeZip(t, path, map[string]string{"word/document.xml": minimalDocxXML})

	n := &OfficeNode{}
	_, err := n.Execute(context.Background(), "", map[string]string{"path": "test.docx", "output": "yaml"})
	if err == nil {
		t.Fatal("expected error for invalid output format")
	}
}
