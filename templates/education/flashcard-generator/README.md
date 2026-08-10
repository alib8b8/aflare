# Flashcard Generator

Flashcard deck generator from study materials with multiple formats and difficulty ratings for spaced repetition.

## Description

This workflow extracts key concepts from study materials and converts them into a structured flashcard deck. It supports front/back, cloze deletion, and multiple choice formats with difficulty ratings and category organization.

## Usage Example

```yaml
params:
  source_text: "Mitochondria are membrane-bound organelles that generate most of the chemical energy..."
  card_count: 20
  card_format: "front_back"
  categories: ["Biology", "Cell Structure"]
  difficulty_levels: true
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| source_text | string | yes | - | Source study material |
| card_count | integer | no | 20 | Number of flashcards |
| card_format | string | no | front_back | Card format |
| categories | array | no | [] | Organization categories |
| difficulty_levels | boolean | no | true | Include difficulty ratings |

## Nodes Used

- `agent` - Extract knowledge and generate flashcards
- `code_interpreter` - Analyze deck statistics
- `transform` - Format deck into structured output
- `file_write` - Save flashcard deck to file

## Category

Education