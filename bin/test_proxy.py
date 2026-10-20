#!/usr/bin/env python3
"""
A demo proxy for rclone serve sftp/webdav/ftp, etc.

This takes the incoming user/pass and converts it into an sftp backend
running on localhost.

Logins from outside ALLOWED_NETWORKS are refused.
"""

import sys
import json
import ipaddress

ALLOWED_NETWORKS = ["127.0.0.0/8", "::1/128"]

def allowed(ip):
    """Return True if ip is in one of ALLOWED_NETWORKS."""
    if ip is None:
        return False
    address = ipaddress.ip_address(ip)
    return any(address in ipaddress.ip_network(network) for network in ALLOWED_NETWORKS)

def main():
    i = json.load(sys.stdin)
    # Exiting non zero refuses the login - rclone logs whatever we
    # write on stderr, so say why.
    if not allowed(i.get("client_ip")):
        sys.exit("client_ip %s not allowed" % i.get("client_ip"))
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
