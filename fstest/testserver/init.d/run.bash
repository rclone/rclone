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
    else
      # Already owned: make sure it’s still up; if not, restart and refresh env
      if ! _is_running; then
        if ! out="$(start)"; then
          echo "failed to restart" >&2
          exit 1
        fi
        printf "%s\n" "$out" > "${RUN_OUTPUT}"
      fi
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
    rm -rf "${RUN_BASE}"

    trap - EXIT
    _release_lock
    ;;

  status)
    # passthrough; do NOT take the lock
    status
    ;;

  *)
    echo "usage: $0 {start|stop|reset|status}" >&2
    exit 2
    ;;
esac
