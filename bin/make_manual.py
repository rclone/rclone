#!/usr/bin/env python
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
    "remote_setup.md",
    "filtering.md",
    "rc.md",
    "overview.md",

    # Keep these alphabetical by full name
    "alias.md",
    "amazonclouddrive.md",
    "s3.md",
    "b2.md",
    "box.md",
    "cache.md",
    "crypt.md",
    "dropbox.md",
    "ftp.md",
    "googlecloudstorage.md",
    "drive.md",
    "http.md",
    "hubic.md",
    "mega.md",
    "azureblob.md",
    "onedrive.md",
    "qingstor.md",
    "swift.md",
    "pcloud.md",
    "sftp.md",
    "webdav.md",
    "yandex.md",

    "local.md",
    "changelog.md",
    "bugs.md",
    "faq.md",
    "licence.md",
    "authors.md",
    "contact.md",
]

# Order to put the commands in - any not on here will be in sorted order
commands_order = [
    "rclone_config.md",
    "rclone_copy.md",
    "rclone_sync.md",
    "rclone_move.md",
    "rclone_delete.md",
    "rclone_purge.md",
    "rclone_mkdir.md",
    "rclone_rmdir.md",
    "rclone_check.md",
    "rclone_ls.md",
    "rclone_lsd.md",
    "rclone_lsl.md",
    "rclone_md5sum.md",
    "rclone_sha1sum.md",
    "rclone_size.md",
    "rclone_version.md",
    "rclone_cleanup.md",
    "rclone_dedupe.md",
]    

# Docs which aren't made into outfile
ignore_docs = [
    "downloads.md",
    "privacy.md",
    "donate.md",
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
    contents = re.sub(r'\((\/.*?\/)\)', r"(https://rclone.org\1)", contents)
    # Interpret provider shortcode
    # {{< provider name="Amazon S3" home="https://aws.amazon.com/s3/" config="/s3/" >}}
    contents = re.sub(r'\{\{<\s+provider.*?name="(.*?)".*?>\}\}', r"\1", contents)
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

def read_command(command):
    doc = read_doc("commands/"+command)
    doc = re.sub(r"### Options inherited from parent commands.*$", "", doc, 0, re.S)
    doc = doc.strip()+"\n"
    return doc

def read_commands(docpath):
    """Reads the commands an makes them into a single page"""
    files = set(f for f in os.listdir(docpath + "/commands") if f.endswith(".md"))
    docs = []
    for command in commands_order:
        docs.append(read_command(command))
        files.remove(command)
    for command in sorted(files):
        if command != "rclone.md":
            docs.append(read_command(command))
    return "\n".join(docs)
    
def main():
    check_docs(docpath)
    command_docs = read_commands(docpath)
    with open(outfile, "w") as out:
        out.write("""\
%% rclone(1) User Manual
%% Nick Craig-Wood
%% %s

""" % datetime.now().strftime("%b %d, %Y"))
        for doc in docs:
            contents = read_doc(doc)
            # Substitute the commands into doc.md
            if doc == "docs.md":
                contents = re.sub(r"The main rclone commands.*?for the full list.", command_docs, contents, 0, re.S)
            out.write(contents)
    print "Written '%s'" % outfile

if __name__ == "__main__":
    main()
