#!/usr/bin/env python3
"""
Make single page versions of the documentation for release and
conversion into man pages etc.
"""

import os
import re
import time
from datetime import datetime

docpath = "docs/content"
outfile = "MANUAL.md"

# Order to add docs segments to make outfile
docs = [
    "_index.md",
    "install.md",
    "docs.md",
    "remote_setup.md",
    "filtering.md",
    "gui.md",
    "rc.md",
    "overview.md",
    "flags.md",
    "docker.md",
    "bisync.md",

    # Keep these alphabetical by full name
    "fichier.md",
    "alias.md",
    "amazonclouddrive.md",
    "s3.md",
    "b2.md",
    "box.md",
    "cache.md",
    "chunker.md",
    "sharefile.md",
    "crypt.md",
    "compress.md",
    "combine.md",
    "dropbox.md",
    "filefabric.md",
    "ftp.md",
    "googlecloudstorage.md",
    "drive.md",
    "googlephotos.md",
    "hasher.md",
    "hdfs.md",
    "http.md",
    "hubic.md",
    "internetarchive.md",
    "jottacloud.md",
    "koofr.md",
    "mailru.md",
    "mega.md",
    "memory.md",
    "netstorage.md",
    "azureblob.md",
    "onedrive.md",
    "opendrive.md",
    "qingstor.md",
    "sia.md",
    "swift.md",
    "pcloud.md",
    "premiumizeme.md",
    "putio.md",
    "seafile.md",
    "sftp.md",
    "storj.md",
    "sugarsync.md",
    "tardigrade.md",            # stub only to redirect to storj.md
    "uptobox.md",
    "union.md",
    "webdav.md",
    "yandex.md",
    "zoho.md",

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
    # Interpret img shortcodes
    # {{< img ... >}}
    contents = re.sub(r'\{\{<\s*img\s+(.*?)>\}\}', r"<img \1>", contents)
    # Make any img tags absolute
    contents = re.sub(r'(<img.*?src=")/', r"\1https://rclone.org/", contents)
    # Make [...](/links/) absolute
    contents = re.sub(r'\]\((\/.*?\/(#.*)?)\)', r"](https://rclone.org\1)", contents)
    # Add additional links on the front page
    contents = re.sub(r'\{\{< rem MAINPAGELINK >\}\}', "- [Donate.](https://rclone.org/donate/)", contents)
    # Interpret provider shortcode
    # {{< provider name="Amazon S3" home="https://aws.amazon.com/s3/" config="/s3/" >}}
    contents = re.sub(r'\{\{<\s*provider.*?name="(.*?)".*?>\}\}', r"- \1", contents)
    # Remove remaining shortcodes
    contents = re.sub(r'\{\{<.*?>\}\}', r"", contents)
    contents = re.sub(r'\{\{%.*?%\}\}', r"", contents)
    return contents

def check_docs(docpath):
    """Check all the docs are in docpath"""
    files = set(f for f in os.listdir(docpath) if f.endswith(".md"))
    files -= set(ignore_docs)
    docs_set = set(docs)
    if files == docs_set:
        return
    print("Files on disk but not in docs variable: %s" % ", ".join(files - docs_set))
    print("Files in docs variable but not on disk: %s" % ", ".join(docs_set - files))
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
    command_docs = read_commands(docpath).replace("\\", "\\\\") # escape \ so we can use command_docs in re.sub
    build_date = datetime.utcfromtimestamp(
            int(os.environ.get('SOURCE_DATE_EPOCH', time.time())))
    with open(outfile, "w") as out:
        out.write("""\
%% rclone(1) User Manual
%% Nick Craig-Wood
%% %s

""" % build_date.strftime("%b %d, %Y"))
        for doc in docs:
            contents = read_doc(doc)
            # Substitute the commands into doc.md
            if doc == "docs.md":
                contents = re.sub(r"The main rclone commands.*?for the full list.", command_docs, contents, 0, re.S)
            out.write(contents)
    print("Written '%s'" % outfile)

if __name__ == "__main__":
    main()
