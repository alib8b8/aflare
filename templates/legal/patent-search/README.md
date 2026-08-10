# Patent Search & Prior Art Analysis

## Description
A comprehensive patent search and prior art analysis workflow. Searches patent databases via API, analyzes prior art for relevance, assesses patentability (novelty, non-obviousness, utility under 35 USC 101-103), and evaluates freedom to operate (FTO) risks with design-around strategies.

## Usage Example
```yaml
workflow: legal/patent-search
params:
  invention_title: "AI-Powered Document Classification System"
  invention_description: "A machine learning system that automatically classifies legal documents using natural language processing and hierarchical neural networks."
  keywords: ["document classification", "neural network", "legal document", "NLP", "text classification"]
  patent_classes: ["G06N20/00", "G06F40/20"]
  assignee: null
  max_results: 20
  output_path: "output/patent_report.md"
```

## Parameters
| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| invention_title | string | Yes | - | Title of the invention |
| invention_description | string | Yes | - | Detailed description of the invention |
| keywords | array | Yes | - | Search keywords for patent search |
| patent_classes | array | No | [] | CPC/IPC patent classification codes |
| assignee | string | No | - | Assignee or company to filter by |
| max_results | integer | No | 20 | Maximum number of patent results |
| model | string | No | gpt-4 | AI model to use |
| output_path | string | No | output/patent_search_report.md | Output file path |

## Nodes Used
- **http_request** (search_patents): Searches patent databases via USPTO API
- **agent** (analyze_patents): Analyzes search results for prior art relevance
- **agent** (novelty_assessment): Assesses patentability and novelty
- **agent** (freedom_to_operate): Assesses freedom to operate risks
- **file_write** (save_report): Saves the patent search report

## Category
legal