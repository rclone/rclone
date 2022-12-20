---
title: "GUI"
description: "Web based Graphical User Interface"
versionIntroduced: "v1.49"
---

# GUI (Experimental)

Rclone can serve a web based GUI (graphical user interface).  This is
somewhat experimental at the moment so things may be subject to
change.

Run this command in a terminal and rclone will download and then
display the GUI in a web browser.

```
rclone rcd --rc-web-gui
```

This will produce logs like this and rclone needs to continue to run to serve the GUI:

```
2019/08/25 11:40:14 NOTICE: A new release for gui is present at https://github.com/rclone/rclone-webui-react/releases/download/v0.0.6/currentbuild.zip
2019/08/25 11:40:14 NOTICE: Downloading webgui binary. Please wait. [Size: 3813937, Path :  /home/USER/.cache/rclone/webgui/v0.0.6.zip]
2019/08/25 11:40:16 NOTICE: Unzipping
2019/08/25 11:40:16 NOTICE: Serving remote control on http://127.0.0.1:5572/
```

This assumes you are running rclone locally on your machine.  It is
possible to separate the rclone and the GUI - see below for details.

If you wish to check for updates then you can add `--rc-web-gui-update`
to the command line.

If you find your GUI broken, you may force it to update by add `--rc-web-gui-force-update`.

By default, rclone will open your browser. Add `--rc-web-gui-no-open-browser`
to disable this feature.

## Using the GUI

Once the GUI opens, you will be looking at the dashboard which has an overall overview.

On the left hand side you will see a series of view buttons you can click on:

- Dashboard - main overview
- Configs - examine and create new configurations
- Explorer - view, download and upload files to the cloud storage systems
- Backend - view or alter the backend config
- Log out

(More docs and walkthrough video to come!)

## How it works

When you run the `rclone rcd --rc-web-gui` this is what happens

- Rclone starts but only runs the remote control API ("rc").
- The API is bound to localhost with an auto-generated username and password.
- If the API bundle is missing then rclone will download it.
- rclone will start serving the files from the API bundle over the same port as the API
- rclone will open the browser with a `login_token` so it can log straight in.

## Advanced use

The `rclone rcd` may use any of the [flags documented on the rc page](https://rclone.org/rc/#supported-parameters).

The flag `--rc-web-gui` is shorthand for

- Download the web GUI if necessary
- Check we are using some authentication
- `--rc-user gui`
- `--rc-pass <random password>`
- `--rc-serve`

These flags can be overridden as desired.

See also the [rclone rcd documentation](https://rclone.org/commands/rclone_rcd/).

### Example: Running a public GUI

For example the GUI could be served on a public port over SSL using an htpasswd file using the following flags:

- `--rc-web-gui`
- `--rc-addr :443`
- `--rc-htpasswd /path/to/htpasswd`
- `--rc-cert /path/to/ssl.crt`
- `--rc-key /path/to/ssl.key`

### Example: Running a GUI behind a proxy

If you want to run the GUI behind a proxy at `/rclone` you could use these flags:

- `--rc-web-gui`
- `--rc-baseurl rclone`
- `--rc-htpasswd /path/to/htpasswd`

Or instead of htpasswd if you just want a single user and password:

- `--rc-user me`
- `--rc-pass mypassword`

## Project

The GUI is being developed in the: [rclone/rclone-webui-react repository](https://github.com/rclone/rclone-webui-react).

Bug reports and contributions are very welcome :-)

If you have questions then please ask them on the [rclone forum](https://forum.rclone.org/).


