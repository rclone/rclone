#!/usr/bin/env python3
"""
A proxy for rclone serve s3
"""

import sys
import json
import os


def main():
    i = json.load(sys.stdin)
    o = {
        "type": "webdav",  # type of backend
        "_root": "",  # root of the fs
        "bearer_token": i["pass"],
        "url": os.getenv("RCLONE_WEBDAV_URL", "https://localhost:9200/webdav"),
    }
    json.dump(o, sys.stdout, indent="\t")


if __name__ == "__main__":
    main()
