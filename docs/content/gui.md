---
title: "GUI"
description: "Web based Graphical User Interface"
versionIntroduced: "v1.49"
status: "Beta"
---

# rclone gui

The `rclone gui` command starts the official Web GUI that comes
bundled with rclone.

With this command, rclone can serve a web-based GUI (graphical user
interface) that is accessible from a normal web browser.

Run it in a terminal and rclone will initialize and then start the
GUI.

```console
rclone gui
```

This will produce logs like this. The terminal window needs to stay
open to continue to run the GUI:

```console
2026/04/14 11:36:04 NOTICE: Serving remote control on http://127.0.0.1:50803/
2026/04/14 11:36:04 NOTICE: Serving GUI on http://127.0.0.1:50802/
2026/04/14 11:36:04 NOTICE: GUI available at http://127.0.0.1:50802/login?pass=XXX&url=http%3A%2F%2F127.0.0.1%3A50803%2F&user=gui
```

You can also add debugging flags when running the GUI, such as `-v`,
which will show more logging output from the rc server.

## Using the GUI

Once the browser opens, you will be presented with the Dashboard, the
main screen where you can see the status of your remotes and system.

At the top, starting from the left, you will see a series of tabs you
can click on. On the right side you will have the logout button and
potentially an "Update available" message if a new rclone version has
been released.

### Dashboard

See live metrics, learn if your remotes are near capacity, and read
the changelog for the current version.

### Explorer

Explore and manage both local disks and remotes, download files and
directories and start transfers.

### Remotes

Scroll the list of remotes and tap on it to navigate to the explorer.
There you can navigate the contents of your remotes, transfer,
download, and even upload files.

### Mounts

Mount remotes as local drives on your computer and check on your
existing mounts.

### Serves

Get quick info about your active serve instances, and start new ones.

### Settings

Edit your `rclone.conf` file directly, set logging flags, and
performance parameters.

## How it works

When you run `rclone gui` this is what happens

- Rclone starts the remote control API ("rc").
- Rclone starts a second server to serve the Web GUI.
- If a port, username or password is not specified, then missing
  values will be auto-generated.
- Unless `--no-open-browser` is passed, a browser window will open.
- The URL already contains the username & password, in which case the
  GUI will use those values and log you in automatically.

## Security

It's important to think first about what rclone has access to and what
you might be sharing.

A few good measures:

- Don't use `--no-auth` (this is for testing only).
- Do not expose to the local network (eg with `--api-addr :5572 --addr
  :8080`) unless you trust all devices on your local network. Prefer
  `127.0.0.1` or `localhost` (the default).
- Use a strong password and non-obvious usernames like "admin" or
  "rclone" if you are using `--user` and `--pass`.
- If you want to host it on a server and access it remotely, make sure
  you're only exposing the GUI and not the RC API. They listen on
  different ports.

If you want to access it remotely but want to avoid running a proxy
and exposing ports, you can use Cloudflare Tunnels or localhost.run or
Tailscale (all free).

## Options

```console
      --addr stringArray       IPaddress:Port for the GUI server (default auto-chosen localhost port)
      --api-addr stringArray   IPaddress:Port for the RC API server (default auto-chosen localhost port)
      --enable-metrics         Enable OpenMetrics/Prometheus compatible endpoint at /metrics
  -h, --help                   help for gui
      --no-auth                Don't require auth for the RC API
      --no-open-browser        Skip opening the browser automatically
      --pass string            Password for RC authentication
      --user string            User name for RC authentication
```

## History

In v1.74 the GUI was redone and embedded within rclone for ease of
use. The GUI bundle ships as a compressed zip embedded in the rclone
binary and is served from the zip at runtime.
