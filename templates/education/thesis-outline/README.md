# Thesis Outline Builder

Thesis and dissertation outline builder with chapter structure, content planning, writing timeline, and citation style integration.

## Description

This workflow creates detailed thesis outlines with chapter-by-chapter structure, content descriptions, estimated page counts, and a realistic writing timeline based on degree level. It supports all degree levels from undergraduate to doctoral.

## Usage Example

```yaml
params:
  thesis_title: "The Impact of Social Media on Adolescent Mental Health"
  field: "Psychology"
  degree_level: "masters"
  chapters: 5
  research_questions: ["How does social media usage correlate with anxiety?", "What protective factors moderate negative effects?"]
  include_timeline: true
  citation_style: "APA"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| thesis_title | string | yes | - | Working title |
| field | string | yes | - | Academic field |
| degree_level | string | no | masters | Degree level |
| chapters | integer | no | 5 | Number of chapters |
| research_questions | array | yes | - | Research questions |
| include_timeline | boolean | no | true | Include writing timeline |
| citation_style | string | no | APA | Citation style |

## Nodes Used

- `agent` - Design detailed thesis outline
- `code_interpreter` - Generate writing timeline and page estimates
- `transform` - Format outline with timeline
- `file_write` - Save outline to file

## Category

Education