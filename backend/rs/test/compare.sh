#!/usr/bin/env bash
#
# compare.sh — black-box tests for the rs backend (local or MinIO shard backends).
#
# Usage:
#   ./compare.sh list
#   ./compare.sh --storage-type=local test smoke
#   ./compare.sh --storage-type=minio test smoke   # needs Docker; ./setup.sh + MinIO config
#   ./compare.sh --storage-type=local test verify   # smoke + rsverify check (needs ./rsverify)
#   ./compare.sh --storage-type=local test heal      # smoke, drop last shard, heal (single-object), rsverify
#
# Requires: ./setup.sh run once from this directory.
#

set -euo pipefail

SCRIPT_NAME=$(basename "$0")
SCRIPT_DIR=$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)

# shellcheck source=compare_common.sh
. "${SCRIPT_DIR}/compare_common.sh"

VERBOSE=0
STORAGE_TYPE=""
COMMAND=""
COMMAND_ARG=""

usage() {
  cat <<EOF
Usage: ${SCRIPT_NAME} [options] <command>

Commands:
  list                 Show available tests.
  test <name>          Run a named test (e.g. smoke).

Options:
  --storage-type local|minio   Required for test (minio: Docker MinIO on ports 9201+, see compare_rs_env.sh).
  -v, --verbose                Show rclone stdout/stderr.
  -h, --help

Run from: ${SCRIPT_DIR}
EOF
}

# rclone with optional S3 timeouts when STORAGE_TYPE=minio (mirrors raid3 compare_common).
rs_rclone() {
  local bin
  bin=$(find_rclone_binary)
  if [[ "${STORAGE_TYPE:-}" == "minio" ]]; then
    "${bin}" --config "${RCLONE_CONFIG}" --retries 1 \
      --contimeout "${RCLONE_CONTIMEOUT:-15s}" --timeout "${RCLONE_HTTP_TIMEOUT:-90s}" "$@"
  else
    "${bin}" --config "${RCLONE_CONFIG}" --retries 1 "$@"
  fi
}

print_if_verbose() {
  local title="$1"
  local out="$2"
  local err="$3"
  if [[ "${VERBOSE}" -eq 1 ]]; then
    printf '\n--- %s (stdout) ---\n' "${title}"
    cat "${out}" 2>/dev/null || true
    printf '\n--- %s (stderr) ---\n' "${title}"
    cat "${err}" 2>/dev/null || true
    printf '\n'
  fi
}

parse_args() {
  if [[ $# -eq 0 ]]; then
    usage
    exit 0
  fi
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --storage-type)
        shift
        [[ $# -gt 0 ]] || die "--storage-type requires a value"
        STORAGE_TYPE="$1"
        ;;
      --storage-type=*)
        STORAGE_TYPE="${1#*=}"
        ;;
      -v | --verbose)
        VERBOSE=1
        ;;
      -h | --help)
        usage
        exit 0
        ;;
      list | test)
        [[ -z "${COMMAND}" ]] || die "Multiple commands: ${COMMAND} and $1"
        COMMAND="$1"
        ;;
      *)
        if [[ "${COMMAND}" == "test" && -z "${COMMAND_ARG}" ]]; then
          COMMAND_ARG="$1"
        else
          die "Unknown argument: $1"
        fi
        ;;
    esac
    shift
  done
  [[ -n "${COMMAND}" ]] || die "No command (try: list or test smoke)"
  if [[ "${COMMAND}" == "test" ]]; then
    [[ -n "${STORAGE_TYPE}" ]] || die "--storage-type is required for test"
    [[ "${STORAGE_TYPE}" == "local" || "${STORAGE_TYPE}" == "minio" ]] || die "Only --storage-type=local or minio is implemented (got: ${STORAGE_TYPE})"
    [[ -n "${COMMAND_ARG}" ]] || die "test requires a name (e.g. smoke)"
  fi
}

list_tests() {
  cat <<'EOF'
Available tests:
  smoke        Upload via rs, cat back, verify each shard has the object on disk.
  verify       Same as smoke, then rsverify check on all shard particles (build: go build -o rsverify ./cmd/rsverify).
  heal         smoke, delete last-shard particle, cat (degraded), backend heal (single-object), rsverify check.
EOF
}

# Object basename used by smoke / verify (must stay in sync).
rs_smoke_object_basename() {
  echo "shell-smoke-hello.txt"
}

run_smoke_test() {
  local test_case="smoke"
  local rclone_bin
  rclone_bin=$(find_rclone_binary)
  # Object name without "/" so each shard stores this filename at its root (matches
  # rclone's fs-relative Remote() passed to shard backends).
  local object_base
  object_base=$(rs_smoke_object_basename)
  local remote_path="${RS_REMOTE}:${object_base}"

  log_info "test:${test_case}" "Removing prior object if present"
  rs_rclone deletefile "${remote_path}" 2>/dev/null || true

  local payload expected
  payload="rs-shell-smoke $(${rclone_bin} version 2>/dev/null | head -n 1 || echo ok)"
  expected="${payload}"

  log_info "test:${test_case}" "rcat → ${remote_path}"
  local out err rc
  out=$(mktemp)
  err=$(mktemp)
  # Do not use `if ! pipeline; then $?` — $? inside `then` is not the pipeline status.
  set +e
  printf '%s' "${payload}" | rs_rclone rcat "${remote_path}" >"${out}" 2>"${err}"
  rc=$?
  set -e
  if [[ "${rc}" -ne 0 ]]; then
    print_if_verbose "rcat" "${out}" "${err}"
    rm -f "${out}" "${err}"
    log_fail "test:${test_case}" "rcat failed (status ${rc})"
    return 1
  fi
  print_if_verbose "rcat" "${out}" "${err}"
  rm -f "${out}" "${err}"

  log_info "test:${test_case}" "cat ← ${remote_path}"
  out=$(mktemp)
  err=$(mktemp)
  set +e
  rs_rclone cat "${remote_path}" >"${out}" 2>"${err}"
  rc=$?
  set -e
  if [[ "${rc}" -ne 0 ]]; then
    print_if_verbose "cat" "${out}" "${err}"
    rm -f "${out}" "${err}"
    log_fail "test:${test_case}" "cat failed (status ${rc})"
    return 1
  fi
  local got
  got=$(cat "${out}")
  print_if_verbose "cat" "${out}" "${err}"
  rm -f "${out}" "${err}"

  if [[ "${got}" != "${expected}" ]]; then
    log_fail "test:${test_case}" "content mismatch"
    return 1
  fi

  local i p shard_remote
  for i in $(seq 1 "${RS_SHARD_TOTAL}"); do
    if [[ "${STORAGE_TYPE}" == "minio" ]]; then
      shard_remote=$(rs_minio_shard_remote_name "${i}")
      p="${shard_remote}:${RS_MINIO_BUCKET}/${object_base}"
      out=$(mktemp)
      err=$(mktemp)
      set +e
      rs_rclone lsl "${p}" >"${out}" 2>"${err}"
      rc=$?
      set -e
      print_if_verbose "shard ${i} lsl" "${out}" "${err}"
      rm -f "${out}" "${err}"
      if [[ "${rc}" -ne 0 ]]; then
        log_fail "test:${test_case}" "missing shard object on ${p}"
        return 1
      fi
    else
      p="${DATA_DIR}/$(printf '%02d' "${i}")_local/${object_base}"
      if [[ ! -f "${p}" ]]; then
        log_fail "test:${test_case}" "missing shard file: ${p}"
        return 1
      fi
    fi
  done

  log_info "test:${test_case}" "backend status"
  out=$(mktemp)
  err=$(mktemp)
  rs_rclone backend status "${RS_REMOTE}:" >"${out}" 2>"${err}" || true
  print_if_verbose "backend status" "${out}" "${err}"
  rm -f "${out}" "${err}"

  log_pass "test:${test_case}" "OK (k=${RS_DATA_SHARDS}, m=${RS_PARITY_SHARDS}, ${RS_SHARD_TOTAL} shards)"
  return 0
}

run_verify_test() {
  local test_case="verify"
  local rsverify_tmp=""
  trap '[[ -n "${rsverify_tmp}" ]] && rm -rf "${rsverify_tmp}"' RETURN

  if ! run_smoke_test; then
    log_fail "test:${test_case}" "smoke step failed"
    return 1
  fi
  local rsverify_bin
  rsverify_bin=$(find_rsverify_binary)
  local object_base
  object_base=$(rs_smoke_object_basename)
  local paths=()
  local i p shard_remote
  if [[ "${STORAGE_TYPE}" == "minio" ]]; then
    rsverify_tmp=$(mktemp -d)
    for i in $(seq 1 "${RS_SHARD_TOTAL}"); do
      shard_remote=$(rs_minio_shard_remote_name "${i}")
      p="${rsverify_tmp}/p${i}"
      if ! rs_rclone copyto "${shard_remote}:${RS_MINIO_BUCKET}/${object_base}" "${p}"; then
        log_fail "test:${test_case}" "failed to download particle from ${shard_remote}"
        return 1
      fi
      paths+=("${p}")
    done
  else
    for i in $(seq 1 "${RS_SHARD_TOTAL}"); do
      p="${DATA_DIR}/$(printf '%02d' "${i}")_local/${object_base}"
      paths+=("${p}")
    done
  fi

  log_info "test:${test_case}" "rsverify check (${#paths[@]} particles)"
  local out err rc
  out=$(mktemp)
  err=$(mktemp)
  set +e
  "${rsverify_bin}" check "${paths[@]}" >"${out}" 2>"${err}"
  rc=$?
  set -e
  print_if_verbose "rsverify check" "${out}" "${err}"
  rm -f "${out}"
  if [[ "${rc}" -ne 0 ]]; then
    log_fail "test:${test_case}" "rsverify check failed (status ${rc})"
    cat "${err}" >&2 || true
    rm -f "${err}"
    return 1
  fi
  rm -f "${err}"
  log_pass "test:${test_case}" "OK (rsverify check)"
  return 0
}

run_heal_test() {
  local test_case="heal"
  local rsverify_tmp=""
  trap '[[ -n "${rsverify_tmp}" ]] && rm -rf "${rsverify_tmp}"' RETURN

  if ! run_smoke_test; then
    log_fail "test:${test_case}" "smoke step failed"
    return 1
  fi
  local rclone_bin
  rclone_bin=$(find_rclone_binary)
  local object_base
  object_base=$(rs_smoke_object_basename)
  local remote_path="${RS_REMOTE}:${object_base}"
  local last_idx="${RS_SHARD_TOTAL}"
  local last_particle
  local last_remote
  if [[ "${STORAGE_TYPE}" == "minio" ]]; then
    last_remote=$(rs_minio_shard_remote_name "${last_idx}")
    last_particle="${last_remote}:${RS_MINIO_BUCKET}/${object_base}"
  else
    last_particle="${DATA_DIR}/$(printf '%02d' "${last_idx}")_local/${object_base}"
  fi

  log_info "test:${test_case}" "Remove last-shard particle: ${last_particle}"
  if [[ "${STORAGE_TYPE}" == "minio" ]]; then
    if ! rs_rclone deletefile "${last_particle}"; then
      log_fail "test:${test_case}" "failed to remove particle on ${last_remote}"
      return 1
    fi
  else
    rm -f "${last_particle}"
    if [[ -f "${last_particle}" ]]; then
      log_fail "test:${test_case}" "failed to remove particle"
      return 1
    fi
  fi

  local payload expected
  payload="rs-shell-smoke $(${rclone_bin} version 2>/dev/null | head -n 1 || echo ok)"
  expected="${payload}"

  log_info "test:${test_case}" "cat (degraded) ← ${remote_path}"
  local out err rc
  out=$(mktemp)
  err=$(mktemp)
  set +e
  rs_rclone cat "${remote_path}" >"${out}" 2>"${err}"
  rc=$?
  set -e
  if [[ "${rc}" -ne 0 ]]; then
    print_if_verbose "cat degraded" "${out}" "${err}"
    rm -f "${out}" "${err}"
    log_fail "test:${test_case}" "cat failed with one shard missing (status ${rc})"
    return 1
  fi
  local got
  got=$(cat "${out}")
  print_if_verbose "cat degraded" "${out}" "${err}"
  rm -f "${out}" "${err}"
  if [[ "${got}" != "${expected}" ]]; then
    log_fail "test:${test_case}" "degraded cat content mismatch"
    return 1
  fi

  log_info "test:${test_case}" "backend heal ${RS_REMOTE}: ${object_base}"
  out=$(mktemp)
  err=$(mktemp)
  set +e
  rs_rclone backend heal "${RS_REMOTE}:" "${object_base}" >"${out}" 2>"${err}"
  rc=$?
  set -e
  print_if_verbose "backend heal" "${out}" "${err}"
  if [[ "${rc}" -ne 0 ]]; then
    rm -f "${out}" "${err}"
    log_fail "test:${test_case}" "backend heal failed (status ${rc})"
    return 1
  fi
  rm -f "${out}" "${err}"

  if [[ "${STORAGE_TYPE}" == "minio" ]]; then
    out=$(mktemp)
    err=$(mktemp)
    set +e
    rs_rclone lsl "${last_particle}" >"${out}" 2>"${err}"
    rc=$?
    set -e
    rm -f "${out}" "${err}"
    if [[ "${rc}" -ne 0 ]]; then
      log_fail "test:${test_case}" "particle not restored on ${last_remote}"
      return 1
    fi
  else
    if [[ ! -f "${last_particle}" ]]; then
      log_fail "test:${test_case}" "particle not restored: ${last_particle}"
      return 1
    fi
  fi

  local rsverify_bin
  rsverify_bin=$(find_rsverify_binary)
  local paths=()
  local i p shard_remote
  if [[ "${STORAGE_TYPE}" == "minio" ]]; then
    rsverify_tmp=$(mktemp -d)
    for i in $(seq 1 "${RS_SHARD_TOTAL}"); do
      shard_remote=$(rs_minio_shard_remote_name "${i}")
      p="${rsverify_tmp}/p${i}"
      if ! rs_rclone copyto "${shard_remote}:${RS_MINIO_BUCKET}/${object_base}" "${p}"; then
        log_fail "test:${test_case}" "failed to download particle from ${shard_remote}"
        return 1
      fi
      paths+=("${p}")
    done
  else
    for i in $(seq 1 "${RS_SHARD_TOTAL}"); do
      p="${DATA_DIR}/$(printf '%02d' "${i}")_local/${object_base}"
      paths+=("${p}")
    done
  fi
  log_info "test:${test_case}" "rsverify check (${#paths[@]} particles)"
  out=$(mktemp)
  err=$(mktemp)
  set +e
  "${rsverify_bin}" check "${paths[@]}" >"${out}" 2>"${err}"
  rc=$?
  set -e
  print_if_verbose "rsverify check" "${out}" "${err}"
  rm -f "${out}"
  if [[ "${rc}" -ne 0 ]]; then
    log_fail "test:${test_case}" "rsverify check failed (status ${rc})"
    cat "${err}" >&2 || true
    rm -f "${err}"
    return 1
  fi
  rm -f "${err}"

  log_pass "test:${test_case}" "OK (drop last shard + heal + rsverify)"
  return 0
}

main() {
  parse_args "$@"
  ensure_workdir

  if [[ "${COMMAND}" == "test" && "${STORAGE_TYPE}" == "minio" ]]; then
    export RS_REMOTE="${RS_REMOTE_MINIO}"
    export RS_SINGLE_REMOTE="${RS_SINGLE_REMOTE_MINIO}"
    export RCLONE_TEST_TIMEOUT="${RCLONE_TEST_TIMEOUT:-120}"
  fi

  ensure_rclone_config

  if [[ "${COMMAND}" == "test" && "${STORAGE_TYPE}" == "minio" ]]; then
    RCLONE_BINARY=$(find_rclone_binary)
    export RCLONE_BINARY
    ensure_minio_rs_containers_ready || die "MinIO is not ready (start Docker or run ./manage.sh start --storage-type=minio)"
  fi

  case "${COMMAND}" in
    list)
      list_tests
      ;;
    test)
      case "${COMMAND_ARG}" in
        smoke)
          if run_smoke_test; then
            exit 0
          fi
          exit 1
          ;;
        verify)
          if run_verify_test; then
            exit 0
          fi
          exit 1
          ;;
        heal)
          if run_heal_test; then
            exit 0
          fi
          exit 1
          ;;
        *)
          die "Unknown test: ${COMMAND_ARG} (try: ./compare.sh list)"
          ;;
      esac
      ;;
  esac
}

main "$@"
