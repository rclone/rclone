#!/usr/bin/python
"""
Make single page versions of the documentation for release and
conversion into man pages etc.
"""

import os
import re
from datetime import datetime

docpath = "docs/content"
outfile = "MANUAL.md"

# Order to add docs segments to make outfile
docs = [
    "about.md",
    "install.md",
    "docs.md",
    "overview.md",
    "drive.md",
    "s3.md",
    "swift.md",
    "dropbox.md",
    "googlecloudstorage.md",
    "amazonclouddrive.md",
    "local.md",
    "changelog.md",
    "bugs.md",
    "faq.md",
    "licence.md",
    "authors.md",
    "contact.md",
]

# Docs which aren't made into outfile
ignore_docs = [
    "downloads.md",
    "privacy.md",
]

def read_doc(doc):
    """Read file as a string"""
    path = os.path.join(docpath, doc)
    with open(path) as fd:
        contents = fd.read()
    parts = contents.split("---\n", 2)
    if len(parts) != 3:
        raise ValueError("Couldn't find --- markers: found %d parts" % len(parts))
    contents = parts[2].strip()+"\n\n"
    # Remove icons
    contents = re.sub(r'<i class="fa.*?</i>\s*', "", contents)
    # Make [...](/links/) absolute
    contents = re.sub(r'\((\/.*?\/)\)', r"(http://rclone.org\1)", contents)
    return contents

def check_docs(docpath):
    """Check all the docs are in docpath"""
    files = set(f for f in os.listdir(docpath) if f.endswith(".md"))
    files -= set(ignore_docs)
    docs_set = set(docs)
    if files == docs_set:
        return
    print "Files on disk but not in docs variable: %s" % ", ".join(files - docs_set)
    print "Files in docs variable but not on disk: %s" % ", ".join(docs_set - files)
    raise ValueError("Missing files")

def main():
    check_docs(docpath)
    with open(outfile, "w") as out:
        out.write("""\
%% rclone(1) User Manual
%% Nick Craig-Wood
%% %s

""" % datetime.now().strftime("%b %d, %Y"))
        for doc in docs:
            out.write(read_doc(doc))
    print "Written '%s'" % outfile

if __name__ == "__main__":
    main()
