# Organizational Chart Generator

Organizational chart generator with hierarchy visualization and structural analytics.

## Description

Build organizational charts from employee data with reporting relationships, support multiple output formats (mermaid, JSON, DOT), and provide structural analysis including span of control, hierarchy depth, vacancy impact, and reorganization recommendations.

## Usage Example

```yaml
params:
  organization_name: "Acme Corp Engineering"
  employees:
    - id: "E001"
      name: "Sarah Chen"
      title: "VP Engineering"
      department: "Engineering"
      level: 5
      location: "San Francisco"
      tenure_years: 4
      manager_id: null
    - id: "E002"
      name: "Mike Torres"
      title: "Director, Platform"
      department: "Engineering"
      level: 4
      location: "San Francisco"
      tenure_years: 3
      manager_id: "E001"
    - id: "E003"
      name: "Lisa Park"
      title: "Director, Product Eng"
      department: "Engineering"
      level: 4
      location: "New York"
      tenure_years: 2
      manager_id: "E001"
    - id: "E004"
      name: "VACANT"
      title: "Director, Infrastructure"
      department: "Engineering"
      level: 4
      location: "Remote"
      tenure_years: 0
      manager_id: "E001"
      is_vacant: true
  chart_format: "mermaid"
  color_by: "department"
  max_depth: 10
  group_by: "department"
  output_file: "engineering_org_chart.mmd"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| organization_name | string | Yes | - | Organization name |
| employees | array | Yes | - | Employee data with manager relationships |
| chart_format | string | No | mermaid | mermaid, plantuml, json, dot |
| include_vacant | boolean | No | true | Show vacant positions |
| color_by | string | No | department | department, level, location, tenure |
| max_depth | integer | No | 10 | Maximum hierarchy depth |
| group_by | string | No | department | Grouping strategy |
| output_file | string | No | org_chart.mmd | Output file path |

## Nodes Used

- **code_interpreter** (×2): Build hierarchy tree and compute span of control; generate chart in specified format
- **agent**: Analyze organizational structure for bottlenecks and recommendations
- **file_write**: Save chart output with statistics and analysis

## Category

HR > Workforce Planning