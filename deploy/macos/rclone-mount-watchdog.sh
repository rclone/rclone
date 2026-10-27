#!/bin/sh
# Mount an rclone remote via NFS and self-monitor it for hangs.
# Run with -h/--help for full usage, flag reference, and design notes
# (the authoritative docs live in the usage() function below, not here,
# to avoid two copies drifting out of sync).
REMOTE=""
MOUNTPOINT=""
NFS_PORT=51397
SERVICE_NAME=""
LOG_FILE=""
READONLY="true"
VFS_CACHE_MAX_SIZE=""
VFS_CACHE_MAX_AGE=""
WATCHDOG_INTERVAL=60
WATCHDOG_TIMEOUT=10
REPORT_DIR="${HOME}/.config/rclone/hang-reports"
WATCHDOG_ENABLE="true"
RCLONE="${RCLONE_BIN:-/opt/homebrew/bin/rclone}"

# Captured before any backgrounding so the watchdog can signal this exact
# process regardless of shell subshell semantics.
SCRIPT_PID=$$

# Enables job control so every '( ... ) &' backgrounded below gets its own
# process group (pgid == its own pid). This matters for the timeout-killer
# helpers further down: a command like 'lsof' or 'stat' can itself fork
# children, and 'kill -9' on just the wrapper subshell's pid would leave
# those children (and lsof in particular, observed during testing) running
# indefinitely, reparented to init. Killing the negative pid (the whole
# process group) instead reliably takes down the command and everything it
# spawned.
set -m

# status: exit code to use (0 for an explicit -h/--help request, 1 for a
# usage error such as a missing required flag or an unknown option).
usage() {
  status="${1:-1}"
  cat >&2 <<EOF
Usage: $0 -r REMOTE -m MOUNTPOINT [options]

Mounts an rclone remote via NFS (pure-Go NFS server, no FUSE/kext required)
and self-monitors it for hangs. A background watchdog periodically proves
the mount is still responsive by stat-ing a random, never-existing
filename; it never cares whether the file exists, only whether the call
returns within a bounded time. On a detected hang it captures a diagnostic
report (goroutine dump + recent logs + system state), force-unmounts, and
exits non-zero so that the service manager (launchd KeepAlive/
ThrottleInterval) respawns the whole script/mount from scratch. The script
itself never restarts anything -- that responsibility is left entirely to
the service manager.

Required:
  -r REMOTE      rclone remote to serve, e.g. 'my-remote:'
  -m MOUNTPOINT   local path to mount at

Optional:
  -p PORT         loopback NFS port (default: ${NFS_PORT})
  -s SERVICE_NAME label used for hang-report subdirectories
                  (default: basename of MOUNTPOINT)
  -l LOG_FILE     path this script (and the rclone process it starts)
                  appends its own stdout/stderr to. The script redirects
                  itself early via 'exec', so this is authoritative
                  regardless of what the service manager's
                  StandardOutPath/StandardErrorPath say; if those are set
                  too, point them at the same path to avoid a confusing
                  second, empty log. (default: /tmp/rclone-<service>.log)
  -R true|false   pass --read-only to rclone (default: ${READONLY})
  -C SIZE         --vfs-cache-max-size value, e.g. '4G' (unset by default)
  -A DURATION     --vfs-cache-max-age value, e.g. '1h' (unset by default)
  -i SECONDS      watchdog probe interval (default: ${WATCHDOG_INTERVAL})
  -t SECONDS      watchdog per-probe timeout (default: ${WATCHDOG_TIMEOUT}).
                  Must stay comfortably above the mount's own worst-case
                  soft-timeout window (timeo*retrans, ~9s by default) or a
                  legitimate, self-resolving slow response can be
                  misclassified as a hang.
  -d REPORT_DIR   base directory for hang reports
                  (default: ~/.config/rclone/hang-reports)
  -w true|false   enable/disable the watchdog (default: ${WATCHDOG_ENABLE})
  -h, --help      show this help and exit

Known limitation: on a sufficiently wedged NFS client, individual
diagnostic commands (lsof, df) or even the force-unmount itself can enter
an uninterruptible kernel sleep (D-state) that no signal, including
SIGKILL, can clear until the kernel's own internal timeout eventually
releases it. Every such step is wrapped with a bounded timeout so this can
only ever *delay* recovery (observed in testing: up to roughly a minute in
a pathological case) -- it cannot make the watchdog hang forever -- but a
delayed exit means a delayed launchd respawn too.

See deploy/macos/README.md for full examples per deployment and the hang
report file format.
EOF
  exit "$status"
}

# Handled before getopts so both spellings work: getopts only understands
# single-letter flags, not GNU-style long options like --help.
case "${1:-}" in
  -h | --help) usage 0 ;;
esac

while getopts "hr:m:p:s:l:R:C:A:i:t:d:w:" opt; do
  case "$opt" in
    h) usage 0 ;;
    r) REMOTE="$OPTARG" ;;
    m) MOUNTPOINT="$OPTARG" ;;
    p) NFS_PORT="$OPTARG" ;;
    s) SERVICE_NAME="$OPTARG" ;;
    l) LOG_FILE="$OPTARG" ;;
    R) READONLY="$OPTARG" ;;
    C) VFS_CACHE_MAX_SIZE="$OPTARG" ;;
    A) VFS_CACHE_MAX_AGE="$OPTARG" ;;
    i) WATCHDOG_INTERVAL="$OPTARG" ;;
    t) WATCHDOG_TIMEOUT="$OPTARG" ;;
    d) REPORT_DIR="$OPTARG" ;;
    w) WATCHDOG_ENABLE="$OPTARG" ;;
    *) usage 1 ;;
  esac
done
# getopts doesn't remove the parsed options from $@ on its own -- without
# this shift, "$@" below would still be the script's own raw argv. It's
# harmless today only because the 'set -- serve nfs ...' further down
# overwrites $@ before anything reads it first, but leaving the shift out
# would silently swallow any stray trailing operand (e.g. a typo'd extra
# argument) instead of reporting it. Shift it away now and reject anything
# left over.
shift $((OPTIND - 1))
if [ "$#" -gt 0 ]; then
  echo "Unexpected extra argument(s): $*" >&2
  usage 1
fi


[ -n "$REMOTE" ] || usage
[ -n "$MOUNTPOINT" ] || usage
[ -n "$SERVICE_NAME" ] || SERVICE_NAME=$(basename "$MOUNTPOINT")
[ -n "$LOG_FILE" ] || LOG_FILE="/tmp/rclone-${SERVICE_NAME}.log"

# If $HOME is empty/unset (plausible in some launchd execution contexts
# that don't inherit a full user session environment) the -d default above
# would resolve to '/.config/rclone/hang-reports', which a non-root agent
# almost certainly can't create -- fall back to a /tmp-based path instead
# of silently never being able to write a single hang report.
case "$REPORT_DIR" in
  /.config/*) REPORT_DIR="/tmp/rclone-hang-reports" ;;
esac

# Make this script the authority on where its (and rclone's, since it
# inherits these descriptors) output goes, rather than relying on the
# service manager's redirect happening to match $LOG_FILE. Everything
# forked from here on, including rclone itself, writes into LOG_FILE.
mkdir -p "$(dirname "$LOG_FILE")" 2>/dev/null || true
exec >> "$LOG_FILE" 2>&1

mkdir -p "$MOUNTPOINT"

# Runs "$@" in the background and kills it if it hasn't finished within
# $1 seconds. Needed because several commands that touch the (possibly
# hung) mounted filesystem -- 'lsof', 'df', and even 'umount'/'diskutil
# unmount' itself -- can block indefinitely against a sufficiently wedged
# NFS client; testing observed this surface as a kernel-level
# uninterruptible-sleep (D-state) condition that not even SIGKILL can clear
# until the kernel's own internal timeout eventually releases it. Defined
# early (before cleanup()) since cleanup() also needs it.
run_with_timeout() {
  to="$1"; shift
  ( "$@" ) &
  cmd_pid=$!
  # Kill the whole process group (negative pid), not just $cmd_pid: 'set -m'
  # above gives this background job its own pgid, so this also reaps any
  # children the command itself forked (e.g. lsof), which a plain
  # 'kill -9 "$cmd_pid"' would otherwise orphan and leave running forever.
  # Note this still cannot clear a genuinely D-state (uninterruptible)
  # process -- no userspace signal can -- it only bounds how long this
  # function itself waits before giving up and letting the caller proceed.
  ( sleep "$to"; kill -9 -- "-$cmd_pid" 2>/dev/null; kill -9 "$cmd_pid" 2>/dev/null ) &
  timer_pid=$!
  wait "$cmd_pid" 2>/dev/null
  kill "$timer_pid" 2>/dev/null
  wait "$timer_pid" 2>/dev/null
}

CLEANUP_DONE=""
cleanup() {
  # Re-entrancy guard: this trap can fire more than once (e.g. once for a
  # caught TERM, then again for EXIT as the script subsequently exits) --
  # without this guard a second pass could re-signal an already-exited
  # WATCHDOG_PID or re-attempt an already-completed unmount pointlessly.
  [ -n "$CLEANUP_DONE" ] && return 0
  CLEANUP_DONE=1

  # Signal the watchdog, then actually wait for it to exit before
  # unmounting. This matters for the case where an external signal (e.g.
  # launchd stopping the service, an operator running 'kill'/'launchctl
  # stop') arrives while the watchdog subshell is mid-handle_hang() writing
  # a report: a bare 'kill' here without waiting would let this function
  # race ahead to its own unmount while handle_hang() is still writing
  # files, truncating the report. Waiting also means WATCHDOG_PID has
  # already naturally exited in the common "hang detected -> handle_hang
  # signals us" path, making this wait an instant no-op there.
  if [ -n "$WATCHDOG_PID" ]; then
    kill "$WATCHDOG_PID" 2>/dev/null
    wait "$WATCHDOG_PID" 2>/dev/null
  fi
  run_with_timeout 15 sh -c 'diskutil unmount force "$1" 2>/dev/null || umount -f "$1" 2>/dev/null || true' -- "$MOUNTPOINT"
  kill "$NFS_PID" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

# Kill any stale rclone instance already serving NFS on the same port.
# This can happen when the mount was force-unmounted without killing rclone:
# launchd restarts this script, but rclone from the previous run is still
# running, so mount_nfs would never be reached.
#
# NOTE: this identifies "stale" purely by port number, with no check that
# it belongs to this SERVICE_NAME/MOUNTPOINT. Each deployment MUST use a
# distinct -p PORT (see README examples) -- if two instances were ever
# misconfigured to share a port, starting one would kill the other's
# legitimately-running rclone.
STALE=$(pgrep -f "rclone serve nfs.*${NFS_PORT}" 2>/dev/null || true)
if [ -n "$STALE" ]; then
  echo "Killing stale rclone (PID $STALE) on port ${NFS_PORT}"
  kill "$STALE" 2>/dev/null || true
  sleep 1
fi

# Build the rclone argument list with 'set --' (POSIX-portable, no arrays
# needed) so optional flags can be omitted cleanly.
#
# --vfs-cache-max-size / --vfs-cache-max-age bound the VFS cache's memory
# footprint when set. Without them, --vfs-cache-mode full has no cap (rclone
# default is "off" = unbounded) and the process can accumulate many GB of
# resident memory over a long uptime, since nothing is ever evicted.
set -- serve nfs "$REMOTE" \
  --addr "127.0.0.1:${NFS_PORT}" \
  --vfs-lazy-dir-read \
  --vfs-case-insensitive=false \
  --no-unicode-normalization \
  --dir-cache-time 24h \
  --vfs-cache-mode full
[ "$READONLY" = "true" ] && set -- "$@" --read-only
[ -n "$VFS_CACHE_MAX_SIZE" ] && set -- "$@" --vfs-cache-max-size "$VFS_CACHE_MAX_SIZE"
[ -n "$VFS_CACHE_MAX_AGE" ] && set -- "$@" --vfs-cache-max-age "$VFS_CACHE_MAX_AGE"

# Start rclone NFS server (serves only — does not mount)
"$RCLONE" "$@" &
NFS_PID=$!

# Wait for server to be ready (up to 10s)
i=0
while [ $i -lt 10 ]; do
  sleep 1
  nc -z 127.0.0.1 "$NFS_PORT" 2>/dev/null && break
  i=$((i + 1))
done

# Mount with soft NFS options:
#   soft      - return ETIMEDOUT instead of hanging if server disappears
#   timeo=30  - 3s initial per-RPC timeout (units are 0.1s)
#   dumbtimer - disable the adaptive/dynamic retransmit timer so timeo stays
#               fixed at 3s per try instead of growing with observed RTT.
#               Without this, a struggling-but-not-dead rclone can cause the
#               kernel to adaptively balloon its own timeout well past the
#               nominal timeo*retrans worst case below -- exactly the
#               condition this watchdog needs a *predictable* bound for, so
#               WATCHDOG_TIMEOUT (-t) can safely stay well above it without
#               guessing at an adaptive ceiling.
#   retrans=3 - retry 3 times before giving up (worst case: ~3s*3=9s -> the
#               default -t of 10s intentionally sits just above this; raise
#               -t if timeo/retrans are ever tuned looser, to avoid
#               misclassifying a legitimate, self-resolving soft-timeout as
#               a watchdog-detected hang)
#   nolocks   - disable NLM lock manager (not needed for loopback)
if ! mount_nfs -o soft,timeo=30,dumbtimer,retrans=3,nolocks,port="${NFS_PORT}",mountport="${NFS_PORT}",tcp \
  "127.0.0.1:/" "$MOUNTPOINT"; then
  echo "mount_nfs failed for $MOUNTPOINT; stopping rclone and exiting for launchd retry" >&2
  kill "$NFS_PID" 2>/dev/null || true
  wait "$NFS_PID" 2>/dev/null || true
  exit 1
fi

# --- Watchdog -----------------------------------------------------------
#
# Captures a diagnostic report, then force-unmounts and signals this script
# to exit. It deliberately does NOT try to restart anything itself — the
# service manager (launchd KeepAlive/ThrottleInterval) owns that, exactly
# like the "stale rclone on same port" recovery above already assumes.
handle_hang() {
  elapsed="$1"

  # Ignore external shutdown signals for the duration of report generation.
  # Without this, a TERM/INT arriving mid-report (e.g. an operator running
  # 'launchctl stop'/'kill' while a hang is actively being diagnosed) would
  # kill this subshell immediately, truncating diagnostics-before.txt /
  # recent-log.txt / goroutine-dump.txt / summary.txt — the exact failure
  # mode this function exists to avoid, just triggered externally instead
  # of by the internal rclone-death race. cleanup() in the main script
  # still responds to such a signal right away and waits for this subshell
  # (see 'wait "$WATCHDOG_PID"' there), so shutdown is only delayed, not
  # lost, by however long this function takes to finish on its own.
  trap '' TERM INT

  ts=$(date +%Y%m%d-%H%M%S)
  report_dir="${REPORT_DIR}/${SERVICE_NAME}-${ts}"
  if ! mkdir -p "$report_dir" 2>/dev/null; then
    # Report directory couldn't be created (e.g. $HOME unset/unwritable in
    # this launchd context) -- fall back to /tmp rather than silently
    # losing every diagnostic file with no indication anywhere that the
    # one thing this function exists to produce didn't happen.
    echo "WARNING: could not create report dir '$report_dir'; falling back to /tmp" >&2
    report_dir="/tmp/${SERVICE_NAME}-hang-${ts}"
    mkdir -p "$report_dir" 2>/dev/null || report_dir="/tmp"
  fi

  # System-level diagnostics. ps/mount/uptime are pure process/kernel-table
  # reads and cannot block. 'df' and 'lsof +D' *can* block against a truly
  # unresponsive NFS server (both ultimately call statfs()/stat() on the
  # mount), so they're run through run_with_timeout to guarantee this
  # function itself can never hang.
  {
    echo "=== ps aux | grep rclone ==="
    ps aux | grep -i rclone | grep -v grep
    echo
    echo "=== mount ==="
    mount | grep -i nfs
    echo
    echo "=== df -h (10s timeout) ==="
    run_with_timeout 10 df -h
    echo
    echo "=== uptime ==="
    uptime
    echo
    echo "=== lsof +D \"$MOUNTPOINT\" (10s timeout) ==="
    run_with_timeout 10 lsof +D "$MOUNTPOINT"
  } > "${report_dir}/diagnostics-before.txt" 2>&1

  # Best-effort "recent logs": a bounded tail, since POSIX sh has no simple
  # by-timestamp log filter. 5000 lines is a practical proxy for "the last
  # hour or so" at this service's typical log verbosity.
  tail -n 5000 "$LOG_FILE" > "${report_dir}/recent-log.txt" 2>/dev/null

  # Goroutine dump: rclone intentionally does not intercept SIGQUIT (see
  # lib/atexit/atexit_unix.go upstream), so sending it both (a) makes the Go
  # runtime dump every goroutine's stack to this process's existing log
  # output, and (b) terminates the process immediately afterward. Snapshot
  # the log's line count before signaling so we can isolate just the dump.
  before_lines=$(wc -l < "$LOG_FILE" 2>/dev/null || echo 0)
  kill -QUIT "$NFS_PID" 2>/dev/null
  sleep 5
  tail -n +"$((before_lines + 1))" "$LOG_FILE" > "${report_dir}/goroutine-dump.txt" 2>/dev/null

  # Fallback hard-kill in case something prevented prompt termination above.
  kill -9 "$NFS_PID" 2>/dev/null

  # Force-unmount so any processes blocked on the mount are released now,
  # rather than waiting for the script's own exit/trap to get there. Bounded
  # with run_with_timeout too: 'diskutil unmount force'/'umount -f' can
  # themselves block against a sufficiently wedged NFS client (observed
  # during testing as a kernel-level uninterruptible-sleep condition that
  # not even SIGKILL can clear) — this must never be the reason the whole
  # recovery sequence stalls.
  run_with_timeout 15 sh -c 'diskutil unmount force "$1" 2>/dev/null || umount -f "$1" 2>/dev/null || true' -- "$MOUNTPOINT"

  {
    echo "service: $SERVICE_NAME"
    echo "remote: $REMOTE"
    echo "mountpoint: $MOUNTPOINT"
    echo "probe file: $probe_name"
    echo "hang detected at: $(date)"
    echo "elapsed: ${elapsed}s (timeout ${WATCHDOG_TIMEOUT}s)"
  } > "${report_dir}/summary.txt"

  echo "Hang detected on $MOUNTPOINT; report written to $report_dir" >&2

  # Belt-and-suspenders: killing $NFS_PID above should already cause the
  # 'wait "$NFS_PID"' below to unblock and the EXIT trap to fire, but signal
  # the script explicitly in case that doesn't happen promptly for any
  # reason (e.g. the goroutine dump itself got stuck).
  kill -TERM "$SCRIPT_PID" 2>/dev/null
}

watchdog_loop() {
  # Random, unique, deliberately never-existing probe filename, generated
  # once and reused for every check. Existence is irrelevant — only
  # responsiveness (an ENOENT returning fast vs. not returning at all)
  # matters, so a stat() of this name exercises the exact statLazy() ->
  # HeadObject (+ bounded virtual-dir probe) path, never a full listing.
  rand=$(od -An -N8 -tx1 /dev/urandom 2>/dev/null | tr -d ' \n')
  probe_name=".watchdog-probe-$$-$(date +%s)-${rand:-fallback}"

  # Grace period before the first check so mount warm-up isn't mistaken for
  # a hang.
  sleep "$WATCHDOG_INTERVAL"

  while true; do
    start_ts=$(date +%s)
    ( stat "${MOUNTPOINT}/${probe_name}" ) >/dev/null 2>&1 &
    probe_pid=$!
    # Process-group kill (see run_with_timeout above) so a 'stat' that forks
    # internally can't leave an orphaned descendant stuck against the mount.
    ( sleep "$WATCHDOG_TIMEOUT"; kill -9 -- "-$probe_pid" 2>/dev/null; kill -9 "$probe_pid" 2>/dev/null ) &
    timer_pid=$!
    wait "$probe_pid" 2>/dev/null
    kill "$timer_pid" 2>/dev/null
    wait "$timer_pid" 2>/dev/null
    elapsed=$(( $(date +%s) - start_ts ))
    if [ "$elapsed" -ge "$WATCHDOG_TIMEOUT" ]; then
      handle_hang "$elapsed"
      exit 0
    fi
    sleep "$WATCHDOG_INTERVAL"
  done
}

if [ "$WATCHDOG_ENABLE" = "true" ]; then
  watchdog_loop &
  WATCHDOG_PID=$!
fi

# Block until all background children (rclone AND the watchdog, if enabled)
# exit; trap handles cleanup. Waiting on NFS_PID alone would be wrong: when
# handle_hang() kills rclone partway through producing a hang report (via
# SIGQUIT or its kill -9 fallback), that alone would wake this 'wait', fire
# the EXIT trap, and cleanup() would kill WATCHDOG_PID while handle_hang()
# is still writing the goroutine dump/summary — truncating the report. By
# waiting on every child, the script only proceeds to cleanup() once the
# watchdog (and therefore any in-flight handle_hang) has itself finished and
# exited.
wait
