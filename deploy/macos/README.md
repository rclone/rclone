# `rclone-mount-watchdog.sh`

A self-healing wrapper around `rclone serve nfs` + `mount_nfs` for mounting
an rclone remote (e.g. an S3 bucket) onto a local path on macOS, used to
mount large/flat buckets where fast single-file `stat` access is required
without triggering full directory listings.

The script is driven entirely by command-line flags, so any number of
deployments (read-only or read-write, single or multiple instances per
host) can share the same script file, with each deployment's LaunchAgent
`.plist` simply passing different arguments.

## What it does

1. Starts `rclone serve nfs <remote>` on a loopback TCP port.
2. Mounts it locally via `mount_nfs` with `soft,timeo=30,retrans=3,nolocks`
   so that if the NFS server becomes unresponsive, the kernel eventually
   returns `ETIMEDOUT` instead of hanging forever.
3. Runs a lightweight background watchdog that periodically `stat`s a
   random, never-existing filename under the mountpoint. It never cares
   whether the file exists — only whether the call returns within a bounded
   time. A fast `ENOENT` is a pass; a timeout is treated as a hang.
4. On a detected hang, it:
   - Collects system diagnostics (`ps`, `mount`, `df`, `uptime`, `lsof +D`).
   - Captures the last ~5000 lines of the log.
   - Sends `SIGQUIT` to the rclone process. rclone deliberately does not
     intercept this signal, so it dumps every goroutine's stack to its own
     log output and then terminates — giving a full diagnostic dump with a
     single signal.
   - Force-unmounts the mountpoint.
   - Writes a `summary.txt` and exits non-zero.
5. It **does not restart anything itself.** Recovery is delegated entirely
   to the service manager (`launchd`'s `KeepAlive`/`ThrottleInterval`),
   which respawns the whole script from scratch after it exits. This keeps
   the script's own logic simple and leaves restart/backoff policy in one
   place.

## Flags

| Flag | Required | Default | Purpose |
|---|---|---|---|
| `-r REMOTE` | yes | — | rclone remote to serve, e.g. `my-remote:` |
| `-m MOUNTPOINT` | yes | — | local path to mount at |
| `-p PORT` | no | `51397` | loopback NFS port |
| `-s SERVICE_NAME` | no | basename of `MOUNTPOINT` | label used for hang-report subdirectories, so multiple instances running on the same host don't collide |
| `-l LOG_FILE` | no | `/tmp/rclone-<service>.log` | where this script (and rclone, which inherits the same descriptors) appends its output. The script `exec`s its own stdout/stderr here early on, so this is authoritative — it does **not** need to match the service manager's `StandardOutPath`/`StandardErrorPath`, though pointing those at the same file avoids a confusing second, empty log |
| `-R true\|false` | no | `true` | pass `--read-only` to rclone |
| `-C SIZE` | no | unset | `--vfs-cache-max-size`, e.g. `4G` |
| `-A DURATION` | no | unset | `--vfs-cache-max-age`, e.g. `1h` |
| `-i SECONDS` | no | `60` | watchdog probe interval |
| `-t SECONDS` | no | `10` | max time a single probe may take before being treated as a hang |
| `-d DIR` | no | `~/.config/rclone/hang-reports` | base directory for hang reports |
| `-w true\|false` | no | `true` | enable/disable the watchdog (useful for manual/debug runs) |

## Example usage

Read-only, single instance:

```sh
rclone-mount-watchdog.sh \
  -r my-remote-noindex: \
  -m "$HOME/mnt/my-remote" \
  -p 51400 \
  -s my-remote \
  -l "$HOME/.config/rclone/my-remote.log" \
  -R false
```

Read-only, two instances sharing one script on the same host (distinct
ports, service names, and log files so their hang reports and logs don't
collide):

```sh
rclone-mount-watchdog.sh -r remote-a-noindex: -m "$HOME/mnt/remote-a" \
  -p 51401 -s remote-a -l "$HOME/.config/rclone/remote-a.log" \
  -R true -C 4G -A 1h

rclone-mount-watchdog.sh -r remote-b-noindex: -m "$HOME/mnt/remote-b" \
  -p 51402 -s remote-b -l "$HOME/.config/rclone/remote-b.log" \
  -R true -C 4G -A 1h
```

Each would typically be launched by its own LaunchAgent, with
`ProgramArguments` passing the flags above.

## Hang reports

Written to `<REPORT_DIR>/<SERVICE_NAME>-<timestamp>/`:

| File | Contents |
|---|---|
| `diagnostics-before.txt` | `ps`/`mount`/`df`/`uptime`/`lsof +D` output captured at hang-detection time |
| `recent-log.txt` | last ~5000 lines of `LOG_FILE` |
| `goroutine-dump.txt` | the portion of `LOG_FILE` appended by rclone's `SIGQUIT` handler (all goroutine stacks). May be empty if rclone could not process the signal (e.g. if its process was itself unschedulable, not just its NFS-serving logic stuck) |
| `summary.txt` | service name, remote, mountpoint, probe filename, detection time, elapsed vs. timeout |

Reports are never pruned automatically — clean up old ones manually or with
your own retention tooling if disk usage becomes a concern.

## Known limitation: uninterruptible (D-state) kernel waits

On a sufficiently wedged NFS client, individual diagnostic commands
(`lsof`, `df`) or even the force-unmount itself can enter an
uninterruptible kernel sleep that no signal — including `SIGKILL` — can
clear until the kernel's own internal timeout eventually releases it. Every
such step is wrapped with its own bounded timeout (via the script's
`run_with_timeout` helper, which also kills the whole process group so a
command that forks children, like `lsof`, can't leave orphans behind once
they're no longer D-state) so this can only ever *delay* recovery — testing
observed up to roughly a minute in a pathological case — it cannot make the
watchdog hang forever. A delayed exit does mean a delayed `launchd`
respawn, though, so this is a real (if rare and self-resolving) source of
extended downtime to be aware of.

## Requirements on the LaunchAgent `.plist`

- `ProgramArguments` should point at this script with the flags for that
  deployment (see examples above).
- `KeepAlive` should be configured (e.g. `{"SuccessfulExit": false}` or
  simply `true`) so launchd respawns the script after it exits, whatever
  the reason.
- A `ThrottleInterval` is recommended to avoid a restart storm if the
  remote is persistently unreachable.
- `StandardOutPath`/`StandardErrorPath` are optional given `-l`, but should
  point at the same file as `-l` if set, to avoid two divergent logs.
