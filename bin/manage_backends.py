#!/usr/bin/env python3
"""
Manage the backend yaml files in docs/data/backends

usage: manage_backends.py [-h] {create,features,update,help} [files ...]

Manage rclone backend YAML files.

positional arguments:
  {create,features,update,help}
                        Action to perform
  files                 List of YAML files to operate on

options:
  -h, --help            show this help message and exit
"""
import argparse
import sys
import os
import yaml
import json
import subprocess
import time
import socket
from contextlib import contextmanager
from pprint import pprint

# --- Configuration ---

# The order in which keys should appear in the YAML file
CANONICAL_ORDER = [
    "backend",
    "name",
    "tier",
    "maintainers",
    "features_score",
    "integration_tests",
    "data_integrity",
    "performance",
    "adoption",
    "docs",
    "security",
    "virtual",
    "remote",
    "features",
    "hashes",
    "precision"
]

# Default values for fields when creating/updating
DEFAULTS = {
    "backend": None,
    "name": None,
    "tier": "Tier 4",
    "maintainers": "External",
    "features_score": 0,
    "integration_tests": "Passing",
    "data_integrity": "Other",
    "performance": "High",
    "adoption": "Some use",
    "docs": "Full",
    "security": "High",
    "virtual": False,
    "remote": "TestBackend:",
    "features": [],
    "hashes": [],
    "precision": None
}

# --- Test server management ---

def wait_for_tcp(address_str, delay=1, timeout=2, tries=60):
    """
    Blocks until the specified TCP address (e.g., '172.17.0.3:21') is reachable.
    """
    host, port = address_str.split(":")
    port = int(port)
    print(f"Waiting for {host}:{port}...")
    for tri in range(tries):
        with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
            sock.settimeout(timeout)
            result = sock.connect_ex((host, port))
            if result == 0:
                print(f"Connected to {host}:{port} successfully!")
                break
            else:
                print(f"Failed to connect to {host}:{port} try {tri} !")
                time.sleep(delay)

def parse_init_output(binary_input):
    """
    Parse the output of the init script
    """
    decoded_str = binary_input.decode('utf-8')
    result = {}
    for line in decoded_str.splitlines():
        if '=' in line:
            key, value = line.split('=', 1)
            result[key.strip()] = value.strip()
    return result
                
@contextmanager
def test_server(remote):
    """Start the test server for remote if needed"""
    remote_name = remote.split(":",1)[0]
    init_script = "fstest/testserver/init.d/" + remote_name
    if not os.path.isfile(init_script):
        yield
        return
    print(f"--- Starting {init_script} ---")
    out = subprocess.check_output([init_script, "start"])
    out = parse_init_output(out)
    pprint(out)
    # Configure the server with environment variables
    env_keys = []
    for key, value in out.items():
        env_key = f"RCLONE_CONFIG_{remote_name.upper()}_{key.upper()}"
        env_keys.append(env_key)
        os.environ[env_key] = value
    for key,var in os.environ.items():
        if key.startswith("RCLON"):
            print(key, var)
    if "_connect" in out:
        wait_for_tcp(out["_connect"])
    try:
        yield
    finally:
        print(f"--- Stopping {init_script} ---")
        subprocess.run([init_script, "stop"], check=True)
        # Remove the env vars
        for env_key in env_keys:
            del os.environ[env_key]

# --- Helper Functions ---

def load_yaml(filepath):
    if not os.path.exists(filepath):
        return {}
    with open(filepath, 'r', encoding='utf-8') as f:
        return yaml.safe_load(f) or {}

def save_yaml(filepath, data):
    # Reconstruct dictionary in canonical order
    ordered_data = {}
    
    # Add known keys in order
    for key in CANONICAL_ORDER:
        if key in data:
            ordered_data[key] = data[key]
    
    # Add any other keys that might exist (custom fields)
    for key in data:
        if key not in CANONICAL_ORDER:
            ordered_data[key] = data[key]

    # Ensure features are a sorted list (if present)
    if 'features' in ordered_data and isinstance(ordered_data['features'], list):
        ordered_data['features'].sort()

    with open(filepath, 'w', encoding='utf-8') as f:
        yaml.dump(ordered_data, f, default_flow_style=False, sort_keys=False, allow_unicode=True)
    print(f"Saved {filepath}")

def get_backend_name_from_file(filepath):
    """
    s3.yaml -> S3
    azureblob.yaml -> Azureblob
    """
    basename = os.path.basename(filepath)
    name, _ = os.path.splitext(basename)
    return name.title()

def fetch_rclone_features(remote_str):
    """
    Runs `rclone backend features remote:` and returns the JSON object.
    """
    cmd = ["rclone", "backend", "features", remote_str]
    try:
        with test_server(remote_str):
            result = subprocess.run(cmd, capture_output=True, text=True, check=True)
        return json.loads(result.stdout)
    except subprocess.CalledProcessError as e:
        print(f"Error running rclone: {e.stderr}")
        return None
    except FileNotFoundError:
        print("Error: 'rclone' command not found in PATH.")
        sys.exit(1)

# --- Verbs ---

def do_create(files):
    for filepath in files:
        if os.path.exists(filepath):
            print(f"Skipping {filepath} (already exists)")
            continue
        
        data = DEFAULTS.copy()
        # Set a default name based on filename
        data['backend'] = get_backend_name_from_file(filepath)
        data['name'] = data['backend'].title()
        data['remote'] = "Test" + data['name'] + ":"
        save_yaml(filepath, data)

def do_update(files):
    for filepath in files:
        if not os.path.exists(filepath):
            print(f"Warning: {filepath} does not exist. Use 'create' first.")
            continue

        data = load_yaml(filepath)
        modified = False

        # Inject the filename as the 'backend'
        file_backend = os.path.splitext(os.path.basename(filepath))[0]
        
        if data.get('backend') != file_backend:
            data['backend'] = file_backend
            modified = True
            print(f"[{filepath}] Updated backend to: {file_backend}")

        # Add missing default fields
        for key, default_val in DEFAULTS.items():
            if key not in data:
                data[key] = default_val
                modified = True
                print(f"[{filepath}] Added missing field: {key}")

        # Special handling for 'name' if it was just added as None or didn't exist
        if data.get('name') is None:
            data['name'] = get_backend_name_from_file(filepath)
            modified = True
            print(f"[{filepath}] Set default name: {data['name']}")

        if modified:
            save_yaml(filepath, data)
        else:
            # We save anyway to enforce canonical order if the file was messy
            save_yaml(filepath, data)

def do_features(files):
    for filepath in files:
        if not os.path.exists(filepath):
            print(f"Error: {filepath} not found.")
            continue

        data = load_yaml(filepath)
        remote = data.get('remote')

        if not remote:
            print(f"Error: [{filepath}] 'remote' field is missing or empty. Cannot fetch features.")
            continue

        print(f"[{filepath}] Fetching features for remote: '{remote}'...")
        rclone_data = fetch_rclone_features(remote)

        if not rclone_data:
            print(f"Failed to fetch data for {filepath}")
            continue

        # Process Features (Dict -> Sorted List of True keys)
        features_dict = rclone_data.get('Features', {})
        # Filter only true values and sort keys
        feature_list = sorted([k for k, v in features_dict.items() if v])
        
        # Process Hashes
        hashes_list = rclone_data.get('Hashes', [])
        
        # Process Precision
        precision = rclone_data.get('Precision')

        # Update data
        data['features'] = feature_list
        data['hashes'] = hashes_list
        data['precision'] = precision

        save_yaml(filepath, data)

# --- Main CLI ---

def main():
    parser = argparse.ArgumentParser(description="Manage rclone backend YAML files.")
    parser.add_argument("verb", choices=["create", "features", "update", "help"], help="Action to perform")
    parser.add_argument("files", nargs="*", help="List of YAML files to operate on")

    args = parser.parse_args()

    if args.verb == "help":
        parser.print_help()
        sys.exit(0)

    if not args.files:
        print("Error: No files specified.")
        parser.print_help()
        sys.exit(1)

    if args.verb == "create":
        do_create(args.files)
    elif args.verb == "update":
        do_update(args.files)
    elif args.verb == "features":
        do_features(args.files)

if __name__ == "__main__":
    main()
