---
title: "Remote Setup"
description: "Configuring rclone up on a remote / headless machine"
---

# Configuring rclone on a remote / headless machine

Some of the configurations (those involving oauth2) require an
internet-connected web browser.

If you are trying to set rclone up on a remote or headless machine with no
browser available on it (e.g. a NAS or a server in a datacenter), then
you will need to use an alternative means of configuration. There are
three ways of doing it, described below.

## Configuring using rclone authorize

On the headless machine run [rclone config](/commands/rclone_config), but
answer `N` to the question `Use web browser to automatically authenticate
rclone with remote?`.

```text
Use web browser to automatically authenticate rclone with remote?
 * Say Y if the machine running rclone has a web browser you can use
 * Say N if running rclone on a (remote) machine without web browser access
If not sure try Y. If Y failed, try N.

y) Yes (default)
n) No
y/n> n

Option config_token.
For this to work, you will need rclone available on a machine that has
a web browser available.
For more help and alternate methods see: https://rclone.org/remote_setup/
Execute the following on the machine with the web browser (same rclone
version recommended):
        rclone authorize "onedrive"
Then paste the result.
Enter a value.
config_token>
```

Then on your main desktop machine, run [rclone authorize](/commands/rclone_authorize/).

```text
rclone authorize "onedrive"
NOTICE: Make sure your Redirect URL is set to "http://localhost:53682/" in your custom config.
NOTICE: If your browser doesn't open automatically go to the following link: http://127.0.0.1:53682/auth?state=xxxxxxxxxxxxxxxxxxxxxx
NOTICE: Log in and authorize rclone for access
NOTICE: Waiting for code...

Got code
Paste the following into your remote machine --->
SECRET_TOKEN
<---End paste
```

Then back to the headless machine, paste in the code.

```text
config_token> SECRET_TOKEN
--------------------
[acd12]
client_id =
client_secret =
token = SECRET_TOKEN
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d>
```

## Configuring by copying the config file

Rclone stores all of its configuration in a single file. This can easily be
copied to configure a remote rclone (although some backends does not support
reusing the same configuration, consult your backend documentation to be
sure).

Start by running [rclone config](/commands/rclone_config) to create the
configuration file on your desktop machine.

```console
rclone config
```

Then locate the file by running [rclone config file](/commands/rclone_config_file).

```console
$ rclone config file
Configuration file is stored at:
/home/user/.rclone.conf
```

Finally, transfer the file to the remote machine (scp, cut paste, ftp, sftp, etc.)
and place it in the correct location (use [rclone config file](/commands/rclone_config_file)
on the remote machine to find out where).

## Configuring using SSH Tunnel

If you have an SSH client installed on your local machine, you can set up an
SSH tunnel to redirect the port 53682 into the headless machine by using the
following command:

```console
ssh -L localhost:53682:localhost:53682 username@remote_server
```

Then on the headless machine run [rclone config](/commands/rclone_config) and
answer `Y` to the question `Use web browser to automatically authenticate rclone
with remote?`.

```text
Use web browser to automatically authenticate rclone with remote?
 * Say Y if the machine running rclone has a web browser you can use
 * Say N if running rclone on a (remote) machine without web browser access
If not sure try Y. If Y failed, try N.

y) Yes (default)
n) No
y/n> y
NOTICE: Make sure your Redirect URL is set to "http://localhost:53682/" in your custom config.
NOTICE: If your browser doesn't open automatically go to the following link: http://127.0.0.1:53682/auth?state=xxxxxxxxxxxxxxxxxxxxxx
NOTICE: Log in and authorize rclone for access
NOTICE: Waiting for code...
```

Finally, copy and paste the presented URL `http://127.0.0.1:53682/auth?state=xxxxxxxxxxxxxxxxxxxxxx`
to the browser on your local machine, complete the auth and you are done.
