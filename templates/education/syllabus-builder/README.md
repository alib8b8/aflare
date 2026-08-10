# Syllabus Builder

Course syllabus and schedule builder with calendar integration. Creates comprehensive syllabi with weekly schedules, readings, assignments, and grading policies.

## Description

This workflow builds a professional course syllabus by combining AI-powered content generation with real academic resource lookups. It produces a complete syllabus including learning objectives, weekly schedules, assignment calendars, and institutional policies.

## Usage Example

```yaml
params:
  course_name: "Introduction to Computer Science"
  course_code: "CS101"
  semester: "Fall 2026"
  weeks: 16
  topics: ["Algorithms", "Data Structures", "Programming Basics", "Complexity Analysis"]
  textbooks: ["Introduction to Algorithms, 4th Edition"]
  grading_policy: "Assignments 40%, Midterm 25%, Final 35%"
  institution: "State University"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| course_name | string | yes | - | Course name |
| course_code | string | yes | - | Course code/number |
| semester | string | yes | - | Academic term |
| weeks | integer | no | 16 | Semester duration in weeks |
| topics | array | yes | - | Course topics/modules |
| textbooks | array | no | [] | Required textbooks |
| grading_policy | string | no | "" | Custom grading policy |
| institution | string | no | "" | Institution name |

## Nodes Used

- `http_request` - Fetch academic resources from OpenAlex
- `agent` - Build comprehensive syllabus content
- `transform` - Format syllabus into structured markdown
- `file_write` - Save syllabus to file

## Category

Education