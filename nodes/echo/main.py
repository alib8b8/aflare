#!/usr/bin/env python3
import sys
import json

def main():
    # Read input from stdin
    input_data = sys.stdin.read()
    payload = json.loads(input_data)

    input_text = payload.get("input", "")
    params = payload.get("params", {})

    prefix = params.get("prefix", "ECHO: ")
    output = f"{prefix}{input_text}"

    # Write output to stdout
    print(output, end="")

if __name__ == "__main__":
    main()
