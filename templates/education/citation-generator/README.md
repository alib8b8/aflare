# Citation Generator

Bibliography and citation formatter supporting multiple academic styles with DOI integration and automatic sorting.

## Description

This workflow formats citations and bibliographies in multiple academic styles (APA, MLA, Chicago, Harvard, IEEE, Vancouver). It validates sources, generates both in-text and bibliographic entries, and includes DOI links when available.

## Usage Example

```yaml
params:
  sources:
    - title: "Deep Learning"
      author: "Goodfellow, Ian"
      year: 2016
      publisher: "MIT Press"
      doi: "10.5555/1234567"
    - title: "Pattern Recognition and Machine Learning"
      author: "Bishop, Christopher"
      year: 2006
      publisher: "Springer"
  citation_style: "APA"
  output_format: "both"
  sort_by: "author"
  include_doi: true
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| sources | array | yes | - | Source metadata list |
| citation_style | string | no | APA | Citation style |
| output_format | string | no | bibliography | Output type |
| sort_by | string | no | author | Sort order |
| include_doi | boolean | no | true | Include DOI links |

## Nodes Used

- `http_request` - Validate sources against academic databases
- `agent` - Format citations in specified style
- `code_interpreter` - Sort and structure bibliography
- `file_write` - Save formatted bibliography to file

## Category

Education