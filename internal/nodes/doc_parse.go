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
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// maxDocParseSize bounds the size of downloaded / decoded / API response
// payloads handled by DocParseNode (10 MB, mirroring fetch_url).
const maxDocParseSize = 10 * 1024 * 1024

var (
	// markdownTableRowRe matches a single markdown table row such as
	// "| Name | Age |". It is matched against a trimmed line.
	markdownTableRowRe = regexp.MustCompile(`^\|.*\|$`)
	// markdownTableSeparatorRe matches a markdown table separator row
	// such as "|------|-----|" or "| :---: | ---: |".
	markdownTableSeparatorRe = regexp.MustCompile(`^\|[\s\-:|]+\|$`)
	// markdownFormulaRe matches LaTeX formulas wrapped in $$...$$ or
	// $...$. The $$...$$ form is matched first so that single-$ formulas
	// don't fragment double-dollar blocks.
	markdownFormulaRe = regexp.MustCompile(`\$\$[^$]+\$\$|\$[^$\n]+\$`)
)

// validDocParseSources is the set of supported source types for DocParseNode.
var validDocParseSources = map[string]bool{
	"text":   true,
	"base64": true,
	"URL":    true,
}

// validDocParseOutputFormats is the set of supported output formats for
// DocParseNode.
var validDocParseOutputFormats = map[string]bool{
	"text":       true,
	"latex":      true,
	"html_table": true,
}

// DocParseNode parses documents (PDF/images/HTML) into text, LaTeX, or
// HTML table format. Inspired by OvisOCR2's "single model handles text,
// LaTeX and HTML tables" approach, the node supports two parsing paths:
//
//   - External OCR API: when api_endpoint and api_key are configured, the
//     node POSTs the base64-encoded document to the OCR service (e.g.
//     OvisOCR2) and returns the parsed result.
//   - Built-in simple parser: for plain text input, the node performs
//     output-format conversion (text/latex/html_table) and optional
//     table/formula extraction without any external service.
type DocParseNode struct{}

func init() {
	Register(&DocParseNode{})
}

// Name returns the node name.
func (n *DocParseNode) Name() string { return "doc_parse" }

// Description returns a human-readable description of the node.
func (n *DocParseNode) Description() string {
	return "Parse documents (PDF/images/HTML) into text, LaTeX, or HTML table format"
}

// Schema returns the node's input/output/params schema.
func (n *DocParseNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        n.Name(),
		Description: n.Description(),
		Input:       "string - document text (source=text), base64-encoded image/PDF (source=base64), or URL (source=URL)",
		Output:      "string - parsed document content in the requested output format",
		Params: []ParamSchema{
			{Name: "source", Type: "string", Description: "Input source type: text|base64|URL (default: text)", Required: false, Default: "text"},
			{Name: "output_format", Type: "string", Description: "Output format: text|latex|html_table (default: text)", Required: false, Default: "text"},
			{Name: "extract_tables", Type: "bool", Description: "Extract markdown tables and return their count/content (default: false)", Required: false, Default: "false"},
			{Name: "extract_formulas", Type: "bool", Description: "Extract LaTeX formulas ($...$, $$...$$) and return their list (default: false)", Required: false, Default: "false"},
			{Name: "lang", Type: "string", Description: "Document language hint: zh|en|auto (default: auto, passed to OCR API)", Required: false, Default: "auto"},
			{Name: "api_endpoint", Type: "string", Description: "OCR API endpoint URL (optional). When set with api_key, calls external OCR (e.g. OvisOCR2)", Required: false},
			{Name: "api_key", Type: "string", Description: "OCR API key (optional)", Required: false},
		},
	}
}

// Execute parses the input document according to the source and
// output_format parameters. For source=text the built-in parser is used;
// for source=base64 or source=URL an external OCR API must be configured.
func (n *DocParseNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	source := getParam(params, "source", "text")
	if !validDocParseSources[source] {
		return "", fmt.Errorf("invalid source: %s (supported: text, base64, URL)", source)
	}

	outputFormat := getParam(params, "output_format", "text")
	if !validDocParseOutputFormats[outputFormat] {
		return "", fmt.Errorf("invalid output_format: %s (supported: text, latex, html_table)", outputFormat)
	}

	extractTables := strings.ToLower(getParam(params, "extract_tables", "false")) == "true"
	extractFormulas := strings.ToLower(getParam(params, "extract_formulas", "false")) == "true"
	lang := getParam(params, "lang", "auto")
	apiEndpoint := getParam(params, "api_endpoint", "")
	apiKey := getParam(params, "api_key", "")

	switch source {
	case "text":
		if strings.TrimSpace(input) == "" {
			return "", fmt.Errorf("input is required for source=text")
		}
		return n.parseText(input, outputFormat, extractTables, extractFormulas)
	case "base64":
		imageBase64, err := decodeBase64Input(input)
		if err != nil {
			return "", err
		}
		if apiEndpoint == "" || apiKey == "" {
			return "", fmt.Errorf("built-in mode does not support image/PDF parsing; please configure api_endpoint and api_key")
		}
		return n.callOCRAPI(ctx, apiEndpoint, apiKey, imageBase64, outputFormat, lang, extractTables, extractFormulas)
	case "URL":
		rawURL := getParam(params, "url", "")
		if rawURL == "" {
			rawURL = input
		}
		if rawURL == "" {
			return "", fmt.Errorf("url parameter (or input) is required for source=URL")
		}
		imageBase64, err := downloadAsBase64(ctx, rawURL)
		if err != nil {
			return "", err
		}
		if apiEndpoint == "" || apiKey == "" {
			return "", fmt.Errorf("built-in mode does not support image/PDF parsing; please configure api_endpoint and api_key")
		}
		return n.callOCRAPI(ctx, apiEndpoint, apiKey, imageBase64, outputFormat, lang, extractTables, extractFormulas)
	default:
		return "", fmt.Errorf("unsupported source: %s", source)
	}
}

// parseText runs the built-in text parsing path: applies output-format
// conversion (text/latex/html_table) and optional table/formula extraction.
func (n *DocParseNode) parseText(input, outputFormat string, extractTables, extractFormulas bool) (string, error) {
	if extractTables || extractFormulas {
		return n.buildExtractionReport(input, outputFormat, extractTables, extractFormulas), nil
	}

	switch outputFormat {
	case "text":
		return input, nil
	case "latex":
		return convertMarkdownTables(input, markdownTableToLatex), nil
	case "html_table":
		return convertMarkdownTables(input, markdownTableToHTML), nil
	default:
		return input, nil
	}
}

// buildExtractionReport produces a structured report listing tables and/or
// formulas found in the input text. Tables are converted to the requested
// output format when it is latex or html_table.
func (n *DocParseNode) buildExtractionReport(input, outputFormat string, extractTables, extractFormulas bool) string {
	var sb strings.Builder
	sb.WriteString("## Doc Parse Extraction Report\n\n")

	if extractTables {
		tables := extractMarkdownTables(input)
		sb.WriteString(fmt.Sprintf("### Tables (%d found)\n\n", len(tables)))
		for i, t := range tables {
			sb.WriteString(fmt.Sprintf("#### Table %d\n\n", i+1))
			switch outputFormat {
			case "latex":
				sb.WriteString(markdownTableToLatex(t))
			case "html_table":
				sb.WriteString(markdownTableToHTML(t))
			default:
				sb.WriteString(t)
			}
			sb.WriteString("\n\n")
		}
	}

	if extractFormulas {
		formulas := extractLatexFormulas(input)
		sb.WriteString(fmt.Sprintf("### Formulas (%d found)\n\n", len(formulas)))
		for i, f := range formulas {
			sb.WriteString(fmt.Sprintf("%d. `%s`\n", i+1, f))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// callOCRAPI POSTs image_base64 to the OCR API endpoint and returns the
// parsed result returned by the API. The endpoint URL is validated for
// SSRF protection and the response is limited to maxDocParseSize.
func (n *DocParseNode) callOCRAPI(ctx context.Context, endpoint, apiKey, imageBase64, outputFormat, lang string, extractTables, extractFormulas bool) (string, error) {
	if err := validateURL(endpoint); err != nil {
		return "", fmt.Errorf("api_endpoint validation failed: %w", err)
	}

	requestBody := struct {
		ImageBase64     string `json:"image_base64"`
		OutputFormat    string `json:"output_format"`
		Lang            string `json:"lang"`
		ExtractTables   bool   `json:"extract_tables"`
		ExtractFormulas bool   `json:"extract_formulas"`
	}{
		ImageBase64:     imageBase64,
		OutputFormat:    outputFormat,
		Lang:            lang,
		ExtractTables:   extractTables,
		ExtractFormulas: extractFormulas,
	}

	bodyJSON, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal OCR request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyJSON))
	if err != nil {
		return "", fmt.Errorf("failed to create OCR request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{
		Timeout:       60 * time.Second,
		Transport:     safeHTTPClient.Transport,
		CheckRedirect: httpRedirectValidator(validateURL),
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("OCR API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("OCR API returned status %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxDocParseSize))
	if err != nil {
		return "", fmt.Errorf("failed to read OCR response: %w", err)
	}

	// Try to parse as JSON {"result": "..."} or {"text": "..."}. If that
	// fails or neither field is present, return the raw body as-is.
	var parsed struct {
		Result string `json:"result"`
		Text   string `json:"text"`
	}
	if json.Unmarshal(respBody, &parsed) == nil {
		if parsed.Result != "" {
			return parsed.Result, nil
		}
		if parsed.Text != "" {
			return parsed.Text, nil
		}
	}
	return string(respBody), nil
}

// decodeBase64Input validates that input is base64-decodable and that the
// decoded payload stays within maxDocParseSize. It returns the original
// base64 string (which is what the OCR API expects to receive).
func decodeBase64Input(input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", fmt.Errorf("input is required for source=base64")
	}
	decoded, err := base64.StdEncoding.DecodeString(input)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64 input: %w", err)
	}
	if len(decoded) > maxDocParseSize {
		return "", fmt.Errorf("decoded base64 content exceeds size limit (%d bytes)", maxDocParseSize)
	}
	return input, nil
}

// downloadAsBase64 downloads the content at rawURL and returns it as a
// base64-encoded string. The URL is validated (SSRF protection) and the
// response is limited to maxDocParseSize. Redirects are re-validated.
func downloadAsBase64(ctx context.Context, rawURL string) (string, error) {
	if err := validateURL(rawURL); err != nil {
		return "", fmt.Errorf("URL validation failed: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create download request: %w", err)
	}
	req.Header.Set("User-Agent", "llm-box/1.0")

	client := &http.Client{
		Timeout:       30 * time.Second,
		Transport:     safeHTTPClient.Transport,
		CheckRedirect: httpRedirectValidator(validateURL),
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxDocParseSize))
	if err != nil {
		return "", fmt.Errorf("failed to read download response: %w", err)
	}

	return base64.StdEncoding.EncodeToString(body), nil
}

// convertMarkdownTables replaces every markdown table block in text with
// the output of converter(table). Non-table lines are returned unchanged.
// A run of fewer than two consecutive table rows is treated as plain text.
func convertMarkdownTables(text string, converter func(string) string) string {
	lines := strings.Split(text, "\n")
	var result []string
	var currentTable []string

	flush := func() {
		if len(currentTable) == 0 {
			return
		}
		if len(currentTable) >= 2 {
			result = append(result, converter(strings.Join(currentTable, "\n")))
		} else {
			result = append(result, currentTable...)
		}
		currentTable = nil
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if markdownTableRowRe.MatchString(trimmed) {
			currentTable = append(currentTable, trimmed)
		} else {
			flush()
			result = append(result, line)
		}
	}
	flush()
	return strings.Join(result, "\n")
}

// extractMarkdownTables finds all markdown table blocks in text. A table
// block is a run of two or more consecutive lines matching ^\|.*\|$. The
// separator row (|---|---|) is kept as part of the block. Returns the
// raw text of each table block.
func extractMarkdownTables(text string) []string {
	lines := strings.Split(text, "\n")
	var tables []string
	var currentTable []string

	flush := func() {
		if len(currentTable) >= 2 {
			tables = append(tables, strings.Join(currentTable, "\n"))
		}
		currentTable = nil
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if markdownTableRowRe.MatchString(trimmed) {
			currentTable = append(currentTable, trimmed)
		} else {
			flush()
		}
	}
	flush()
	return tables
}

// markdownTableToLatex converts a markdown table to a LaTeX tabular
// environment. Separator rows (|---|---|) are skipped. Each cell is
// emitted as a left-aligned column entry separated by " & " and
// terminated with " \\", with \hline before every row.
//
// Input:
//
//	| Name | Age |
//	|------|-----|
//	| Alice | 30 |
//
// Output:
//
//	\begin{tabular}{|l|l|}
//	\hline
//	Name & Age \\
//	\hline
//	Alice & 30 \\
//	\hline
//	\end{tabular}
func markdownTableToLatex(table string) string {
	lines := strings.Split(table, "\n")
	var dataRows [][]string
	colCount := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !markdownTableRowRe.MatchString(trimmed) {
			continue
		}
		if markdownTableSeparatorRe.MatchString(trimmed) {
			continue
		}
		cells := splitMarkdownTableRow(trimmed)
		if colCount == 0 {
			colCount = len(cells)
		}
		dataRows = append(dataRows, cells)
	}
	if len(dataRows) == 0 {
		return ""
	}
	if colCount == 0 {
		colCount = len(dataRows[0])
	}

	var sb strings.Builder
	sb.WriteString("\\begin{tabular}{")
	for i := 0; i < colCount; i++ {
		sb.WriteString("|l")
	}
	sb.WriteString("|}\n")
	sb.WriteString("\\hline\n")
	for _, row := range dataRows {
		// Pad short rows so every row has colCount cells.
		for len(row) < colCount {
			row = append(row, "")
		}
		cleaned := make([]string, colCount)
		for i := 0; i < colCount; i++ {
			cleaned[i] = strings.TrimSpace(row[i])
		}
		sb.WriteString(strings.Join(cleaned, " & "))
		sb.WriteString(" \\\\\n")
		sb.WriteString("\\hline\n")
	}
	sb.WriteString("\\end{tabular}")
	return sb.String()
}

// markdownTableToHTML converts a markdown table to an HTML <table>. The
// first data row is treated as the header (<th>); subsequent rows are
// body cells (<td>). Separator rows (|---|---|) are skipped. Cell
// content is HTML-escaped.
func markdownTableToHTML(table string) string {
	lines := strings.Split(table, "\n")
	var dataRows [][]string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !markdownTableRowRe.MatchString(trimmed) {
			continue
		}
		if markdownTableSeparatorRe.MatchString(trimmed) {
			continue
		}
		dataRows = append(dataRows, splitMarkdownTableRow(trimmed))
	}
	if len(dataRows) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("<table>\n")
	for i, row := range dataRows {
		sb.WriteString("<tr>")
		tag := "td"
		if i == 0 {
			tag = "th"
		}
		for _, cell := range row {
			sb.WriteString("<")
			sb.WriteString(tag)
			sb.WriteString(">")
			sb.WriteString(escapeHTML(strings.TrimSpace(cell)))
			sb.WriteString("</")
			sb.WriteString(tag)
			sb.WriteString(">")
		}
		sb.WriteString("</tr>\n")
	}
	sb.WriteString("</table>")
	return sb.String()
}

// splitMarkdownTableRow splits a markdown table row such as "| a | b |"
// into ["a", "b"] by trimming the leading/trailing pipes and splitting
// on the remaining pipes. Each cell is space-trimmed.
func splitMarkdownTableRow(row string) []string {
	s := strings.TrimSpace(row)
	s = strings.TrimPrefix(s, "|")
	s = strings.TrimSuffix(s, "|")
	parts := strings.Split(s, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

// escapeHTML escapes the basic HTML-significant characters in s.
func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}

// extractLatexFormulas finds all LaTeX formulas wrapped in $$...$$ or
// $...$ in text. The $$...$$ form is matched first so that single-$
// formulas don't fragment double-dollar blocks. Returns the matched
// formula strings (including the surrounding $ or $$ delimiters).
func extractLatexFormulas(text string) []string {
	matches := markdownFormulaRe.FindAllString(text, -1)
	if len(matches) == 0 {
		return nil
	}
	return matches
}
