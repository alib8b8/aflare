# Litigation Timeline & Deadline Tracker

## Description
A litigation timeline and deadline tracking workflow that calculates procedural deadlines based on jurisdiction and case type, generates comprehensive timelines, and provides risk assessments for tight deadlines, bottlenecks, and resource conflicts.

## Usage Example
```yaml
workflow: legal/litigation-timeline
params:
  case_name: "Acme Corp v. Beta Industries"
  jurisdiction: "US District Court, Northern District of California"
  case_type: "civil"
  filing_date: "2025-01-15"
  trial_date: "2026-03-01"
  key_events:
    - {event: "Motion to Dismiss filed", date: "2025-02-10"}
    - {event: "Motion to Dismiss denied", date: "2025-03-05"}
  output_path: "output/timeline.md"
```

## Parameters
| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| case_name | string | Yes | - | Name of the litigation case |
| jurisdiction | string | Yes | - | Court jurisdiction |
| case_type | string | Yes | - | Type of case (civil, criminal, appellate, administrative) |
| filing_date | string | Yes | - | Date the case was filed (YYYY-MM-DD) |
| key_events | array | No | [] | List of known key events with dates |
| trial_date | string | No | - | Scheduled trial date (YYYY-MM-DD) |
| model | string | No | gpt-4 | AI model to use |
| output_path | string | No | output/litigation_timeline.md | Output file path |

## Nodes Used
- **code_interpreter** (calculate_deadlines): Calculates all procedural deadlines
- **agent** (generate_timeline): Generates comprehensive litigation timeline
- **agent** (risk_assessment): Assesses timeline risks and bottlenecks
- **file_write** (save_timeline): Saves the litigation timeline

## Category
legal