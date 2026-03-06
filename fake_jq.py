#!/usr/bin/env python3
import sys
import json
import re

def get_value(data, path):
    parts = path.split('.')
    for part in parts:
        if not part: continue
        if isinstance(data, dict):
            data = data.get(part)
        else:
            return None
    return data

def main():
    raw_input = sys.stdin.read()
    
    # Try to find JSON in the input (in case there's extra text)
    json_match = re.search(r'\{.*\}|\[.*\]', raw_input, re.DOTALL)
    if json_match:
        content = json_match.group(0)
    else:
        content = raw_input

    try:
        data = json.loads(content)
    except json.JSONDecodeError:
        # If it's not JSON, just print it as is (similar to jq behavior on non-json)
        print(raw_input.strip())
        return

    # Handle arguments
    if len(sys.argv) > 1:
        arg = sys.argv[1]
        
        # Simple -r flag handling
        raw_output = False
        if arg == '-r':
            raw_output = True
            if len(sys.argv) > 2:
                arg = sys.argv[2]
            else:
                arg = '.'

        # Simple path handling (e.g., .token or .user_id)
        if arg.startswith('.'):
            val = get_value(data, arg[1:])
            if val is not None:
                if raw_output and isinstance(val, str):
                    print(val)
                else:
                    print(json.dumps(val, indent=2))
            else:
                if not raw_output:
                    print("null")
        else:
            print(json.dumps(data, indent=2))
    else:
        print(json.dumps(data, indent=2))

if __name__ == "__main__":
    main()
