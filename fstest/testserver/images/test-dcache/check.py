#!/usr/bin/env python3
"""Check local dCache authentication, redirect-before-body, and data integrity."""

import base64
import hashlib
import http.client
import ipaddress
import os
import sys
import urllib.parse
import uuid


def connect(url):
    parsed = urllib.parse.urlsplit(url)
    if parsed.scheme != "http" or not ipaddress.ip_address(parsed.hostname).is_private:
        raise ValueError("expected the private HTTP test server")
    return http.client.HTTPConnection(parsed.hostname, parsed.port or 80, timeout=5), parsed


def request(url, method, authorization=None, body=None, headers=None):
    conn, parsed = connect(url)
    options = dict(headers or {})
    if authorization is not None:
        options["Authorization"] = authorization
    try:
        path = urllib.parse.urlunsplit(("", "", parsed.path, parsed.query, ""))
        conn.request(method, path, body=body, headers=options)
        response = conn.getresponse()
        return response.status, response.getheader("Location"), response.read()
    finally:
        conn.close()


def check(base, user, password):
    authorization = "Basic " + base64.b64encode((user + ":" + password).encode()).decode()
    status, _, _ = request(base, "PROPFIND", authorization, headers={"Depth": "0"})
    if status != 207:
        raise RuntimeError("authenticated PROPFIND returned %d" % status)
    wrong = "Basic " + base64.b64encode((user + ":incorrect").encode()).decode()
    status, _, _ = request(base, "PROPFIND", wrong, headers={"Depth": "0"})
    if status != 401:
        raise RuntimeError("incorrect password returned %d" % status)

    url = base.rstrip("/") + "/rclone-readiness-" + uuid.uuid4().hex
    try:
        conn, parsed = connect(url)
        try:
            conn.putrequest("PUT", parsed.path)
            conn.putheader("Authorization", authorization)
            conn.putheader("Content-Length", "1048576")
            conn.putheader("Expect", "100-continue")
            conn.putheader("Connection", "close")
            conn.endheaders()
            with conn.sock.makefile("rb") as reader:
                line = reader.readline(8192)
                if not line:
                    raise RuntimeError("door closed before redirect")
                status = int(line.split()[1])
                headers = http.client.parse_headers(reader)
            if status not in (307, 308):
                raise RuntimeError("expected redirect before body, got %d" % status)
            location = headers.get("Location")
            if not location:
                raise RuntimeError("redirect has no Location")
            target = urllib.parse.urljoin(url, location)
        finally:
            conn.close()

        # Only generate the body after the door has issued the redirect.
        payload = os.urandom(1048576)
        status, _, _ = request(target, "PUT", body=payload)
        if status not in (200, 201, 204):
            raise RuntimeError("pool PUT returned %d" % status)
        current = url
        for _ in range(6):
            auth = authorization if urllib.parse.urlsplit(current).netloc == parsed.netloc else None
            status, location, data = request(current, "GET", auth)
            if status in (301, 302, 303, 307, 308) and location:
                current = urllib.parse.urljoin(current, location)
                continue
            if status != 200 or hashlib.sha256(data).digest() != hashlib.sha256(payload).digest():
                raise RuntimeError("readback status or SHA-256 mismatch")
            break
        else:
            raise RuntimeError("too many read redirects")
    finally:
        status, _, _ = request(url, "DELETE", authorization)
        if status not in (204, 404):
            raise RuntimeError("readiness cleanup returned %d" % status)
    print("dCache ready: authenticated, redirect before body, SHA-256 verified")


if __name__ == "__main__":
    try:
        check(*sys.argv[1:])
    except Exception as error:
        print("dCache readiness: %s" % error, file=sys.stderr)
        sys.exit(1)
