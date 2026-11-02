#!/usr/bin/env python3
"""Summarise an s3-tests JUnit XML result and check it against the
list of tests expected to pass.

usage: report.py [--update] [--s3tests DIR] junit.xml expected-pass.txt

Exits 1 if any test in expected-pass.txt which was run didn't pass.
With --update, rewrites expected-pass.txt with the tests which passed
(keeping expected tests which weren't run).
"""
import argparse
import ast
import collections
import os
import re
import sys
import xml.etree.ElementTree as ET

# Feature areas, matched in order against the test name and its pytest
# markers. The first match wins.
FEATURES = [
    ('object_lock', ['object_lock', 'retention', 'legal_hold']),
    ('versioning', ['versioning', 'version', 'delete_marker']),
    ('lifecycle', ['lifecycle', 'transition', 'restore', 'read_through']),
    ('logging', ['bucket_logging']),
    ('encryption', ['encryption', 'sse', 'encrypted']),
    ('policy', ['bucket_policy', 'policy']),
    ('acl/ownership', ['acl', 'grant', 'ownership', 'access_bucket', 'public']),
    ('tagging', ['tagging', 'tags']),
    ('cors', ['cors']),
    ('post_object', ['post_object']),
    ('checksum', ['checksum']),
    ('conditional', ['conditional_write', 'if_match', 'if_none_match', 'ifmatch',
                     'ifnonematch', 'if_modified', 'ifmodified', 'unmodified']),
    ('multipart', ['multipart']),
    ('copy', ['copy']),
    ('list', ['list', 'delimiter', 'prefix', 'marker', 'max_keys', 'maxkeys',
              'continuationtoken', 'startafter', 'fetchowner', 'encoding']),
    ('auth/headers', ['auth', 'contentlength', 'content_length', 'md5', 'expect',
                      'date', 'header', 'anonymous', 'presigned', 'cookie']),
    ('bucket', ['bucket_create', 'bucket_name', 'bucket_head', 'bucket_delete',
                'buckets_create', 'bucket_list_long_name']),
    ('range', ['range']),
    ('metadata', ['metadata']),
]


def load_markers(s3tests):
    """Return {test name: set of pytest markers} from the s3-tests source."""
    markers = {}
    d = os.path.join(s3tests, 's3tests', 'functional')
    for fn in ('test_s3.py', 'test_headers.py'):
        try:
            tree = ast.parse(open(os.path.join(d, fn)).read())
        except OSError:
            continue
        for n in tree.body:
            if isinstance(n, ast.FunctionDef) and n.name.startswith('test_'):
                markers[n.name] = {m.group(1) for m in
                                   (re.match(r'pytest\.mark\.(\w+)', ast.unparse(dec))
                                    for dec in n.decorator_list) if m}
    return markers


def feature(name, markers):
    """Return the feature area of test name."""
    marks = markers.get(name.split('[')[0], set())
    for area, pats in FEATURES:
        if any(p in name or p in marks for p in pats):
            return area
    return 'other'


def load_results(path):
    """Return {test id: (status, message)} from a JUnit XML file."""
    results = {}
    for tc in ET.parse(path).iter('testcase'):
        tid = tc.get('classname').replace('.', '/') + '.py::' + tc.get('name')
        status, msg = 'pass', ''
        for c in tc:
            if c.tag in ('failure', 'error'):
                status = 'fail'
                msg = msg or c.get('message') or ''
            elif c.tag == 'skipped' and status == 'pass':
                status = 'skip'
        results[tid] = (status, re.sub(r'\s+', ' ', msg))
    return results


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--update', action='store_true')
    p.add_argument('--s3tests', default='')
    p.add_argument('junit')
    p.add_argument('expected')
    args = p.parse_args()

    results = load_results(args.junit)
    markers = load_markers(args.s3tests) if args.s3tests else {}

    table = collections.defaultdict(collections.Counter)
    reasons = collections.Counter()
    for tid, (status, msg) in results.items():
        table[feature(tid.split('::')[1], markers)][status] += 1
        if status == 'fail':
            reasons[re.sub(r'rclone-[a-z0-9]+-\d+', 'BUCKET', msg)[:110]] += 1
    total = collections.Counter()
    print(f"{'feature':16s} {'pass':>5s} {'fail':>5s} {'skip':>5s}")
    for area, c in sorted(table.items(), key=lambda kv: -sum(kv[1].values())):
        print(f"{area:16s} {c['pass']:5d} {c['fail']:5d} {c['skip']:5d}")
        total.update(c)
    print(f"{'TOTAL':16s} {total['pass']:5d} {total['fail']:5d} {total['skip']:5d}")
    print("\nCommonest failures:")
    for msg, n in reasons.most_common(15):
        print(f"{n:5d} {msg}")

    try:
        with open(args.expected) as fh:
            expected = {l.strip() for l in fh if l.strip() and not l.startswith('#')}
    except FileNotFoundError:
        expected = set()
    passed = {t for t, (s, _) in results.items() if s == 'pass'}
    regressions = sorted(t for t in expected if t in results and t not in passed)
    new = sorted(passed - expected)

    if new:
        print(f"\n{len(new)} tests pass which aren't in {os.path.basename(args.expected)}:")
        for t in new:
            print("  " + t)
    if regressions:
        print(f"\n{len(regressions)} REGRESSIONS - expected to pass but didn't:")
        for t in regressions:
            print(f"  {t}: {results[t][1][:150]}")

    if args.update:
        keep = (expected - set(results)) | passed
        with open(args.expected, 'w') as fh:
            fh.write("# Tests expected to pass - maintained by run.sh --update\n")
            fh.writelines(t + "\n" for t in sorted(keep))
        print(f"\nWrote {len(keep)} tests to {args.expected}")
        return 0
    return 1 if regressions else 0


if __name__ == '__main__':
    sys.exit(main())
