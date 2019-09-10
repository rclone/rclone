#!/usr/bin/env python3
"""
A demo proxy for rclone serve sftp/webdav/ftp etc

This takes the incoming user/pass and converts it into an sftp backend
running on localhost.
"""

import sys
import json

def main():
    i = json.load(sys.stdin)
    o = {
        "type": "sftp",              # type of backend
        "_root": "",                 # root of the fs
        "_obscure": "pass",          # comma sep list of fields to obscure
        "user": i["user"],
        "pass": i["pass"],
        "host": "127.0.0.1",
    }
    json.dump(o, sys.stdout, indent="\t")

if __name__ == "__main__":
    main()
