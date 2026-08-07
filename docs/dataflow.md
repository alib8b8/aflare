# Cross-Step Data Flow

aflare supports flexible data flow between workflow steps through various reference mechanisms.

## Basic Data Flow

By default, each step receives the output of the previous step as its input:

```yaml
steps:
  - node: http_request
    params:
      url: "https://api.example.com/data"
    # Output: JSON response

  - node: json_parse
    # Input: Previous step's output (JSON response)
    params:
      path: "results"
    # Output: Parsed data

  - node: agent
    # Input: Previous step's output (parsed data)
    params:
      model: gpt-4o
    # Output: Analysis result
```

## Reference Mechanisms

### `{{input}}` - Previous Step Output

The `{{input}}` placeholder refers to the **output of the immediately preceding step**:

```yaml
steps:
  - node: file_read
    params:
      path: "document.txt"

  - node: agent
    params:
      model: gpt-4o
      prompt: "Summarize this: {{input}}"
```

**Behavior**:
- Always references the output of step `i-1` when executing step `i`
- For the first step, `{{input}}` is empty unless workflow input is provided
- Can be used in both `params` and `input` fields

### `{{step.N}}` - Step by Index

Reference any step by its zero-based index:

```yaml
steps:
  - node: http_request
    params:
      url: "https://api.example.com/users"

  - node: http_request
    params:
      url: "https://api.example.com/products"

  - node: combine
    params:
      separator: "\n---\n"
    input:
      - "{{step.0}}"  # Users data
      - "{{step.1}}"  # Products data
```

**Note**: Steps are indexed sequentially (0, 1, 2, ...) regardless of branching structures.

### `{{step.name}}` - Step by Name

Reference steps by their assigned names:

```yaml
steps:
  - node: http_request
    id: fetch_users
    params:
      url: "https://api.example.com/users"

  - node: http_request
    id: fetch_products
    params:
      url: "https://api.example.com/products"

  - node: agent
    input: "Users: {{step.fetch_users}}\nProducts: {{step.fetch_products}}"
    params:
      model: gpt-4o
```

**Setup**: Assign names using the `id` field in your step definition.

### `{{var.NAME}}` - Workflow Variables

Define and reference workflow-level variables:

```yaml
name: data-processing
vars:
  api_key: "{{secret.api.service}}"
  base_url: "https://api.example.com"

steps:
  - node: http_request
    params:
      url: "{{var.base_url}}/data"
      headers: "Authorization: Bearer {{var.api_key}}"
```

## Combining Multiple Step Outputs

### Using `combine` Node

The `combine` node merges multiple inputs into a single output:

```yaml
steps:
  - node: http_request
    id: fetch_users
    params:
      url: "https://api.example.com/users"

  - node: http_request
    id: fetch_orders
    params:
      url: "https://api.example.com/orders"

  - node: combine
    input:
      - "{{step.fetch_users}}"
      - "{{step.fetch_orders}}"
    params:
      separator: "\n---\n"
    id: merged_data

  - node: agent
    input: "{{step.merged_data}}"
    params:
      model: gpt-4o
```

**`combine` Parameters**:

| Parameter | Description |
|-----------|-------------|
| `separator` | String to separate combined outputs (default: `\n---\n`) |

### Using Parallel Steps

Parallel steps automatically collect all outputs:

```yaml
steps:
  - parallel:
      - node: http_request
        params:
          url: "https://api.example.com/service-a"
      - node: http_request
        params:
          url: "https://api.example.com/service-b"
      - node: http_request
        params:
          url: "https://api.example.com/service-c"
    output_strategy: join  # Options: join, first, last, longest, shortest
    id: parallel_results

  - node: agent
    input: "{{step.parallel_results}}"
```

**Output Strategies**:

| Strategy | Description |
|----------|-------------|
| `join` | Combine all outputs with separator (default) |
| `first` | Use only the first step's output |
| `last` | Use only the last step's output |
| `longest` | Use the output with the most content |
| `shortest` | Use the output with the least content |

## Expression Syntax Summary

| Expression | Description | Example |
|------------|-------------|---------|
| `{{input}}` | Output of the previous step | Immediate predecessor |
| `{{step.0}}` | Output of step by index | Zero-based index |
| `{{step.name}}` | Output by step name/id | Named reference |
| `{{var.name}}` | Workflow variable | Defined in vars section |
| `{{env.NAME}}` | Environment variable | OS environment |
| `{{secret.GROUP.KEY}}` | Secret value | Secure storage |
| `{{file.PATH}}` | File content | Read file content |

## Advanced Data Flow Patterns

### Chaining with Custom Input

Override the default input flow:

```yaml
steps:
  - node: file_read
    params:
      path: "config.yaml"
    id: config

  - node: agent
    params:
      model: gpt-4o
      prompt: "Using config: {{step.config}}, analyze this data"
    input: "Raw data to analyze"  # Override default input
```

### Conditional Data Flow

```yaml
steps:
  - node: condition
    params:
      condition: "{{input}} contains 'error'"
    id: check_error

  - if: "{{step.check_error}}"
    steps:
      - node: agent
        params:
          model: gpt-4o
          prompt: "Fix this error: {{input}}"
    else:
      - node: agent
        params:
          model: gpt-4o
          prompt: "Summarize this success: {{input}}"
```

### Looping with Accumulated Output

```yaml
steps:
  - loop:
      for_each: "[1, 2, 3, 4, 5]"
      steps:
        - node: http_request
          params:
            url: "https://api.example.com/items/{{loop.item}}"
    output_strategy: join
    id: batch_results

  - node: agent
    input: "All results:\n{{step.batch_results}}"
    params:
      model: gpt-4o
```

## Data Type Considerations

### Text-Based Flow

All data flow in aflare is text-based. Nodes receive and return strings.

### Handling JSON Data

Use `json_parse` to extract specific fields:

```yaml
steps:
  - node: http_request
    params:
      url: "https://api.example.com/data"
    id: raw_response

  - node: json_parse
    params:
      path: "items[0].name"
    input: "{{step.raw_response}}"
    id: extracted_name

  - node: agent
    input: "Item name: {{step.extracted_name}}"
    params:
      model: gpt-4o
```

### Type Mismatch Handling

When data types don't match (e.g., planner output vs. code_review input):

1. **Use `transform` node** to convert data format
2. **Use `template_render`** to restructure content
3. **Use `agent` node** to reformat text

```yaml
steps:
  - node: planner
    params:
      task: "Review the following code"
    id: plan

  - node: transform
    params:
      pattern: "Extract code section"
      template: "Code to review: {{input}}"
    input: "{{step.plan}}"
    id: formatted_input

  - node: code_review
    input: "{{step.formatted_input}}"
    params:
      model: gpt-4o
```

## Common Patterns

### Pattern 1: Multi-Source Aggregation

```yaml
steps:
  - node: http_request
    id: source_a
    params:
      url: "https://api.a.com/data"

  - node: file_read
    id: source_b
    params:
      path: "local_data.json"

  - node: combine
    input:
      - "{{step.source_a}}"
      - "{{step.source_b}}"
    id: aggregated

  - node: agent
    input: "{{step.aggregated}}"
    params:
      model: gpt-4o
      prompt: "Analyze these data sources together"
```

### Pattern 2: Pipeline with Branching

```yaml
steps:
  - node: http_request
    params:
      url: "https://api.example.com/events"
    id: events

  - node: condition
    params:
      condition: "{{input}} contains 'critical'"
    id: is_critical

  - if: "{{step.is_critical}}"
    steps:
      - node: notify
        params:
          channel: "slack"
          message: "Critical event detected: {{step.events}}"
    else:
      - node: file_write
        params:
          path: "events.log"
          mode: "append"
        input: "{{step.events}}"
```

### Pattern 3: Reusing Early Results

```yaml
steps:
  - node: file_read
    params:
      path: "requirements.txt"
    id: requirements

  - node: agent
    input: "Analyze these requirements: {{step.requirements}}"
    params:
      model: gpt-4o
      prompt: "Generate implementation plan"
    id: plan

  - node: agent
    input: "Based on requirements:\n{{step.requirements}}\n\nAnd plan:\n{{step.plan}}\n\nGenerate code"
    params:
      model: gpt-4o
    id: code

  - node: file_write
    params:
      path: "implementation.py"
    input: "{{step.code}}"
```

## Best Practices

1. **Use descriptive IDs**: Assign meaningful names to steps you need to reference later
2. **Keep dependencies clear**: Avoid complex cross-step references that make workflows hard to follow
3. **Use `combine` for multiple sources**: Explicitly combine outputs rather than relying on implicit flow
4. **Handle type conversions**: Use transform nodes when passing data between incompatible nodes
5. **Test incrementally**: Verify each step's output before building complex data flows
