This directory contains scripts to start and stop servers for testing.

The commands are named after the remotes in use. They are executable
files with the following parameters:

    start  - starts the server if not running
    stop   - stops the server if nothing is using it
    status - returns non-zero exit code if the server is not running
    reset  - stops the server and resets any reference counts

These will be called automatically by test_all if that remote is
required.

When start is run it should output config parameters for that remote.
If a `_connect` parameter is output then that will be used for a
connection test. For example if `_connect=127.0.0.1:80` then a TCP
connection will be made to `127.0.0.1:80` and only when that succeeds
will the test continue.

If in addition to `_connect`, `_connect_delay=5s` is also present then
after the connection succeeds rclone will wait `5s` before continuing.
This is for servers that aren't quite ready even though they have
opened their TCP ports.

## Writing new scripts

A docker based server or an `rclone serve` based server should be easy
to write. Look at one of the examples.

`run.bash` contains boilerplate to be included in a bash script for
interpreting the command line parameters. This does reference counting
to ensure multiple copies of the server aren't running at once.
Including this is mandatory. It will call your `start()`, `stop()` and
`status()` functions.

`docker.bash` contains library functions to help with docker
implementations. It contains implementations of `stop()` and
`status()` so all you have to do is write a `start()` function.

`rclone-serve.bash` contains functions to help with `rclone serve`
based implementations. It contains implementations of `stop()` and
`status()` so all you have to do is write a `start()` function which
should call the `run()` function provided.

Any external TCP or UDP ports used should be unique as any of the
servers might be running together. So please create a new line in the
[PORTS](PORTS.md) file to allocate your server a port. Bind any ports
to localhost so they aren't accessible externally.
