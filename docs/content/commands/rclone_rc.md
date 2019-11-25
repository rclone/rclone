---
date: 2019-10-26T11:04:03+01:00
title: "rclone rc"
slug: rclone_rc
url: /commands/rclone_rc/
---
## rclone rc

Run a command against a running rclone.

### Synopsis



This runs a command against a running rclone.  Use the --url flag to
specify an non default URL to connect on.  This can be either a
":port" which is taken to mean "http://localhost:port" or a
"host:port" which is taken to mean "http://host:port"

A username and password can be passed in with --user and --pass.

Note that --rc-addr, --rc-user, --rc-pass will be read also for --url,
--user, --pass.

Arguments should be passed in as parameter=value.

The result will be returned as a JSON object by default.

The --json parameter can be used to pass in a JSON blob as an input
instead of key=value arguments.  This is the only way of passing in
more complicated values.

Use --loopback to connect to the rclone instance running "rclone rc".
This is very useful for testing commands without having to run an
rclone rc server, eg:

    rclone rc --loopback operations/about fs=/

Use "rclone rc" to see a list of all possible commands.

```
rclone rc commands parameter [flags]
```

### Options

```
  -h, --help          help for rc
      --json string   Input JSON - use instead of key=value args.
      --loopback      If set connect to this rclone instance not via HTTP.
      --no-output     If set don't output the JSON result.
      --pass string   Password to use to connect to rclone remote control.
      --url string    URL to connect to rclone remote control. (default "http://localhost:5572/")
      --user string   Username to use to rclone remote control.
```

See the [global flags page](/flags/) for global options not listed here.

### SEE ALSO

* [rclone](/commands/rclone/)	 - Show help for rclone commands, flags and backends.

