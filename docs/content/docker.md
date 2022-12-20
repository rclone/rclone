---
title: "Docker Volume Plugin"
description: "Docker Volume Plugin"
versionIntroduced: "v1.56"
---

# Docker Volume Plugin

## Introduction

Docker 1.9 has added support for creating
[named volumes](https://docs.docker.com/storage/volumes/) via
[command-line interface](https://docs.docker.com/engine/reference/commandline/volume_create/)
and mounting them in containers as a way to share data between them.
Since Docker 1.10 you can create named volumes with
[Docker Compose](https://docs.docker.com/compose/) by descriptions in
[docker-compose.yml](https://docs.docker.com/compose/compose-file/compose-file-v2/#volume-configuration-reference)
files for use by container groups on a single host.
As of Docker 1.12 volumes are supported by
[Docker Swarm](https://docs.docker.com/engine/swarm/key-concepts/)
included with Docker Engine and created from descriptions in
[swarm compose v3](https://docs.docker.com/compose/compose-file/compose-file-v3/#volume-configuration-reference)
files for use with _swarm stacks_ across multiple cluster nodes.

[Docker Volume Plugins](https://docs.docker.com/engine/extend/plugins_volume/)
augment the default `local` volume driver included in Docker with stateful
volumes shared across containers and hosts. Unlike local volumes, your
data will _not_ be deleted when such volume is removed. Plugins can run
managed by the docker daemon, as a native system service
(under systemd, _sysv_ or _upstart_) or as a standalone executable.
Rclone can run as docker volume plugin in all these modes.
It interacts with the local docker daemon
via [plugin API](https://docs.docker.com/engine/extend/plugin_api/) and
handles mounting of remote file systems into docker containers so it must
run on the same host as the docker daemon or on every Swarm node.

## Getting started

In the first example we will use the [SFTP](/sftp/)
rclone volume with Docker engine on a standalone Ubuntu machine.

Start from [installing Docker](https://docs.docker.com/engine/install/)
on the host.

The _FUSE_ driver is a prerequisite for rclone mounting and should be
installed on host:
```
sudo apt-get -y install fuse
```

Create two directories required by rclone docker plugin:
```
sudo mkdir -p /var/lib/docker-plugins/rclone/config
sudo mkdir -p /var/lib/docker-plugins/rclone/cache
```

Install the managed rclone docker plugin for your architecture (here `amd64`):
```
docker plugin install rclone/docker-volume-rclone:amd64 args="-v" --alias rclone --grant-all-permissions
docker plugin list
```

Create your [SFTP volume](/sftp/#standard-options):
```
docker volume create firstvolume -d rclone -o type=sftp -o sftp-host=_hostname_ -o sftp-user=_username_ -o sftp-pass=_password_ -o allow-other=true
```

Note that since all options are static, you don't even have to run
`rclone config` or create the `rclone.conf` file (but the `config` directory
should still be present). In the simplest case you can use `localhost`
as _hostname_ and your SSH credentials as _username_ and _password_.
You can also change the remote path to your home directory on the host,
for example `-o path=/home/username`.


Time to create a test container and mount the volume into it:
```
docker run --rm -it -v firstvolume:/mnt --workdir /mnt ubuntu:latest bash
```

If all goes well, you will enter the new container and change right to
the mounted SFTP remote. You can type `ls` to list the mounted directory
or otherwise play with it. Type `exit` when you are done.
The container will stop but the volume will stay, ready to be reused.
When it's not needed anymore, remove it:
```
docker volume list
docker volume remove firstvolume
```

Now let us try **something more elaborate**:
[Google Drive](/drive/) volume on multi-node Docker Swarm.

You should start from installing Docker and FUSE, creating plugin
directories and installing rclone plugin on _every_ swarm node.
Then [setup the Swarm](https://docs.docker.com/engine/swarm/swarm-mode/).

Google Drive volumes need an access token which can be setup via web
browser and will be periodically renewed by rclone. The managed
plugin cannot run a browser so we will use a technique similar to the
[rclone setup on a headless box](/remote_setup/).

Run [rclone config](/commands/rclone_config_create/)
on _another_ machine equipped with _web browser_ and graphical user interface.
Create the [Google Drive remote](/drive/#standard-options).
When done, transfer the resulting `rclone.conf` to the Swarm cluster
and save as `/var/lib/docker-plugins/rclone/config/rclone.conf`
on _every_ node. By default this location is accessible only to the
root user so you will need appropriate privileges. The resulting config
will look like this:
```
[gdrive]
type = drive
scope = drive
drive_id = 1234567...
root_folder_id = 0Abcd...
token = {"access_token":...}
```

Now create the file named `example.yml` with a swarm stack description
like this:
```
version: '3'
services:
  heimdall:
    image: linuxserver/heimdall:latest
    ports: [8080:80]
    volumes: [configdata:/config]
volumes:
  configdata:
    driver: rclone
    driver_opts:
      remote: 'gdrive:heimdall'
      allow_other: 'true'
      vfs_cache_mode: full
      poll_interval: 0
```

and run the stack:
```
docker stack deploy example -c ./example.yml
```

After a few seconds docker will spread the parsed stack description
over cluster, create the `example_heimdall` service on port _8080_,
run service containers on one or more cluster nodes and request
the `example_configdata` volume from rclone plugins on the node hosts.
You can use the following commands to confirm results:
```
docker service ls
docker service ps example_heimdall
docker volume ls
```

Point your browser to `http://cluster.host.address:8080` and play with
the service. Stop it with `docker stack remove example` when you are done.
Note that the `example_configdata` volume(s) created on demand at the
cluster nodes will not be automatically removed together with the stack
but stay for future reuse. You can remove them manually by invoking
the `docker volume remove example_configdata` command on every node.

## Creating Volumes via CLI

Volumes can be created with [docker volume create](https://docs.docker.com/engine/reference/commandline/volume_create/).
Here are a few examples:
```
docker volume create vol1 -d rclone -o remote=storj: -o vfs-cache-mode=full
docker volume create vol2 -d rclone -o remote=:storj,access_grant=xxx:heimdall
docker volume create vol3 -d rclone -o type=storj -o path=heimdall -o storj-access-grant=xxx -o poll-interval=0
```

Note the `-d rclone` flag that tells docker to request volume from the
rclone driver. This works even if you installed managed driver by its full
name `rclone/docker-volume-rclone` because you provided the `--alias rclone`
option.

Volumes can be inspected as follows:
```
docker volume list
docker volume inspect vol1
```

## Volume Configuration

Rclone flags and volume options are set via the `-o` flag to the
`docker volume create` command. They include backend-specific parameters
as well as mount and _VFS_ options. Also there are a few
special `-o` options:
`remote`, `fs`, `type`, `path`, `mount-type` and `persist`.

`remote` determines an existing remote name from the config file, with
trailing colon and optionally with a remote path. See the full syntax in
the [rclone documentation](/docs/#syntax-of-remote-paths).
This option can be aliased as `fs` to prevent confusion with the
_remote_ parameter of such backends as _crypt_ or _alias_.

The `remote=:backend:dir/subdir` syntax can be used to create
[on-the-fly (config-less) remotes](/docs/#backend-path-to-dir),
while the `type` and `path` options provide a simpler alternative for this.
Using two split options
```
-o type=backend -o path=dir/subdir
```
is equivalent to the combined syntax
```
-o remote=:backend:dir/subdir
```
but is arguably easier to parameterize in scripts.
The `path` part is optional.

[Mount and VFS options](/commands/rclone_serve_docker/#options)
as well as [backend parameters](/flags/#backend-flags) are named
like their twin command-line flags without the `--` CLI prefix.
Optionally you can use underscores instead of dashes in option names.
For example, `--vfs-cache-mode full` becomes
`-o vfs-cache-mode=full` or `-o vfs_cache_mode=full`.
Boolean CLI flags without value will gain the `true` value, e.g.
`--allow-other` becomes `-o allow-other=true` or `-o allow_other=true`.

Please note that you can provide parameters only for the backend immediately
referenced by the backend type of mounted `remote`.
If this is a wrapping backend like _alias, chunker or crypt_, you cannot
provide options for the referred to remote or backend. This limitation is
imposed by the rclone connection string parser. The only workaround is to
feed plugin with `rclone.conf` or configure plugin arguments (see below).

## Special Volume Options

`mount-type` determines the mount method and in general can be one of:
`mount`, `cmount`, or `mount2`. This can be aliased as `mount_type`.
It should be noted that the managed rclone docker plugin currently does
not support the `cmount` method and `mount2` is rarely needed.
This option defaults to the first found method, which is usually `mount`
so you generally won't need it.

`persist` is a reserved boolean (true/false) option.
In future it will allow to persist on-the-fly remotes in the plugin
`rclone.conf` file.

## Connection Strings

The `remote` value can be extended
with [connection strings](/docs/#connection-strings)
as an alternative way to supply backend parameters. This is equivalent
to the `-o` backend options with one _syntactic difference_.
Inside connection string the backend prefix must be dropped from parameter
names but in the `-o param=value` array it must be present.
For instance, compare the following option array
```
-o remote=:sftp:/home -o sftp-host=localhost
```
with equivalent connection string:
```
-o remote=:sftp,host=localhost:/home
```
This difference exists because flag options `-o key=val` include not only
backend parameters but also mount/VFS flags and possibly other settings.
Also it allows to discriminate the `remote` option from the `crypt-remote`
(or similarly named backend parameters) and arguably simplifies scripting
due to clearer value substitution.

## Using with Swarm or Compose

Both _Docker Swarm_ and _Docker Compose_ use
[YAML](http://yaml.org/spec/1.2/spec.html)-formatted text files to describe
groups (stacks) of containers, their properties, networks and volumes.
_Compose_ uses the [compose v2](https://docs.docker.com/compose/compose-file/compose-file-v2/#volume-configuration-reference) format,
_Swarm_ uses the [compose v3](https://docs.docker.com/compose/compose-file/compose-file-v3/#volume-configuration-reference) format.
They are mostly similar, differences are explained in the
[docker documentation](https://docs.docker.com/compose/compose-file/compose-versioning/#upgrading).

Volumes are described by the children of the top-level `volumes:` node.
Each of them should be named after its volume and have at least two
elements, the self-explanatory `driver: rclone` value and the
`driver_opts:` structure playing the same role as `-o key=val` CLI flags:

```
volumes:
  volume_name_1:
    driver: rclone
    driver_opts:
      remote: 'gdrive:'
      allow_other: 'true'
      vfs_cache_mode: full
      token: '{"type": "borrower", "expires": "2021-12-31"}'
      poll_interval: 0
```

Notice a few important details:
- YAML prefers `_` in option names instead of `-`.
- YAML treats single and double quotes interchangeably.
  Simple strings and integers can be left unquoted.
- Boolean values must be quoted like `'true'` or `"false"` because
  these two words are reserved by YAML.
- The filesystem string is keyed with `remote` (or with `fs`).
  Normally you can omit quotes here, but if the string ends with colon,
  you **must** quote it like `remote: "storage_box:"`.
- YAML is picky about surrounding braces in values as this is in fact
  another [syntax for key/value mappings](http://yaml.org/spec/1.2/spec.html#id2790832).
  For example, JSON access tokens usually contain double quotes and
  surrounding braces, so you must put them in single quotes.

## Installing as Managed Plugin

Docker daemon can install plugins from an image registry and run them managed.
We maintain the
[docker-volume-rclone](https://hub.docker.com/p/rclone/docker-volume-rclone/)
plugin image on [Docker Hub](https://hub.docker.com).

Rclone volume plugin requires **Docker Engine >= 19.03.15**

The plugin requires presence of two directories on the host before it can
be installed. Note that plugin will **not** create them automatically.
By default they must exist on host at the following locations
(though you can tweak the paths):
- `/var/lib/docker-plugins/rclone/config`
  is reserved for the `rclone.conf` config file and **must** exist
  even if it's empty and the config file is not present.
- `/var/lib/docker-plugins/rclone/cache`
  holds the plugin state file as well as optional VFS caches.

You can [install managed plugin](https://docs.docker.com/engine/reference/commandline/plugin_install/)
with default settings as follows:
```
docker plugin install rclone/docker-volume-rclone:amd64 --grant-all-permissions --alias rclone
```

The `:amd64` part of the image specification after colon is called a _tag_.
Usually you will want to install the latest plugin for your architecture. In
this case the tag will just name it, like `amd64` above. The following plugin
architectures are currently available:
- `amd64`
- `arm64`
- `arm-v7`

Sometimes you might want a concrete plugin version, not the latest one.
Then you should use image tag in the form `:ARCHITECTURE-VERSION`.
For example, to install plugin version `v1.56.2` on architecture `arm64`
you will use tag `arm64-1.56.2` (note the removed `v`) so the full image
specification becomes `rclone/docker-volume-rclone:arm64-1.56.2`.

We also provide the `latest` plugin tag, but since docker does not support
multi-architecture plugins as of the time of this writing, this tag is
currently an **alias for `amd64`**.
By convention the `latest` tag is the default one and can be omitted, thus
both `rclone/docker-volume-rclone:latest` and just `rclone/docker-volume-rclone`
will refer to the latest plugin release for the `amd64` platform.

Also the `amd64` part can be omitted from the versioned rclone plugin tags.
For example, rclone image reference `rclone/docker-volume-rclone:amd64-1.56.2`
can be abbreviated as `rclone/docker-volume-rclone:1.56.2` for convenience.
However, for non-intel architectures you still have to use the full tag as
`amd64` or `latest` will fail to start.

Managed plugin is in fact a special container running in a namespace separate
from normal docker containers. Inside it runs the `rclone serve docker`
command. The config and cache directories are bind-mounted into the
container at start. The docker daemon connects to a unix socket created
by the command inside the container. The command creates on-demand remote
mounts right inside, then docker machinery propagates them through kernel
mount namespaces and bind-mounts into requesting user containers.

You can tweak a few plugin settings after installation when it's disabled
(not in use), for instance:
```
docker plugin disable rclone
docker plugin set rclone RCLONE_VERBOSE=2 config=/etc/rclone args="--vfs-cache-mode=writes --allow-other"
docker plugin enable rclone
docker plugin inspect rclone
```

Note that if docker refuses to disable the plugin, you should find and
remove all active volumes connected with it as well as containers and
swarm services that use them. This is rather tedious so please carefully
plan in advance.

You can tweak the following settings:
`args`, `config`, `cache`, `HTTP_PROXY`, `HTTPS_PROXY`, `NO_PROXY`
and `RCLONE_VERBOSE`.
It's _your_ task to keep plugin settings in sync across swarm cluster nodes.

`args` sets command-line arguments for the `rclone serve docker` command
(_none_ by default). Arguments should be separated by space so you will
normally want to put them in quotes on the
[docker plugin set](https://docs.docker.com/engine/reference/commandline/plugin_set/)
command line. Both [serve docker flags](/commands/rclone_serve_docker/#options)
and [generic rclone flags](/flags/) are supported, including backend
parameters that will be used as defaults for volume creation.
Note that plugin will fail (due to [this docker bug](https://github.com/moby/moby/blob/v20.10.7/plugin/v2/plugin.go#L195))
if the `args` value is empty. Use e.g. `args="-v"` as a workaround.

`config=/host/dir` sets alternative host location for the config directory.
Plugin will look for `rclone.conf` here. It's not an error if the config
file is not present but the directory must exist. Please note that plugin
can periodically rewrite the config file, for example when it renews
storage access tokens. Keep this in mind and try to avoid races between
the plugin and other instances of rclone on the host that might try to
change the config simultaneously resulting in corrupted `rclone.conf`.
You can also put stuff like private key files for SFTP remotes in this
directory. Just note that it's bind-mounted inside the plugin container
at the predefined path `/data/config`. For example, if your key file is
named `sftp-box1.key` on the host, the corresponding volume config option
should read `-o sftp-key-file=/data/config/sftp-box1.key`.

`cache=/host/dir` sets alternative host location for the _cache_ directory.
The plugin will keep VFS caches here. Also it will create and maintain
the `docker-plugin.state` file in this directory. When the plugin is
restarted or reinstalled, it will look in this file to recreate any volumes
that existed previously. However, they will not be re-mounted into
consuming containers after restart. Usually this is not a problem as
the docker daemon normally will restart affected user containers after
failures, daemon restarts or host reboots.

`RCLONE_VERBOSE` sets plugin verbosity from `0` (errors only, by default)
to `2` (debugging). Verbosity can be also tweaked via `args="-v [-v] ..."`.
Since arguments are more generic, you will rarely need this setting.
The plugin output by default feeds the docker daemon log on local host.
Log entries are reflected as _errors_ in the docker log but retain their
actual level assigned by rclone in the encapsulated message string.

`HTTP_PROXY`, `HTTPS_PROXY`, `NO_PROXY` customize the plugin proxy settings.

You can set custom plugin options right when you install it, _in one go_:
```
docker plugin remove rclone
docker plugin install rclone/docker-volume-rclone:amd64 \
       --alias rclone --grant-all-permissions \
       args="-v --allow-other" config=/etc/rclone
docker plugin inspect rclone
```

## Healthchecks

The docker plugin volume protocol doesn't provide a way for plugins
to inform the docker daemon that a volume is (un-)available.
As a workaround you can setup a healthcheck to verify that the mount
is responding, for example:
```
services:
  my_service:
    image: my_image
    healthcheck:
      test: ls /path/to/rclone/mount || exit 1
      interval: 1m
      timeout: 15s
      retries: 3
      start_period: 15s
```

## Running Plugin under Systemd

In most cases you should prefer managed mode. Moreover, MacOS and Windows
do not support native Docker plugins. Please use managed mode on these
systems. Proceed further only if you are on Linux.

First, [install rclone](/install/).
You can just run it (type `rclone serve docker` and hit enter) for the test.

Install _FUSE_:
```
sudo apt-get -y install fuse
```

Download two systemd configuration files:
[docker-volume-rclone.service](https://raw.githubusercontent.com/rclone/rclone/master/contrib/docker-plugin/systemd/docker-volume-rclone.service)
and [docker-volume-rclone.socket](https://raw.githubusercontent.com/rclone/rclone/master/contrib/docker-plugin/systemd/docker-volume-rclone.socket).

Put them to the `/etc/systemd/system/` directory:
```
cp docker-volume-plugin.service /etc/systemd/system/
cp docker-volume-plugin.socket  /etc/systemd/system/
```

Please note that all commands in this section must be run as _root_ but
we omit `sudo` prefix for brevity.
Now create directories required by the service:
```
mkdir -p /var/lib/docker-volumes/rclone
mkdir -p /var/lib/docker-plugins/rclone/config
mkdir -p /var/lib/docker-plugins/rclone/cache
```

Run the docker plugin service in the socket activated mode:
```
systemctl daemon-reload
systemctl start docker-volume-rclone.service
systemctl enable docker-volume-rclone.socket
systemctl start docker-volume-rclone.socket
systemctl restart docker
```

Or run the service directly:
- run `systemctl daemon-reload` to let systemd pick up new config
- run `systemctl enable docker-volume-rclone.service` to make the new
  service start automatically when you power on your machine.
- run `systemctl start docker-volume-rclone.service`
  to start the service now.
- run `systemctl restart docker` to restart docker daemon and let it
  detect the new plugin socket. Note that this step is not needed in
  managed mode where docker knows about plugin state changes.

The two methods are equivalent from the user perspective, but I personally
prefer socket activation.

## Troubleshooting

You can [see managed plugin settings](https://docs.docker.com/engine/extend/#debugging-plugins)
with
```
docker plugin list
docker plugin inspect rclone
```
Note that docker (including latest 20.10.7) will not show actual values
of `args`, just the defaults.

Use `journalctl --unit docker` to see managed plugin output as part of
the docker daemon log. Note that docker reflects plugin lines as _errors_
but their actual level can be seen from encapsulated message string.

You will usually install the latest version of managed plugin for your platform.
Use the following commands to print the actual installed version:
```
PLUGID=$(docker plugin list --no-trunc | awk '/rclone/{print$1}')
sudo runc --root /run/docker/runtime-runc/plugins.moby exec $PLUGID rclone version
```

You can even use `runc` to run shell inside the plugin container:
```
sudo runc --root /run/docker/runtime-runc/plugins.moby exec --tty $PLUGID bash
```

Also you can use curl to check the plugin socket connectivity:
```
docker plugin list --no-trunc
PLUGID=123abc...
sudo curl -H Content-Type:application/json -XPOST -d {} --unix-socket /run/docker/plugins/$PLUGID/rclone.sock http://localhost/Plugin.Activate
```
though this is rarely needed.

## Caveats

Finally I'd like to mention a _caveat with updating volume settings_.
Docker CLI does not have a dedicated command like `docker volume update`.
It may be tempting to invoke `docker volume create` with updated options
on existing volume, but there is a gotcha. The command will do nothing,
it won't even return an error. I hope that docker maintainers will fix
this some day. In the meantime be aware that you must remove your volume
before recreating it with new settings:
```
docker volume remove my_vol
docker volume create my_vol -d rclone -o opt1=new_val1 ...
```

and verify that settings did update:
```
docker volume list
docker volume inspect my_vol
```

If docker refuses to remove the volume, you should find containers
or swarm services that use it and stop them first.
