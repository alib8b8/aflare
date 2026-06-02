#!/usr/bin/env python3
"""
Template entry script for an llm-box community node.

The node receives a JSON payload via stdin of the form:
    {
      "input": "text from the previous step",
      "params": {"key": "value", ...}
    }

It must write the output text to stdout.
"""
import json
import sys


def main() -> None:
    payload = json.load(sys.stdin)
    input_text = payload.get("input", "")
    params = payload.get("params", {})

    # TODO: implement your node logic here
    output = f"received: {input_text}"

    sys.stdout.write(output)


if __name__ == "__main__":
    main()
