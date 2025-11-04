#!/usr/bin/env bash
set -euo pipefail

BASE="${STATE_DIR:-${XDG_RUNTIME_DIR:-/tmp}/rclone-test-server}"
NAME="$(basename "$0")"
ROOT="${BASE}/${NAME}"
STATE="${ROOT}/state"
LOCKF="${ROOT}/lock"
REFC="${STATE}/refcount"
ENVF="${STATE}/env"

mkdir -p "${STATE}"
[[ -f "${REFC}" ]] || echo 0 >"${REFC}"
[[ -f "${ENVF}" ]] || : >"${ENVF}"
: > "${LOCKF}"  # ensure file exists

# status helper that won't trip set -e
_is_running() { set +e; status >/dev/null 2>&1; local rc=$?; set -e; return $rc; }

_acquire_lock() {
  # open fd 9 on lock file and take exclusive lock
  exec 9>"${LOCKF}"
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

    rc=$(cat "${REFC}" 2>/dev/null || echo 0)

    if (( rc == 0 )); then
      # First client: ensure a clean instance, then start and cache env
      if _is_running; then
        stop || true
      fi
      if ! out="$(start)"; then
        echo "failed to start" >&2
        exit 1
      fi
      printf "%s\n" "$out" > "${ENVF}"
    else
      # Already owned: make sure it’s still up; if not, restart and refresh env
      if ! _is_running; then
        if ! out="$(start)"; then
          echo "failed to restart" >&2
          exit 1
        fi
        printf "%s\n" "$out" > "${ENVF}"
      fi
    fi

    rc=$((rc+1)); echo "${rc}" > "${REFC}"
    cat "${ENVF}"

    trap - EXIT
    _release_lock
    ;;

  stop)
    _acquire_lock
    trap '_release_lock' EXIT

    rc=$(cat "${REFC}" 2>/dev/null || echo 0)
    if (( rc > 0 )); then rc=$((rc-1)); fi
    echo "${rc}" > "${REFC}"
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
    rm -rf "${BASE}"

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
