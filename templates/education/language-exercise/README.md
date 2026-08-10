# Language Exercise Generator

Language learning exercise generator with vocabulary, grammar, translation, and conversation practice across CEFR levels A1-C2.

## Description

This workflow creates language learning exercises tailored to the student's proficiency level. It fetches cultural context data, generates diverse exercise types (vocabulary, grammar, translation, conversation), and structures them with estimated durations and type breakdowns.

## Usage Example

```yaml
params:
  target_language: "Spanish"
  native_language: "English"
  proficiency_level: "A2"
  exercise_types: ["vocabulary", "grammar", "translation", "conversation"]
  topic: "food_and_dining"
  exercise_count: 15
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| target_language | string | yes | - | Language being learned |
| native_language | string | no | English | Student's native language |
| proficiency_level | string | no | beginner | CEFR level (A1-C2) |
| exercise_types | array | no | ["vocabulary","grammar","translation","conversation"] | Exercise types |
| topic | string | no | general | Thematic topic |
| exercise_count | integer | no | 15 | Number of exercises |

## Nodes Used

- `http_request` - Fetch language country data for cultural context
- `agent` - Generate language learning exercises
- `code_interpreter` - Structure exercises by type and calculate metadata
- `file_write` - Save exercises to file

## Category

Education