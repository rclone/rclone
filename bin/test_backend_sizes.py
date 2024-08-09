#!/usr/bin/env python3
"""
Test the sizes in the rclone binary of each backend by compiling
rclone with and without the backend and measuring the difference.

Run with no arguments to test all backends or a supply a list of
backends to test.
"""

all_backends = "backend/all/all.go"

# compile command which is more or less like the production builds
compile_command = ["go", "build", "--ldflags", "-s", "-trimpath"]

import os
import re
import sys
import subprocess

match_backend = re.compile(r'"github.com/rclone/rclone/backend/(.*?)"')

def read_backends():
    """
    Reads the backends file, returning a list of backends and the original file
    """
    with open(all_backends) as fd:
        orig_all = fd.read()
    # find the backends
    backends = []
    for line in orig_all.split("\n"):
        match = match_backend.search(line)
        if match:
            backends.append(match.group(1))
    return backends, orig_all

def write_all(orig_all, backend):
    """
    Write the all backends file without the backend given
    """
    with open(all_backends, "w") as fd:
        for line in orig_all.split("\n"):
            match = re.search(r'"github.com/rclone/rclone/backend/(.*?)"', line)
            # Comment out line matching backend
            if match and match.group(1) == backend:
                line = "// " + line
            fd.write(line+"\n")

def compile():
    """
    Compile the binary, returning the size
    """
    subprocess.check_call(compile_command)
    return os.stat("rclone").st_size
        
def main():
    # change directory to the one with this script in
    os.chdir(os.path.dirname(os.path.abspath(__file__)))
    # change directory to the main rclone source
    os.chdir("..")

    to_test = sys.argv[1:]
    backends, orig_all = read_backends()
    if len(to_test) == 0:
        to_test = backends
    # Compile with all backends
    ref = compile()
    print(f"Total binary size {ref/1024/1024:.3f} MiB")
    print("Backend,Size MiB")
    for test_backend in to_test:
        write_all(orig_all, test_backend)
        new_size = compile()
        print(f"{test_backend},{(ref-new_size)/1024/1024:.3f}")
    # restore all file
    with open(all_backends, "w") as fd:
        fd.write(orig_all)

if __name__ == "__main__":
    main()
