#!/usr/bin/python3
"""
Generate a markdown changelog for the rclone project
"""

import os
import sys
import re
import datetime
import subprocess
from collections import defaultdict

IGNORE_RES = [
    r"^Add .* to contributors$",
    r"^Start v\d+\.\d+(\.\d+)?-DEV development$",
    r"^Version v\d+\.\d+(\.\d+)?$",
]

IGNORE_RE = re.compile("(?:" + "|".join(IGNORE_RES) + ")")

CATEGORY = re.compile(r"(^[\w/ ]+(?:, *[\w/ ]+)*):\s*(.*)$")

backends = [ x for x in os.listdir("backend") if x != "all"]

backend_aliases = {
    "google cloud storage" : "googlecloudstorage",
    "gcs" : "googlecloudstorage",
    "azblob" : "azureblob",
    "mountlib": "mount",
    "cmount": "mount",
    "mount/cmount": "mount",
}

backend_titles = {
    "googlecloudstorage": "Google Cloud Storage",
    "azureblob": "Azure Blob",
    "ftp": "FTP",
    "sftp": "SFTP",
    "http": "HTTP",
    "webdav": "WebDAV",
}

STRIP_FIX_RE = re.compile(r"(\s+-)?\s+((fixes|addresses)\s+)?#\d+", flags=re.I)

STRIP_PATH_RE = re.compile(r"^(backend|fs)/")

IS_FIX_RE = re.compile(r"\b(fix|fixes)\b", flags=re.I)

def make_out(data, indent=""):
    """Return a out, lines the first being a function for output into the second"""
    out_lines = []
    def out(category, title=None):
        if title == None:
            title = category
        lines = data.get(category)
        if not lines:
            return
        del(data[category])
        if indent != "" and len(lines) == 1:
            out_lines.append(indent+"* " + title+": " + lines[0])
            return
        out_lines.append(indent+"* " + title)
        for line in lines:
            out_lines.append(indent+"    * " + line)
    return out, out_lines


def process_log(log):
    """Process the incoming log into a category dict of lists"""
    by_category = defaultdict(list)
    for log_line in reversed(log.split("\n")):
        log_line = log_line.strip()
        hash, author, timestamp, message = log_line.split("|", 3)
        message = message.strip()
        if IGNORE_RE.search(message):
            continue
        match = CATEGORY.search(message)
        categories = "UNKNOWN"
        if match:
            categories = match.group(1).lower()
            message = match.group(2)
        message = STRIP_FIX_RE.sub("", message)
        message = message +" ("+author+")"
        message = message[0].upper()+message[1:]
        seen = set()
        for category in categories.split(","):
            category = category.strip()
            category = STRIP_PATH_RE.sub("", category)
            category = backend_aliases.get(category, category)
            if category in seen:
                continue
            by_category[category].append(message)
            seen.add(category)
            #print category, hash, author, timestamp, message
    return by_category

def main():
    if len(sys.argv) != 3:
        print("Syntax: %s vX.XX vX.XY" % sys.argv[0], file=sys.stderr)
        sys.exit(1)
    version, next_version = sys.argv[1], sys.argv[2]
    log = subprocess.check_output(["git", "log", '''--pretty=format:%H|%an|%aI|%s'''] + [version+".."+next_version])
    log = log.decode("utf-8")
    by_category = process_log(log)

    # Output backends first so remaining in by_category are core items
    out, backend_lines = make_out(by_category)
    out("mount", title="Mount")
    out("vfs", title="VFS")
    out("local", title="Local")
    out("cache", title="Cache")
    out("crypt", title="Crypt")
    backend_names = sorted(x for x in list(by_category.keys()) if x in backends)
    for backend_name in backend_names:
        if backend_name in backend_titles:
            backend_title = backend_titles[backend_name]
        else:
            backend_title = backend_name.title()
        out(backend_name, title=backend_title)

    # Split remaining in by_category into new features and fixes
    new_features = defaultdict(list)
    bugfixes = defaultdict(list)
    for name, messages in by_category.items():
        for message in messages:
            if IS_FIX_RE.search(message):
                bugfixes[name].append(message)
            else:
                new_features[name].append(message)

    # Output new features
    out, new_features_lines = make_out(new_features, indent="    ")
    for name in sorted(new_features.keys()):
        out(name)

    # Output bugfixes
    out, bugfix_lines = make_out(bugfixes, indent="    ")
    for name in sorted(bugfixes.keys()):
        out(name)

    # Read old changelog and split
    with open("docs/content/changelog.md") as fd:
        old_changelog = fd.read()
    heading = "# Changelog"
    i = old_changelog.find(heading)
    if i < 0:
        raise AssertionError("Couldn't find heading in old changelog")
    i += len(heading)
    old_head, old_tail = old_changelog[:i], old_changelog[i:]

    # Update the build date
    old_head = re.sub(r"\d\d\d\d-\d\d-\d\d", str(datetime.date.today()), old_head)

    # Output combined changelog with new part
    sys.stdout.write(old_head)
    today = datetime.date.today()
    new_features = "\n".join(new_features_lines)
    bugfixes = "\n".join(bugfix_lines)
    backend_changes = "\n".join(backend_lines)
    sys.stdout.write("""

## %(next_version)s - %(today)s

[See commits](https://github.com/rclone/rclone/compare/%(version)s...%(next_version)s)

* New backends
* New commands
* New Features
%(new_features)s
* Bug Fixes
%(bugfixes)s
%(backend_changes)s""" % locals())
    sys.stdout.write(old_tail)
                

if __name__ == "__main__":
    main()
