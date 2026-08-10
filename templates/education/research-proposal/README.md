# Research Proposal Generator

Academic research proposal generator with automated literature search, methodology design, and refinement for dissertations, theses, and grants.

## Description

This workflow creates comprehensive research proposals by searching academic literature, drafting structured proposals with all required sections, and refining them for methodological rigor and academic standards.

## Usage Example

```yaml
params:
  research_topic: "Machine Learning Applications in Early Cancer Detection"
  field: "Computer Science / Biomedical Engineering"
  proposal_type: "dissertation"
  keywords: ["machine learning", "cancer detection", "medical imaging", "deep learning"]
  word_limit: 3000
  methodology_preference: "mixed"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| research_topic | string | yes | - | Research topic |
| field | string | yes | - | Academic discipline |
| proposal_type | string | no | dissertation | Proposal type |
| keywords | array | no | [] | Research keywords |
| word_limit | integer | no | 3000 | Target word count |
| methodology_preference | string | no | "" | Methodology preference |

## Nodes Used

- `http_request` - Search academic literature via OpenAlex
- `agent` - Draft and refine research proposal
- `file_write` - Save proposal to file

## Category

Education