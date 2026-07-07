"""
xAI API Function Calling Example for llm-box

This example demonstrates how to use llm-box tools with the xAI API
(Grok) function calling feature.

Prerequisites:
  pip install openai pyyaml

Set environment variable:
  export XAI_API_KEY="your-xai-api-key"
"""

import json
import os
import subprocess

from openai import OpenAI

# Initialize xAI client
client = OpenAI(
    api_key=os.environ.get("XAI_API_KEY"),
    base_url="https://api.x.ai/v1",
)

# Load tool definitions
with open(os.path.join(os.path.dirname(__file__), "tools.json")) as f:
    tools = json.load(f)["tools"]


def execute_tool_call(name: str, arguments: dict) -> str:
    """Execute a tool call by running llm-box CLI commands."""
    if name == "create_workflow":
        result = subprocess.run(
            ["llm-box", "create", arguments["description"]],
            capture_output=True, text=True,
        )
        return result.stdout or result.stderr

    elif name == "run_workflow":
        result = subprocess.run(
            ["llm-box", "run", arguments["file"]],
            capture_output=True, text=True,
        )
        return result.stdout or result.stderr

    elif name == "run_workflow_yaml":
        # Write YAML to temp file, then run
        import tempfile
        with tempfile.NamedTemporaryFile(
            mode="w", suffix=".yaml", delete=False
        ) as f:
            f.write(arguments["yaml"])
            tmpfile = f.name
        try:
            result = subprocess.run(
                ["llm-box", "run", tmpfile],
                capture_output=True, text=True,
            )
            return result.stdout or result.stderr
        finally:
            os.unlink(tmpfile)

    elif name == "list_nodes":
        result = subprocess.run(
            ["llm-box", "list"],
            capture_output=True, text=True,
        )
        return result.stdout or result.stderr

    elif name == "validate_workflow":
        result = subprocess.run(
            ["llm-box", "validate", arguments["file"]],
            capture_output=True, text=True,
        )
        return result.stdout or result.stderr

    return f"Unknown tool: {name}"


def chat_with_grok(user_message: str) -> str:
    """Send a message to Grok with llm-box tools available."""
    messages = [{"role": "user", "content": user_message}]

    while True:
        response = client.chat.completions.create(
            model="grok-3",
            messages=messages,
            tools=tools,
        )

        choice = response.choices[0]

        # If Grok wants to call a tool
        if choice.finish_reason == "tool_calls":
            for tool_call in choice.message.tool_calls:
                fn_name = tool_call.function.name
                fn_args = json.loads(tool_call.function.arguments)

                print(f"  Tool call: {fn_name}({fn_args})")
                result = execute_tool_call(fn_name, fn_args)
                print(f"  Result: {result[:200]}...")

                messages.append(choice.message)
                messages.append({
                    "role": "tool",
                    "tool_call_id": tool_call.id,
                    "content": result,
                })
        else:
            # Grok returned a final text response
            return choice.message.content


if __name__ == "__main__":
    # Example: Ask Grok to create and run a workflow
    result = chat_with_grok(
        "Create a workflow that fetches the top 5 stories from "
        "https://hacker-news.firebaseio.com/v0/topstories.json "
        "and saves them to a file called top_stories.json"
    )
    print("\nGrok's response:")
    print(result)
