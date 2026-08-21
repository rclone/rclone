---
title: "ADB"
description: "Rclone docs for Android Debug Bridge"
versionIntroduced: "v1.75"
---

# ADB

The ADB backend treats any device exposing the Android Debug Bridge protocol as an rclone remote. ADB is faster than MTP, scriptable, and works headless once the device is paired. The backend was developed and tested against phones and tablets running stock Android 8 through Android 17 (API levels 26, 29, 36, 37); other ADB-speaking devices (Android TV, streaming boxes) should also work because they speak the same protocol, but they are not exercised by the test matrix. The host running rclone connects to the local `adb` daemon over TCP and the daemon connects to the device over USB or wireless ADB; both transports work the same way from rclone's perspective. The backend speaks the ADB wire protocol directly via a vendored Go library and does not shell out to the `adb` binary per operation. The `adb` daemon must be running on the host before rclone connects.

This backend is classified Tier 4 / External in rclone's backend registry. Tier 4 means the backend ships in rclone but receives community maintenance rather than core-team support; "External" means the maintainer is a community contributor (`ferrumclaudepilgrim`), not a member of the rclone core team. File issues at github.com/rclone/rclone with the `adb` label.

## Limitations

**Files larger than 4 GiB minus 1 byte (4294967295 bytes).** The vendored gadb subset uses the SYNC V1 commands (`LIST`, `STAT`, `SEND`, `RECV`) which carry file size as a 32-bit unsigned integer. Files at or above 4 GiB cannot be reported, pushed, or pulled with correct size by this code path. AOSP also defines V2 variants (`LIS2`, `STA2`, `SND2`, `RCV2` per `packages/modules/adb/file_sync_protocol.h`) with a 64-bit size field, but the vendored gadb subset does not implement them. For files above the cap, use `adb push` with `cat` redirection or split the file before transfer; raising the cap in this backend means adding the V2 commands to the vendored gadb subset, which is a follow-up PR not this one.

**Symlinks on `/sdcard`.** The FUSE-backed `/sdcard` mount on Android 11 and later, and the `/data/media` mount on Android 8, both prohibit symlink creation at the filesystem layer. The `--adb-copy-links` option only matters when the configured root is a non-`/sdcard` path (for example `/system` on rooted devices) where symlinks can exist.

**SELinux restricts most non-storage paths.** ADB shell runs in `u:r:shell:s0`. Reads of `/data` (excluding `/data/media`), `/system_ext`, and many vendor partitions are denied even though directory listings appear to work. `rclone copy phone:/data ./backup` will produce a long list of permission errors and partial output. Use specific subpaths the shell context can read.

**Wireless ADB pairing is one-time per host.** The `adb pair` step is required by Android 11 and later before the host can `adb connect` to a wireless device. This is enforced by the device, not by rclone, and is documented in the Android developer docs linked above.

**`/sdcard/Android/data` access on Android 10 and earlier.** The `ext_data_rw` supplementary group (gid 1078), which lets ADB shell read other apps' data directories, is present on Android 11 and later. On Android 10 and earlier, `adb shell` cannot read `/sdcard/Android/data/<other-app>/` without root. Use `--adb-host` to target a device running Android 11 or later if you need cross-app data directory access.

**No metadata exposure.** The backend does not currently expose Android uid/gid, file mode, or SELinux context via rclone's `--metadata` interface. This is a future contribution path; no current rclone backend exposes the equivalent for Linux either.

**Cancellation does not abort in-flight device-side shell operations.** `Ctrl-C` (or any context cancel) returns rclone immediately, but the underlying ADB shell command running on the device continues to completion. A cancelled `copy` may leave the destination file partially written; a cancelled `purge` or `delete` may continue removing files on the device after rclone exits. The producer goroutine on the rclone side is bounded by the per-read socket deadline (60 seconds default), so it always exits, but the device-side work is not aborted by the cancel. This is a property of the ADB shell protocol, which has no in-band cancel signal.

## Configuration

Install Android platform-tools on the host first; it ships the `adb` binary. On Debian or Ubuntu: `apt install adb`. On macOS: `brew install --cask android-platform-tools`. On Windows: download the platform-tools zip from [developer.android.com/tools/releases/platform-tools](https://developer.android.com/tools/releases/platform-tools) and add it to `PATH`.

Make sure the `adb` daemon is running on the host (`adb start-server`) and that at least one device is attached and authorized (`adb devices` shows a device line ending in `device`, not `unauthorized`). On the first USB connection the device shows an "Allow USB debugging?" dialog with a host RSA fingerprint; tap Allow (and "Always allow from this computer" to skip the prompt next time). The backend talks to the daemon, not to the device directly, so the daemon must be up before `rclone config` can complete and before any remote operation can run.

Here is an example of how to make a remote called `phone`. First run:

     rclone config

This will guide you through an interactive setup process:

```
No remotes found, make a new one?
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n

name> phone
Option Storage.
Type of storage to configure.
Choose a number from below, or type in your own value.
[snip]
XX / Android Debug Bridge
   \ "adb"
[snip]
Storage> adb

Edit advanced config?
y) Yes
n) No (default)
y/n> n

Configuration complete.
Options:
- type: adb
Keep this "phone" remote?
y) Yes this is OK (default)
e) Edit this remote
d) Delete this remote
y/e/d> y
```

The walkthrough above is short because all four backend options (`serial`, `host`, `port`, `copy_links`) are advanced. `rclone config` skips them unless you answer "y" to "Edit advanced config?" The defaults connect to the local ADB server at `localhost:5037` and auto-select the only connected device. If multiple devices are attached, the backend errors at first use and asks for the device serial via `--adb-serial` or the matching config entry. Get the serial from `adb devices` output (the first column).

In the examples below, `phone:` is the remote name you just created in `rclone config`; the path after the colon is the path on the device. The default root is `/`. For phones, tablets, and TV / streaming boxes that expose the standard `/sdcard` user-storage layout, append `/sdcard`:

```
rclone ls phone:/sdcard/Download
rclone copy ./photos phone:/sdcard/DCIM/Camera
rclone sync phone:/sdcard/Music ./Music
```

`/sdcard` is the canonical user storage path on consumer Android and is preserved across Android versions through an AOSP `init.rc` symlink chain. Rooted devices commonly use `/data/media/0` (equivalent to `/sdcard` on many devices) and `/storage/emulated/0` as alternatives. The default `/` works but most paths below it are restricted from the ADB shell context by SELinux (see Limitations).

Examples for non-`/sdcard` use on rooted devices:

```
rclone ls phone:/data/media/0/Music                # equivalent to /sdcard/Music on most devices
rclone copy ./apps phone:/data/local/tmp           # cache directory writable to ADB shell
rclone ls phone:/system/etc                        # read-only system config (often listable)
```

### Wireless ADB pairing

USB-attached devices need no extra setup once `adb devices` lists them. Wireless ADB on Android 11 and later requires a one-time `adb pair` against an mDNS-advertised pairing code before `adb connect` will succeed.

The pair port and the connect port are different numbers. The pair port is shown in the wireless debugging settings screen and is used only once. The connect port is the one to use with rclone and with `adb connect`. It changes each time wireless debugging is toggled off and on.

Full setup sequence for a device at 192.168.1.42:

1. On the Android device, enable Developer Options (open Settings, find About phone, tap Build number seven times) then enable Wireless debugging in Settings, System, Developer options.
2. Tap "Pair device with pairing code" to display a pairing code and a pairing port.
3. On the host, pair once:

```
adb pair 192.168.1.42:33445
Enter pairing code: 123456
Successfully paired to 192.168.1.42:33445 [guid=...]
```

4. Connect using the connect port shown in the main Wireless Debugging screen (not the pairing screen):

```
adb connect 192.168.1.42:39221
connected to 192.168.1.42:39221
```

5. Confirm the device is visible:

```
adb devices
List of devices attached
192.168.1.42:39221    device
```

The pairing keypair persists across reboots. Only the connect port rotates. On Android 12 and later, mDNS discovery can reveal the current connect port without re-pairing:

```
adb mdns services
```

Look for the `_adb-tls-connect._tcp` service entry. Its port is the current connect port.

### Modification times and hashes

The backend reports modification times at one-second precision. Android filesystems below `/sdcard` typically resolve to FUSE on Android 11 and later or to `/data/media` on Android 8 (see About below). Both store mtime as POSIX seconds.

The ADB protocol exposes no native file hash. `Hashes()` returns `hash.None`. rclone falls back to size and modification-time comparison for change detection, which is the same approach used for SMB and other hashless backends.

### Restricted filename characters

The Android FUSE-backed `/sdcard` mount accepts most printable Unicode in filenames, including non-ASCII characters such as CJK.

Filenames containing newline characters are not safe. The backend constructs shell command lines using single-quote escaping and a literal newline inside a filename can split the command on the device side. Avoid newlines in filenames you sync via this backend.

### About command

`rclone about phone:` runs `df -k /sdcard` on the device and parses the result. The reported backing filesystem differs by Android version:

- Android 8 (API 26): `/data/media` (direct mount)
- Android 11 and later: `/dev/fuse` (MediaProvider FUSE shim)

The total and free numbers are the same user-visible storage in both cases. The change in backing filesystem reflects the scoped storage architecture introduced in Android 11; the bytes reported are the bytes available to the user.

### /sdcard/Android/data access

This is the differentiator versus MTP and most file-manager apps. Apps targeting Android 11 or later cannot read each other's `/sdcard/Android/data/<package>/` directories even with all storage permissions granted. The ADB shell context can.

The reason is that ADB shell on Android 11 and later runs with the `ext_data_rw` (gid 1078) and `ext_obb_rw` (gid 1079) supplementary groups in addition to its `u:r:shell:s0` SELinux context. These groups exist in AOSP specifically so ADB-based file-management tooling continues to work after scoped storage tightened user-app access. See [source.android.com/docs/core/storage/scoped](https://source.android.com/docs/core/storage/scoped) for the platform-side rationale.

On modern Android (API 36 and API 37), `ls /sdcard/Android/data/` returns the full installed-package list with no root, no special permissions, and no app-side opt-in.

This is officially supported, not a workaround. Use cases:

- Back up app data without root (browser bookmarks, message stores, app caches)
- Migrate app state between devices
- Inspect what an app stores locally

Read access is confirmed across the probed devices. Write access may be more restricted for some package directories depending on OEM (notably Samsung Knox-protected packages). Test before relying on write paths into specific app directories.

### Connecting to a remote ADB server

The `host` and `port` options point at a remote `adb` daemon over TCP. This is useful when the host running rclone is not the host running the daemon, for example syncing from a NAS to a phone that is paired with a desktop. Set `host` to the daemon's hostname or IP and `port` to its server port (default 5037). Note that the `adb` daemon listens on localhost only by default; reach a remote daemon through an SSH port forward, or start the daemon with the `-a` flag (`adb -a start-server`) to listen on all interfaces.

### Performance

Indicative numbers measured against an Android 16 (API 36) handset over wireless ADB on a local network. Each row is the median of three runs of `rclone copy` (push) or `rclone sync` (pull) with `--progress` against a freshly created scratch directory on `/sdcard`, with `adb start-server` on the host to ensure no cold-start variance:

| Operation | Throughput |
|---|---|
| Push 100 MiB | 15.82 MiB/s |
| Pull 100 MiB | 21.24 MiB/s |
| List 1000-entry directory | 0.22 s |
| Push 100 small files (4 KiB each) | 71.71 files/s |

USB-attached devices are typically faster than the wireless ADB numbers above because they avoid the network hop. Wireless ADB is the more useful baseline for headless and remote-management use cases.

### Daemon lifecycle

The backend connects to whichever `adb` daemon is reachable at the configured `host` and `port`. If the daemon dies (after a long idle period, an Android Studio restart, or a system update), rclone returns a connection error and exits. There is no auto-restart. The recovery is a one-line shell command (`adb start-server`) followed by re-running the rclone invocation. If you are running rclone from cron or a long-lived service against a phone that disconnects, wrap the call so it reconnects on failure. A minimal pattern:

```
adb start-server
rclone sync phone:/sdcard/DCIM ./backup
```

A retry loop for cron-driven backups against a phone that disconnects:

```
until adb start-server && rclone sync phone:/sdcard/DCIM ./backup; do
    sleep 60
done
```

<!-- autogenerated options start - DO NOT EDIT - instead edit fs.RegInfo in backend/adb/adb.go and run make backenddocs to verify --> <!-- markdownlint-disable-line line-length -->
### Advanced options

Here are the Advanced options specific to adb (Android Debug Bridge).

#### --adb-serial

The device serial to use. Leave empty for auto selection.

Properties:

- Config:      serial
- Env Var:     RCLONE_ADB_SERIAL
- Type:        string
- Required:    false

#### --adb-host

The ADB server host.

Properties:

- Config:      host
- Env Var:     RCLONE_ADB_HOST
- Type:        string
- Default:     "localhost"

#### --adb-port

The ADB server port.

Properties:

- Config:      port
- Env Var:     RCLONE_ADB_PORT
- Type:        uint16
- Default:     5037

#### --adb-copy-links

Follow symlinks and copy the pointed to item.

Properties:

- Config:      copy_links
- Env Var:     RCLONE_ADB_COPY_LINKS
- Type:        bool
- Default:     false

#### --adb-encoding

The encoding for the backend.

See the [encoding section in the overview](/overview/#encoding) for more info.

Properties:

- Config:      encoding
- Env Var:     RCLONE_ADB_ENCODING
- Type:        Encoding
- Default:     Slash,LtGt,DoubleQuote,Colon,Question,Asterisk,Pipe,Percent,BackSlash,Del,Ctl,LeftSpace,LeftPeriod,LeftTilde,LeftCrLfHtVt,RightSpace,RightPeriod,RightCrLfHtVt,InvalidUtf8,Dot

#### --adb-description

Description of the remote.

Properties:

- Config:      description
- Env Var:     RCLONE_ADB_DESCRIPTION
- Type:        string
- Required:    false

<!-- autogenerated options stop -->

