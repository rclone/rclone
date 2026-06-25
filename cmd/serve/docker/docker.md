This command implements the Docker volume plugin API allowing docker to use
rclone as a data storage mechanism for various cloud providers.
rclone provides [docker volume plugin](/docker) based on it.

To create a docker plugin, one must create a Unix or TCP socket that Docker
will look for when you use the plugin and then it listens for commands from
docker daemon and runs the corresponding code when necessary.
Docker plugins can run as a managed plugin under control of the docker daemon
or as an independent native service. For testing, you can just run it directly
from the command line, for example:

```console
sudo rclone serve docker --base-dir /tmp/rclone-volumes --socket-addr localhost:8787 -vv
```

Running `rclone serve docker` will create the said socket, listening for
commands from Docker to create the necessary Volumes. Normally you need not
give the `--socket-addr` flag. The API will listen on the unix domain socket
at `/run/docker/plugins/rclone.sock`. In the example above rclone will create
a TCP socket and a small file `/etc/docker/plugins/rclone.spec` containing
the socket address. We use `sudo` because both paths are writeable only by
the root user.

If you later decide to change listening socket, the docker daemon must be
restarted to reconnect to `/run/docker/plugins/rclone.sock`
or parse new `/etc/docker/plugins/rclone.spec`. Until you restart, any
volume related docker commands will timeout trying to access the old socket.
Running directly is supported on **Linux only**, not on Windows or MacOS.
This is not a problem with managed plugin mode described in details
in the [full documentation](https://rclone.org/docker).

The command will create volume mounts under the path given by `--base-dir`
(by default `/var/lib/docker-volumes/rclone` available only to root)
and maintain the JSON formatted file `docker-plugin.state` in the rclone cache
directory with book-keeping records of created and mounted volumes.

All mount and VFS options are submitted by the docker daemon via API, but
you can also provide defaults on the command line as well as set path to the
config file and cache directory or adjust logging verbosity.

## Security

The plugin API accepts a `remote` (aka `fs`) option on volume creation, and this
is parsed exactly like an rclone connection string. Connection strings are
trusted configuration: they may carry inline backend options, and some backends
use those options to run local commands (for example the `sftp` backend's `ssh`
option spawns an external binary). Anyone who can send requests to the plugin
socket can therefore make rclone run arbitrary commands as the user running
`rclone serve docker` (typically root). Treat access to the socket as equivalent
to that level of access and only expose it to trusted callers.

When listening on the default unix socket at `/run/docker/plugins/rclone.sock`
rclone creates it with mode `0660` owned by `root` and the group given by
`--socket-gid` (the process GID by default), so only root and members of that
group - normally just the docker daemon - can reach it. Do not loosen these
permissions or hand the group to untrusted users.

When using `--socket-addr` to listen on a TCP socket there is no authentication
and the API is reachable by anyone who can open the port, so bind it to a
loopback or otherwise trusted address and protect it with a firewall. Note that
holding Docker daemon access is already equivalent to root on the host, so a
caller able to issue `docker volume create` does not gain anything new from
this; the concern is exposing the socket more widely than the daemon itself.
