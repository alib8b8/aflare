# Math Problem Generator

Math problem and solution generator with step-by-step explanations, hints, common mistakes, and solution verification.

## Description

This workflow generates math problems across various topics and difficulty levels. It creates a problem framework, generates detailed problems with step-by-step solutions, and verifies solution completeness including hints and common mistake warnings.

## Usage Example

```yaml
params:
  topic: "Quadratic Equations"
  problem_count: 10
  difficulty: "medium"
  problem_types: ["computation", "word_problem", "proof"]
  include_solutions: true
  grade_level: "high_school"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| topic | string | yes | - | Math topic |
| problem_count | integer | no | 10 | Number of problems |
| difficulty | string | no | medium | Difficulty level |
| problem_types | array | no | ["computation","word_problem","proof"] | Problem types |
| include_solutions | boolean | no | true | Include solutions |
| grade_level | string | no | high_school | Target grade level |

## Nodes Used

- `code_interpreter` - Generate problem framework and verify solutions
- `agent` - Create detailed math problems with solutions
- `file_write` - Save problems to file

## Category

Education