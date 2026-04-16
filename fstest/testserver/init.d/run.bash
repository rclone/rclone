#!/usr/bin/env bash
set -euo pipefail

RUN_BASE="${STATE_DIR:-${XDG_RUNTIME_DIR:-/tmp}/rclone-test-server}"
: "${NAME:=$(basename "$0")}"
RUN_ROOT="${RUN_BASE}/${NAME}"
RUN_STATE="${RUN_ROOT}/state"
RUN_LOCK_FILE="${RUN_ROOT}/lock"
RUN_REF_COUNT="${RUN_STATE}/refcount"
RUN_OUTPUT="${RUN_STATE}/env"

mkdir -p "${RUN_STATE}"
[[ -f "${RUN_REF_COUNT}" ]] || echo 0 >"${RUN_REF_COUNT}"
[[ -f "${RUN_OUTPUT}" ]] || : >"${RUN_OUTPUT}"
: > "${RUN_LOCK_FILE}"  # ensure file exists

# status helper that won't trip set -e
_is_running() { set +e; status >/dev/null 2>&1; local rc=$?; set -e; return $rc; }

_acquire_lock() {
  # open fd 9 on lock file and take exclusive lock
  exec 9>"${RUN_LOCK_FILE}"
  flock -x 9
}

_release_lock() {
  flock -u 9
  exec 9>&-
}

case "${1:-}" in
  start)
    _acquire_lock
    trap '_release_lock' EXIT

    rc=$(cat "${RUN_REF_COUNT}" 2>/dev/null || echo 0)

    # If a previous run died without decrementing the refcount, the
    # container will be gone but the count will still be > 0. Treat
    # that as rc=0 so future stops actually reach zero and stop the
    # server.
    if (( rc > 0 )) && ! _is_running; then
      echo "stale refcount ${rc} with no running container — resetting to 0" >&2
      rc=0
    fi

    if (( rc == 0 )); then
      # First client: ensure a clean instance, then start and cache env
      if _is_running; then
        stop || true
      fi
      if ! out="$(start)"; then
        echo "failed to start" >&2
        exit 1
      fi
      printf "%s\n" "$out" > "${RUN_OUTPUT}"
    fi

    rc=$((rc+1)); echo "${rc}" > "${RUN_REF_COUNT}"
    cat "${RUN_OUTPUT}"

    trap - EXIT
    _release_lock
    ;;

  stop)
    _acquire_lock
    trap '_release_lock' EXIT

    rc=$(cat "${RUN_REF_COUNT}" 2>/dev/null || echo 0)
    if (( rc > 0 )); then rc=$((rc-1)); fi
    echo "${rc}" > "${RUN_REF_COUNT}"
    if (( rc == 0 )) && _is_running; then
      stop || true
    fi

    trap - EXIT
    _release_lock
    ;;

  reset)
    _acquire_lock
    trap '_release_lock' EXIT

    stop || true
    rm -rf "${RUN_ROOT}"

    trap - EXIT
    _release_lock
    ;;

  force-stop)
    # Unconditionally stop the server and zero the refcount. Used as a
    # safety-net sweep at the end of a test_all run so a stuck refcount
    # can't leave a container running.
    _acquire_lock
    trap '_release_lock' EXIT

    if _is_running; then
      stop || true
    fi
    echo 0 > "${RUN_REF_COUNT}"

    trap - EXIT
    _release_lock
    ;;

  status)
    # passthrough; do NOT take the lock
    status
    ;;

  *)
    echo "usage: $0 {start|stop|reset|force-stop|status}" >&2
    exit 2
    ;;
esac
