#!/usr/bin/env python
"""
Update the authors.md file with the authors from the git log
"""

import re
import subprocess

AUTHORS = "docs/content/authors.md"
IGNORE = [ "nick@raig-wood.com" ]

def load():
    """
    returns a set of emails already in authors.md
    """
    with open(AUTHORS) as fd:
        authors = fd.read()
    emails = set(re.findall(r"<(.*?)>", authors))
    emails.update(IGNORE)
    return emails

def add_email(name, email):
    """
    adds the email passed in to the end of authors.md
    """
    print "Adding %s <%s>" % (name, email)
    with open(AUTHORS, "a+") as fd:
        print >>fd, "  * %s <%s>" % (name, email)
    subprocess.check_call(["git", "commit", "-m", "Add %s to contributors" % name, AUTHORS])
    
def main():
    out = subprocess.check_output(["git", "log", '--reverse', '--format=%an|%ae', "master"])

    previous = load()
    for line in out.split("\n"):
        line = line.strip()
        if line == "":
            continue
        name, email = line.split("|")
        if email in previous:
            continue
        previous.add(email)
        add_email(name, email)

if __name__ == "__main__":
    main()
