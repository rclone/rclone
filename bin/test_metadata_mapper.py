#!/usr/bin/env python3
"""
A demo metadata mapper
"""

import sys
import json

def main():
    i = json.load(sys.stdin)
    # Add tag to description
    metadata = i["Metadata"]
    if "description" in metadata:
        metadata["description"] += " [migrated from domain1]"
    else:
        metadata["description"] = "[migrated from domain1]"
    # Modify owner
    if "owner" in metadata:
        metadata["owner"] = metadata["owner"].replace("domain1.com", "domain2.com")
    o = { "Metadata": metadata }
    json.dump(o, sys.stdout, indent="\t")

if __name__ == "__main__":
    main()
