#!/usr/bin/env python3
"""
Test program to demonstrate the remote config interfaces in
rclone.

This program can simulate

    rclone config create
    rclone config update
    rclone config password - NOT implemented yet
    rclone authorize       - NOT implemented yet

Pass the desired action as the first argument then any parameters.

This assumes passwords will be passed in the clear.
"""

import argparse
import subprocess
import json
from pprint import pprint

sep = "-"*60

def rpc(args, command, params):
    """
    Run the command. This could be either over the CLI or the API.

    Here we run over the API either using `rclone rc --loopback` which
    is useful for making sure state is saved properly or to an
    existing rclone rcd if `--rc` is used on the command line.
    """
    if args.rc:
        import requests
        kwargs = {
            "json": params,
        }
        if args.user:
            kwargs["auth"] = (args.user, args.password)
        r = requests.post('http://localhost:5572/'+command, **kwargs)
        if r.status_code != 200:
            raise ValueError(f"RC command failed: Error {r.status_code}: {r.text}")
        return r.json()
    cmd = ["rclone", "-vv", "rc", "--loopback", command, "--json", json.dumps(params)]
    result = subprocess.run(cmd, stdout=subprocess.PIPE, check=True)
    return json.loads(result.stdout)

def parse_parameters(parameters):
    """
    Parse the incoming key=value parameters into a dict
    """
    d = {}
    for param in parameters:
        parts = param.split("=", 1)
        if len(parts) != 2:
            raise ValueError("bad format for parameter need name=value")
        d[parts[0]] = parts[1]
    return d

def ask(opt):
    """
    Ask the user to enter the option

    This is the user interface for asking a user a question.

    If there are examples they should be presented.
    """
    while True:
        if opt["IsPassword"]:
            print("*** Inputting a password")
        print(opt['Help'])
        examples = opt.get("Examples", ())
        or_number = ""
        if len(examples) > 0:
            or_number = " or choice number"
            for i, example in enumerate(examples):
                print(f"{i:3} value: {example['Value']}")
                print(f"    help:  {example['Help']}")
        print(f"Enter a {opt['Type']} value{or_number}. Press Enter for the default ('{opt['DefaultStr']}')")
        print(f"{opt['Name']}> ", end='')
        s = input()
        if s == "":
            return opt["DefaultStr"]
        try:
            i = int(s)
            if i >= 0 and i < len(examples):
                return examples[i]["Value"]
        except ValueError:
            pass
        if opt["Exclusive"]:
            for example in examples:
                if s == example["Value"]:
                    return s
            # Exclusive is set but the value isn't one of the accepted
            # ones so continue
            print("Value isn't one of the acceptable values")
        else:
            return s
    return s

def create_or_update(what, args):
    """
    Run the equivalent of rclone config create
    or rclone config update

    what should either be "create" or "update
    """
    print(what, args)
    params = parse_parameters(args.parameters)
    inp = {
        "name": args.name,
        "parameters": params,
        "opt": {
            "nonInteractive": True,
            "all": args.all,
            "noObscure": args.obscured_passwords,
            "obscure": not args.obscured_passwords,
        },
    }
    if what == "create":
        inp["type"] = args.type
    while True:
        print(sep)
        print("Input to API")
        pprint(inp)
        print(sep)
        out = rpc(args, "config/"+what, inp)
        print(sep)
        print("Output from API")
        pprint(out)
        print(sep)
        if out["State"] == "":
            return
        if out["Error"]:
                print("Error", out["Error"])
        result = ask(out["Option"])
        inp["opt"]["state"] = out["State"]
        inp["opt"]["result"] = result
        inp["opt"]["continue"] = True

def create(args):
    """Run the equivalent of rclone config create"""
    create_or_update("create", args)

def update(args):
    """Run the equivalent of rclone config update"""
    create_or_update("update", args)

def password(args):
    """Run the equivalent of rclone config password"""
    print("password", args)
    raise NotImplementedError()

def authorize(args):
    """Run the equivalent of rclone authorize"""
    print("authorize", args)
    raise NotImplementedError()

def main():
    """
    Make the command line parser and dispatch
    """
    parser = argparse.ArgumentParser(
        description=__doc__,
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    parser.add_argument("-a", "--all", action='store_true',
                        help="Ask all the config questions if set")
    parser.add_argument("-o", "--obscured-passwords", action='store_true',
                        help="If set assume the passwords are obscured")
    parser.add_argument("--rc", action='store_true',
                        help="If set use the rc (you'll need to start an rclone rcd)")
    parser.add_argument("--user", type=str, default="",
                        help="Username for use with --rc")
    parser.add_argument("--pass", type=str, default="", dest='password',
                        help="Password for use with --rc")

    subparsers = parser.add_subparsers(dest='command', required=True)
    
    subparser = subparsers.add_parser('create')
    subparser.add_argument("name", type=str, help="Name of remote to create")
    subparser.add_argument("type", type=str, help="Type of remote to create")
    subparser.add_argument("parameters", type=str, nargs='*', help="Config parameters name=value name=value")
    subparser.set_defaults(func=create)

    subparser = subparsers.add_parser('update')
    subparser.add_argument("name", type=str, help="Name of remote to update")
    subparser.add_argument("parameters", type=str, nargs='*', help="Config parameters name=value name=value")
    subparser.set_defaults(func=update)

    subparser = subparsers.add_parser('password')
    subparser.add_argument("name", type=str, help="Name of remote to update")
    subparser.add_argument("parameters", type=str, nargs='*', help="Config parameters name=value name=value")
    subparser.set_defaults(func=password)

    subparser = subparsers.add_parser('authorize')
    subparser.set_defaults(func=authorize)
    
    args = parser.parse_args()
    args.func(args)

if __name__ == "__main__":
    main()
