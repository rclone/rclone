---
title: "Remote Setup"
description: "Configuring rclone up on a remote / headless machine"
---

# Configuring rclone on a remote / headless machine

Some of the configurations (those involving oauth2) require an
Internet connected web browser.

If you are trying to set rclone up on a remote or headless box with no
browser available on it (e.g. a NAS or a server in a datacenter) then
you will need to use an alternative means of configuration.  There are
two ways of doing it, described below.

## Configuring using rclone authorize

On the headless box run `rclone` config but answer `N` to the `Use auto config?`
question.

```text
Use auto config?
 * Say Y if not sure
 * Say N if you are working on a remote or headless machine

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

Then on your main desktop machine

```text
rclone authorize "onedrive"
If your browser doesn't open automatically go to the following link: http://127.0.0.1:53682/auth
Log in and authorize rclone for access
Waiting for code...
Got code
Paste the following into your remote machine --->
SECRET_TOKEN
<---End paste
```

Then back to the headless box, paste in the code

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

Rclone stores all of its config in a single configuration file.  This
can easily be copied to configure a remote rclone.

So first configure rclone on your desktop machine with

```sh
rclone config
```

to set up the config file.

Find the config file by running `rclone config file`, for example

```sh
$ rclone config file
Configuration file is stored at:
/home/user/.rclone.conf
```

Now transfer it to the remote box (scp, cut paste, ftp, sftp, etc.) and
place it in the correct place (use `rclone config file` on the remote
box to find out where).

## Configuring using SSH Tunnel

Linux and MacOS users can utilize SSH Tunnel to redirect the headless box
port 53682 to local machine by using the following command:

```sh
ssh -L localhost:53682:localhost:53682 username@remote_server
```

Then on the headless box run `rclone config` and answer `Y` to the
`Use auto config?` question.

```text
Use auto config?
 * Say Y if not sure
 * Say N if you are working on a remote or headless machine

y) Yes (default)
n) No
y/n> y
```

Then copy and paste the auth url `http://127.0.0.1:53682/auth?state=xxxxxxxxxxxx`
to the browser on your local machine, complete the auth and it is done.
