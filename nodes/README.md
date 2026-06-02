# Node Template

This is a template for creating your own llm-box nodes!

## How to Create a Node

1. **Copy this template** and rename the folder to your node name
2. **Edit metadata.yaml** - set your node's name and description
3. **Write your entry script** - main.py, main.sh, or compiled binary
4. **Test it** - run llm-box to see if your node is loaded
5. **Contribute** - submit a pull request!

## metadata.yaml Format

```yaml
name: "your_node_name"
description: "A short description of what your node does"
entry: "main.py"  # or "main.sh", "binary", etc.
```

## Input/Output Protocol

Your node will receive JSON via stdin:

```json
{
  "input": "The text input from the previous node",
  "params": {
    "param1": "value1",
    "param2": "value2"
  }
}
```

Your node should write the output text to stdout (no JSON wrapping needed!).

## Examples

### Python Node

See the `echo` node for a simple Python example.

### Bash Node

```bash
#!/bin/bash
read -r input
params=$(echo "$input" | python3 -c "import sys, json; d=json.load(sys.stdin); print(d.get('params', {}).get('suffix', ''))")
echo "${input}${params}"
```

### Go Node

Compile a Go binary and set `entry: "mybinary"`

## Tips

- Keep nodes simple and focused on one task
- Document your params in metadata description
- Handle errors gracefully - write errors to stderr
- Make sure your script is executable (chmod +x)
