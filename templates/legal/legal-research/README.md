# Legal Research & Case Law Search

## Description
An automated legal research workflow that searches case law databases and legal sources, analyzes results with AI, extracts key legal principles and precedents, and generates formal legal research memoranda with proper citations and structured analysis.

## Usage Example
```yaml
workflow: legal/legal-research
params:
  query: "Whether a website's terms of use constitute a binding contract under California law"
  jurisdiction: "US"
  sources: ["case-law", "statutes", "regulations"]
  date_range: {start: "2020", end: "2025"}
  max_results: 10
  output_path: "output/research_memo.md"
```

## Parameters
| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| query | string | Yes | - | Legal research question or topic |
| jurisdiction | string | No | US | Jurisdiction for research |
| sources | array | No | [case-law, statutes, regulations] | Types of legal sources to search |
| date_range | object | No | - | Date range filter (start and end years) |
| max_results | integer | No | 10 | Maximum results per source |
| model | string | No | gpt-4 | AI model to use |
| api_key | string | No | - | API key for legal databases |
| output_path | string | No | output/legal_research_memo.md | Output file path |

## Nodes Used
- **http_request** (search_case_law): Searches case law databases via API
- **agent** (analyze_results): Analyzes and synthesizes legal research results
- **agent** (identify_principles): Extracts key legal principles and precedents
- **agent** (generate_memo): Generates formal legal research memorandum
- **file_write** (save_memo): Saves the research memorandum

## Category
legal