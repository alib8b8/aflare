# Reading Comprehension Generator

Reading comprehension test generator with leveled passages, diverse question types, readability statistics, and answer keys.

## Description

This workflow generates complete reading comprehension tests including a leveled passage, diverse question types (main idea, detail, inference, vocabulary), answer choices, and an answer key with explanations. It calculates readability statistics to ensure appropriate difficulty.

## Usage Example

```yaml
params:
  passage_topic: "The Water Cycle"
  reading_level: "intermediate"
  passage_length: "medium"
  question_types: ["main_idea", "detail", "inference", "vocabulary"]
  question_count: 8
  include_answers: true
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| passage_topic | string | yes | - | Reading passage topic |
| reading_level | string | no | intermediate | Reading difficulty |
| passage_length | string | no | medium | Passage length |
| question_types | array | no | ["main_idea","detail","inference","vocabulary"] | Question types |
| question_count | integer | no | 8 | Number of questions |
| include_answers | boolean | no | true | Include answer key |

## Nodes Used

- `agent` - Generate passage and comprehension questions
- `code_interpreter` - Calculate readability statistics
- `transform` - Format test into structured output
- `file_write` - Save test to file

## Category

Education