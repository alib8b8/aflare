// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​​​​​‌​​‌​​​‌​​​​‌​​​‌​​‌​​‌​‌​​‌‌‌​​​​‌​‌​‌‌​‌​​​​​​​​​​​​​​​​​​‌‌‌‌‌‌​​​‌​​​‌​⁠
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
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/alib8b8/aflare/internal/nodes/core"
)

// maxOfficeFileSize bounds in-memory OOXML processing (50 MB, enough for
// large spreadsheets while preventing unbounded memory use).
const maxOfficeFileSize = 50 * 1024 * 1024

// validOfficeFormats is the set of supported OOXML container formats.
var validOfficeFormats = map[string]bool{
	"docx": true,
	"xlsx": true,
	"pptx": true,
}

// validOfficeOutputs is the set of supported output formats.
var validOfficeOutputs = map[string]bool{
	"text":     true,
	"markdown": true,
	"json":     true,
}

// OfficeNode reads Microsoft Office (OOXML) documents — .docx, .xlsx,
// .pptx — without external dependencies. OOXML files are ZIP containers
// of XML parts, so the standard library (archive/zip + encoding/xml) is
// sufficient to extract text, tables and slide content.
//
// Inspired by iOfficeAI/OfficeCLI's "AI Agent 可读写 Word/Excel/PPT" goal,
// this node gives workflows the ability to ingest office documents for
// summarization, search, or further LLM processing. Write support is out
// of scope for this iteration; the node is read-only.
type OfficeNode struct{}

func init() {
	Register(&OfficeNode{})
}

// Name returns the node name.
func (n *OfficeNode) Name() string { return "office" }

// Description returns a human-readable description.
func (n *OfficeNode) Description() string {
	return "Read .docx/.xlsx/.pptx documents (text, tables, slides) using pure-Go OOXML parsing"
}

// Schema returns the node's input/output/params schema.
func (n *OfficeNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        n.Name(),
		Description: n.Description(),
		Input:       "string - path to the office document (.docx/.xlsx/.pptx)",
		Output:      "string - extracted content in the requested output format",
		Params: []ParamSchema{
			{Name: "path", Type: "string", Description: "Path to the office document (overrides input if set)", Required: false},
			{Name: "format", Type: "string", Description: "Source format hint: docx|xlsx|pptx (default: inferred from extension)", Required: false},
			{Name: "output", Type: "string", Description: "Output format: text|markdown|json (default: markdown)", Required: false, Default: "markdown"},
			{Name: "sheet", Type: "string", Description: "xlsx only: sheet name to read (default: all sheets)", Required: false},
			{Name: "max_rows", Type: "int", Description: "xlsx only: max rows per sheet (default: 1000, 0 = unlimited)", Required: false, Default: "1000"},
		},
	}
}

// Execute reads the office document and returns extracted content.
func (n *OfficeNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	path := core.GetParam(params, "path", strings.TrimSpace(input))
	if path == "" {
		return "", fmt.Errorf("path is required (via 'path' param or input)")
	}

	safePath, err := validateReadPath(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	format := core.GetParam(params, "format", "")
	if format == "" {
		format = strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	}
	if !validOfficeFormats[format] {
		return "", fmt.Errorf("unsupported format: %q (supported: docx, xlsx, pptx)", format)
	}

	output := core.GetParam(params, "output", "markdown")
	if !validOfficeOutputs[output] {
		return "", fmt.Errorf("invalid output: %q (supported: text, markdown, json)", output)
	}

	// Open and size-check the zip container.
	zr, err := zip.OpenReader(safePath) // #nosec G304 -- path validated by validateReadPath
	if err != nil {
		return "", fmt.Errorf("failed to open office file (is it a valid %s?): %w", format, err)
	}
	defer zr.Close()

	// Guard total uncompressed size to avoid zip bombs.
	var totalSize uint64
	for _, f := range zr.File {
		totalSize += f.UncompressedSize64
	}
	if totalSize > maxOfficeFileSize {
		return "", fmt.Errorf("uncompressed size %d exceeds limit %d", totalSize, maxOfficeFileSize)
	}

	switch format {
	case "docx":
		return parseDocx(&zr.Reader, output)
	case "xlsx":
		sheet := core.GetParam(params, "sheet", "")
		return parseXlsx(&zr.Reader, output, sheet)
	case "pptx":
		return parsePptx(&zr.Reader, output)
	default:
		return "", fmt.Errorf("unsupported format: %s", format)
	}
}

// readZipFile reads a single file from the zip archive into memory.
func readZipFile(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

// findZipFile returns the first file matching name (exact match).
func findZipFile(r *zip.Reader, name string) *zip.File {
	for _, f := range r.File {
		if f.Name == name {
			return f
		}
	}
	return nil
}

// findZipFiles returns all files whose name starts with prefix, sorted by
// name for deterministic ordering.
func findZipFiles(r *zip.Reader, prefix string) []*zip.File {
	var out []*zip.File
	for _, f := range r.File {
		if strings.HasPrefix(f.Name, prefix) {
			out = append(out, f)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// ---- OOXML XML element types ----
//
// Only the elements we care about are modeled; unknown elements are
// skipped by the decoder. XML namespaces are matched by local name to
// tolerate namespace-prefix variations across producers.

// xName returns the local name of an XML start element (namespace prefix
// stripped). e.g. "w:t" -> "t", "t" -> "t".
func xName(e xml.StartElement) string {
	return localName(e.Name.Local)
}

// xEndName returns the local name of an XML end element.
func xEndName(e xml.EndElement) string {
	return localName(e.Name.Local)
}

// localName strips the namespace prefix from an XML name.
func localName(name string) string {
	if i := strings.IndexByte(name, ':'); i >= 0 {
		return name[i+1:]
	}
	return name
}

// collectText walks an XML stream and concatenates all text inside <...:t>
// elements (the OOXML convention for visible text runs: w:t in docx,
// a:t in pptx, and inline strings in xlsx are read elsewhere).
func collectText(dec *xml.Decoder) (string, error) {
	var sb strings.Builder
	depth := 0
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if xName(t) == "t" {
				// DecodeElement consumes the whole <t>..</t> including its
				// EndElement, so depth must not be bumped here (no matching
				// EndElement will be returned by Token to balance it).
				var s string
				if err := dec.DecodeElement(&s, &t); err != nil {
					return "", err
				}
				sb.WriteString(s)
			} else {
				depth++
			}
		case xml.EndElement:
			depth--
			if depth < 0 {
				return sb.String(), nil // back to caller's level
			}
		}
	}
	return sb.String(), nil
}

// ---- DOCX ----

// parseDocx extracts paragraphs and tables from word/document.xml.
func parseDocx(r *zip.Reader, output string) (string, error) {
	f := findZipFile(r, "word/document.xml")
	if f == nil {
		return "", fmt.Errorf("not a valid docx: missing word/document.xml")
	}
	data, err := readZipFile(f)
	if err != nil {
		return "", fmt.Errorf("read document.xml: %w", err)
	}

	blocks, err := parseDocxBlocks(data)
	if err != nil {
		return "", err
	}
	return renderDocx(blocks, output), nil
}

// docxBlock is a paragraph or a table extracted from a docx.
type docxBlock struct {
	Kind    string     // "para" | "table"
	Text    string     // for para
	Rows    [][]string // for table (first row = header)
	Heading int        // 0=body, 1-9=heading level
}

// parseDocxBlocks walks document.xml and produces ordered blocks.
func parseDocxBlocks(data []byte) ([]docxBlock, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	var blocks []docxBlock
	depth := 0
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		local := xName(se)
		// Only inspect top-level body children to keep structure simple.
		if local == "p" && depth == 0 {
			depth++
			blk, err := parseDocxParagraph(dec)
			depth--
			if err != nil {
				return nil, err
			}
			if blk != nil {
				blocks = append(blocks, *blk)
			}
			continue
		}
		if local == "tbl" && depth == 0 {
			depth++
			rows, err := parseDocxTable(dec)
			depth--
			if err != nil {
				return nil, err
			}
			if len(rows) > 0 {
				blocks = append(blocks, docxBlock{Kind: "table", Rows: rows})
			}
		}
	}
	return blocks, nil
}

// parseDocxParagraph reads a single <w:p> and returns its text + heading
// level (derived from <w:pStyle w:val="Heading1"/> etc.). Returns nil for
// empty paragraphs.
func parseDocxParagraph(dec *xml.Decoder) (*docxBlock, error) {
	var sb strings.Builder
	heading := 0
	depth := 1 // we're inside <w:p>
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			local := xName(t)
			if local == "t" {
				depth++
				var s string
				if err := dec.DecodeElement(&s, &t); err != nil {
					return nil, err
				}
				sb.WriteString(s)
				depth--
				continue
			}
			if local == "tab" {
				sb.WriteString("\t")
			}
			if local == "br" {
				sb.WriteString("\n")
			}
			if local == "pStyle" {
				depth++
				var ps struct {
					Val string `xml:"val,attr"`
				}
				if err := dec.DecodeElement(&ps, &t); err != nil {
					return nil, err
				}
				heading = parseHeadingLevel(ps.Val)
				depth--
				continue // DecodeElement consumed the whole element; skip the trailing depth++
			}
			depth++
		case xml.EndElement:
			depth--
			if depth == 0 {
				// end of <w:p>
				text := sb.String()
				if strings.TrimSpace(text) == "" {
					return nil, nil
				}
				return &docxBlock{Kind: "para", Text: text, Heading: heading}, nil
			}
		}
	}
	return &docxBlock{Kind: "para", Text: sb.String(), Heading: heading}, nil
}

// parseHeadingLevel maps a pStyle val like "Heading1" / "Heading 2" to
// its numeric level. Returns 0 for non-heading styles.
func parseHeadingLevel(val string) int {
	val = strings.ToLower(strings.TrimSpace(val))
	if !strings.HasPrefix(val, "heading") {
		return 0
	}
	rest := strings.TrimSpace(strings.TrimPrefix(val, "heading"))
	var n int
	_, _ = fmt.Sscanf(rest, "%d", &n)
	if n < 1 || n > 9 {
		return 0
	}
	return n
}

// parseDocxTable reads a <w:tbl> and returns its rows (each row is a
// slice of cell texts).
func parseDocxTable(dec *xml.Decoder) ([][]string, error) {
	var rows [][]string
	depth := 1
	var curRow []string
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			local := xName(t)
			if local == "tr" {
				curRow = nil
				depth++
				continue
			}
			if local == "tc" {
				depth++
				txt, err := collectText(dec)
				if err != nil {
					return nil, err
				}
				curRow = append(curRow, strings.TrimSpace(txt))
				depth--
				continue
			}
			depth++
		case xml.EndElement:
			depth--
			if xEndName(t) == "tr" {
				if len(curRow) > 0 {
					rows = append(rows, curRow)
				}
				continue
			}
			if depth == 0 {
				return rows, nil
			}
		}
	}
	return rows, nil
}

// renderDocx renders blocks in the requested output format.
func renderDocx(blocks []docxBlock, output string) string {
	switch output {
	case "text":
		var sb strings.Builder
		for _, b := range blocks {
			if b.Kind == "para" {
				sb.WriteString(b.Text)
				sb.WriteString("\n\n")
				continue
			}
			for _, row := range b.Rows {
				sb.WriteString(strings.Join(row, "\t"))
				sb.WriteString("\n")
			}
			sb.WriteString("\n")
		}
		return sb.String()
	case "json":
		// Reuse markdown renderer then wrap; JSON of mixed blocks is
		// structured below.
		var sb strings.Builder
		sb.WriteString("{\"blocks\":[")
		for i, b := range blocks {
			if i > 0 {
				sb.WriteString(",")
			}
			if b.Kind == "para" {
				sb.WriteString(fmt.Sprintf(`{"kind":"para","text":%q}`, b.Text))
				continue
			}
			sb.WriteString(`{"kind":"table","rows":[`)
			for j, row := range b.Rows {
				if j > 0 {
					sb.WriteString(",")
				}
				sb.WriteString("[")
				for k, cell := range row {
					if k > 0 {
						sb.WriteString(",")
					}
					sb.WriteString(fmt.Sprintf("%q", cell))
				}
				sb.WriteString("]")
			}
			sb.WriteString("]}")
		}
		sb.WriteString("]}")
		return sb.String()
	default: // markdown
		var sb strings.Builder
		for _, b := range blocks {
			if b.Kind == "para" {
				if b.Heading > 0 {
					sb.WriteString(strings.Repeat("#", b.Heading))
					sb.WriteString(" ")
				}
				sb.WriteString(b.Text)
				sb.WriteString("\n\n")
				continue
			}
			renderMarkdownTable(&sb, b.Rows)
			sb.WriteString("\n")
		}
		return sb.String()
	}
}

// renderMarkdownTable writes rows as a GitHub-flavored markdown table.
// The first row is treated as the header.
func renderMarkdownTable(sb *strings.Builder, rows [][]string) {
	if len(rows) == 0 {
		return
	}
	header := rows[0]
	maxCols := len(header)
	for _, r := range rows[1:] {
		if len(r) > maxCols {
			maxCols = len(r)
		}
	}
	padRow := func(r []string) []string {
		out := make([]string, maxCols)
		for i := range out {
			if i < len(r) {
				out[i] = strings.ReplaceAll(r[i], "|", "\\|")
			}
		}
		return out
	}
	h := padRow(header)
	sb.WriteString("| ")
	sb.WriteString(strings.Join(h, " | "))
	sb.WriteString(" |\n")
	sb.WriteString("|")
	for i := 0; i < maxCols; i++ {
		sb.WriteString(" --- |")
	}
	sb.WriteString("\n")
	for _, r := range rows[1:] {
		pr := padRow(r)
		sb.WriteString("| ")
		sb.WriteString(strings.Join(pr, " | "))
		sb.WriteString(" |\n")
	}
}

// ---- XLSX ----

// parseXlsx extracts cell values from all sheets (or a named sheet).
// Shared strings are resolved; formula results and inline strings are
// read; numbers and booleans are returned as their stored text.
func parseXlsx(r *zip.Reader, output, sheetName string) (string, error) {
	shared, err := loadSharedStrings(r)
	if err != nil {
		return "", err
	}

	sheets, err := loadXlsxSheets(r, sheetName, shared)
	if err != nil {
		return "", err
	}
	if len(sheets) == 0 {
		if sheetName != "" {
			return "", fmt.Errorf("sheet %q not found", sheetName)
		}
		return "", fmt.Errorf("no sheets found")
	}

	var sb strings.Builder
	switch output {
	case "json":
		sb.WriteString("{\"sheets\":[")
		for i, sh := range sheets {
			if i > 0 {
				sb.WriteString(",")
			}
			sb.WriteString(fmt.Sprintf(`{"name":%q,"rows":[`, sh.name))
			for j, row := range sh.rows {
				if j > 0 {
					sb.WriteString(",")
				}
				sb.WriteString("[")
				for k, cell := range row {
					if k > 0 {
						sb.WriteString(",")
					}
					sb.WriteString(fmt.Sprintf("%q", cell))
				}
				sb.WriteString("]")
			}
			sb.WriteString("]}")
		}
		sb.WriteString("]}")
	default: // text or markdown
		for i, sh := range sheets {
			if i > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(fmt.Sprintf("## %s\n\n", sh.name))
			if output == "markdown" {
				renderMarkdownTable(&sb, sh.rows)
			} else {
				for _, row := range sh.rows {
					sb.WriteString(strings.Join(row, "\t"))
					sb.WriteString("\n")
				}
			}
		}
	}
	return sb.String(), nil
}

type xlsxSheet struct {
	name string
	rows [][]string
}

// loadSharedStrings reads xl/sharedStrings.xml and returns the string
// table. Returns an empty table if the file is absent (workbook with no
// shared strings).
func loadSharedStrings(r *zip.Reader) ([]string, error) {
	f := findZipFile(r, "xl/sharedStrings.xml")
	if f == nil {
		return nil, nil
	}
	data, err := readZipFile(f)
	if err != nil {
		return nil, fmt.Errorf("read sharedStrings.xml: %w", err)
	}
	dec := xml.NewDecoder(bytes.NewReader(data))
	var items []string
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if xName(se) == "si" {
			txt, err := collectText(dec)
			if err != nil {
				return nil, err
			}
			items = append(items, txt)
		}
	}
	return items, nil
}

// loadXlsxSheets parses xl/workbook.xml for sheet names and the matching
// xl/worksheets/sheetN.xml files. If sheetName is non-empty, only that
// sheet is returned.
func loadXlsxSheets(r *zip.Reader, sheetName string, shared []string) ([]xlsxSheet, error) {
	wbFile := findZipFile(r, "xl/workbook.xml")
	if wbFile == nil {
		return nil, fmt.Errorf("not a valid xlsx: missing xl/workbook.xml")
	}
	wbData, err := readZipFile(wbFile)
	if err != nil {
		return nil, fmt.Errorf("read workbook.xml: %w", err)
	}

	names, err := parseWorkbookSheetNames(wbData)
	if err != nil {
		return nil, err
	}
	// Sheet files are xl/worksheets/sheet1.xml .. sheetN.xml in order.
	var sheets []xlsxSheet
	for i, name := range names {
		if sheetName != "" && !strings.EqualFold(name, sheetName) {
			continue
		}
		path := fmt.Sprintf("xl/worksheets/sheet%d.xml", i+1)
		f := findZipFile(r, path)
		if f == nil {
			continue
		}
		data, err := readZipFile(f)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		rows, err := parseXlsxSheetRows(data, shared)
		if err != nil {
			return nil, err
		}
		sheets = append(sheets, xlsxSheet{name: name, rows: rows})
	}
	return sheets, nil
}

// parseWorkbookSheetNames extracts <sheet name="..."/> entries in order.
func parseWorkbookSheetNames(data []byte) ([]string, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	var names []string
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if xName(se) == "sheet" {
			for _, a := range se.Attr {
				if a.Name.Local == "name" {
					names = append(names, a.Value)
				}
			}
		}
	}
	return names, nil
}

// parseXlsxSheetRows reads a worksheet XML and returns rows of cell
// values. Shared-string cells (t="s") are resolved against the shared
// table; inline strings (t="inlineStr") are read inline; numeric cells
// return their <v>.
func parseXlsxSheetRows(data []byte, shared []string) ([][]string, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	var rows [][]string
	var curRow []string
	depth := 0
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			local := xName(t)
			if local == "row" {
				curRow = nil
				depth++
				continue
			}
			if local == "c" {
				cellType := ""
				for _, a := range t.Attr {
					if a.Name.Local == "t" {
						cellType = a.Value
					}
				}
				depth++
				val, err := parseXlsxCell(dec, cellType, shared)
				if err != nil {
					return nil, err
				}
				curRow = append(curRow, val)
				depth--
				continue
			}
			depth++
		case xml.EndElement:
			depth--
			if xEndName(t) == "row" {
				if len(curRow) > 0 {
					rows = append(rows, curRow)
				}
				continue
			}
		}
	}
	return rows, nil
}

// parseXlsxCell reads a single <c> element and returns its value. For
// shared-string cells (t="s") the index in <v> is resolved via shared.
func parseXlsxCell(dec *xml.Decoder, cellType string, shared []string) (string, error) {
	var value strings.Builder
	depth := 1
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			local := xName(t)
			if local == "v" {
				depth++
				var s string
				if err := dec.DecodeElement(&s, &t); err != nil {
					return "", err
				}
				if cellType == "s" {
					// shared string index
					var idx int
					_, _ = fmt.Sscanf(s, "%d", &idx)
					if idx >= 0 && idx < len(shared) {
						return shared[idx], nil
					}
					return "", nil
				}
				value.WriteString(s)
				depth--
				continue
			}
			if local == "t" {
				depth++
				var s string
				if err := dec.DecodeElement(&s, &t); err != nil {
					return "", err
				}
				value.WriteString(s)
				depth--
				continue
			}
			depth++
		case xml.EndElement:
			depth--
			if depth == 0 {
				return value.String(), nil
			}
		}
	}
	return value.String(), nil
}

// ---- PPTX ----

// parsePptx extracts text from all slides. Each slide's text runs are
// concatenated; slides are separated by a heading.
func parsePptx(r *zip.Reader, output string) (string, error) {
	slideFiles := findZipFiles(r, "ppt/slides/slide")
	if len(slideFiles) == 0 {
		return "", fmt.Errorf("not a valid pptx: no slides found")
	}
	var sb strings.Builder
	switch output {
	case "json":
		sb.WriteString("{\"slides\":[")
	default:
	}
	for i, sf := range slideFiles {
		data, err := readZipFile(sf)
		if err != nil {
			return "", fmt.Errorf("read %s: %w", sf.Name, err)
		}
		text, err := parsePptxSlide(data)
		if err != nil {
			return "", err
		}
		switch output {
		case "json":
			if i > 0 {
				sb.WriteString(",")
			}
			sb.WriteString(fmt.Sprintf(`{"slide":%d,"text":%q}`, i+1, text))
		case "text":
			sb.WriteString(fmt.Sprintf("=== Slide %d ===\n%s\n\n", i+1, text))
		default: // markdown
			sb.WriteString(fmt.Sprintf("## Slide %d\n\n%s\n\n", i+1, text))
		}
	}
	if output == "json" {
		sb.WriteString("]}")
	}
	return sb.String(), nil
}

// parsePptxSlide reads a slideN.xml and returns all <a:t> text
// concatenated with newlines between runs.
func parsePptxSlide(data []byte) (string, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	var lines []string
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", err
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if xName(se) == "t" {
			var s string
			if err := dec.DecodeElement(&s, &se); err != nil {
				return "", err
			}
			if s = strings.TrimSpace(s); s != "" {
				lines = append(lines, s)
			}
		}
	}
	return strings.Join(lines, "\n"), nil
}
