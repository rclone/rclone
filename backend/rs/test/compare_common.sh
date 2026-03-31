# Shared helpers for rs comparison scripts.
# shellcheck shell=bash

# Native Windows guard (same idea as raid3/compare_common.sh)
if [[ -n "${WINDIR:-}" ]] || [[ -n "${SYSTEMROOT:-}" ]]; then
  if [[ "${OSTYPE:-}" != "msys" ]] && [[ "${OSTYPE:-}" != "cygwin" ]] && [[ ! -f /proc/version ]] && [[ ! -d /usr/bin ]]; then
    cat >&2 <<'EOF'
ERROR: These integration test scripts require a Unix-like shell (Linux, macOS, WSL, Git Bash, Cygwin).
EOF
    exit 1
  fi
fi

SCRIPT_DIR="${SCRIPT_DIR:-$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)}"
DATA_DIR="${DATA_DIR:-${SCRIPT_DIR}/_data}"

if [[ ! -f "${SCRIPT_DIR}/compare_rs_env.sh" ]]; then
  printf '[%s] ERROR: Missing compare_rs_env.sh in %s\n' "${SCRIPT_NAME:-compare_common.sh}" "${SCRIPT_DIR}" >&2
  exit 1
fi
# shellcheck source=/dev/null
. "${SCRIPT_DIR}/compare_rs_env.sh"

if [[ -f "${SCRIPT_DIR}/compare_rs_env.local.sh" ]]; then
  # shellcheck source=/dev/null
  . "${SCRIPT_DIR}/compare_rs_env.local.sh"
fi

# Recompute after optional local overrides
RS_SHARD_TOTAL=$((RS_DATA_SHARDS + RS_PARITY_SHARDS))

TEST_SPECIFIC_CONFIG="${SCRIPT_DIR}/tests.config"
RCLONE_CONFIG="${TEST_SPECIFIC_CONFIG}"

VERBOSE="${VERBOSE:-0}"

# Caller (compare.sh, compare_all.sh, manage.sh) sets SCRIPT_NAME; setup.sh does not.
if [[ -z "${SCRIPT_NAME:-}" ]]; then
  if [[ "${#BASH_SOURCE[@]}" -ge 2 ]]; then
    SCRIPT_NAME=$(basename "${BASH_SOURCE[1]}")
  else
    SCRIPT_NAME=$(basename "${BASH_SOURCE[0]}")
  fi
fi

log_tag() {
  local tag="$1"
  shift
  printf '[%s] %s %s\n' "${SCRIPT_NAME}" "${tag}" "$*"
}

log_info() {
  log_tag "INFO" "$*"
}

log_warn() {
  log_tag "WARN" "$*"
}

log_pass() {
  log_tag "PASS" "$*"
}

log_fail() {
  log_tag "FAIL" "$*"
}

die() {
  local prefix="[${SCRIPT_NAME}] ERROR:"
  for msg in "$@"; do
    printf '%s %s\n' "${prefix}" "${msg}" >&2
    prefix="[${SCRIPT_NAME}]"
  done
  exit 1
}

ensure_workdir() {
  if [[ ! -d "${SCRIPT_DIR}" ]]; then
    die "Integration test directory does not exist: ${SCRIPT_DIR}"
  fi
  if [[ "${PWD}" != "${SCRIPT_DIR}" ]]; then
    die "This script must be run from ${SCRIPT_DIR} (current: ${PWD})" \
      "Please: cd ${SCRIPT_DIR}"
  fi
}

ensure_rclone_config() {
  if [[ ! -f "${RCLONE_CONFIG}" ]]; then
    die "Missing ${RCLONE_CONFIG}" "" "Run ./setup.sh in ${SCRIPT_DIR} first."
  fi
  log_info "config" "Using rclone config: ${RCLONE_CONFIG}"
}

# Paths under DATA_DIR as relative to SCRIPT_DIR (for portable tests.config)
make_relative_path() {
  local abs_path="$1"
  local test_dir="${SCRIPT_DIR}"
  if [[ "${abs_path}" == "${test_dir}"/* ]]; then
    echo "${abs_path#"${test_dir}"/}"
  else
    echo "${abs_path}"
  fi
}

find_rclone_binary() {
  if [[ -n "${RCLONE_BINARY:-}" ]] && [[ -x "${RCLONE_BINARY}" ]]; then
    echo "${RCLONE_BINARY}"
    return 0
  fi
  local repo_root
  repo_root=$(cd "${SCRIPT_DIR}/../../.." && pwd)
  # Prefer the repo-built binary so optional backends (e.g. rs) are linked in.
  if [[ -x "${repo_root}/rclone" ]]; then
    echo "${repo_root}/rclone"
    return 0
  fi
  if command -v rclone >/dev/null 2>&1; then
    command -v rclone
    return 0
  fi
  die "rclone binary not found (build ./rclone in repo root or set RCLONE_BINARY)"
}

find_rsverify_binary() {
  if [[ -n "${RSVERIFY_BINARY:-}" ]] && [[ -x "${RSVERIFY_BINARY}" ]]; then
    echo "${RSVERIFY_BINARY}"
    return 0
  fi
  local repo_root
  repo_root=$(cd "${SCRIPT_DIR}/../../.." && pwd)
  if [[ -x "${repo_root}/rsverify" ]]; then
    echo "${repo_root}/rsverify"
    return 0
  fi
  if command -v rsverify >/dev/null 2>&1; then
    command -v rsverify
    return 0
  fi
  die "rsverify not found (go build -o rsverify ./cmd/rsverify in repo root, or set RSVERIFY_BINARY)"
}

# MinIO test containers: name|user|pass|host_s3_port|host_console_port|data_dir
MINIO_RS_CONTAINERS=()

rs_minio_shard_remote_name() {
  printf '%s%02d' "${MINIO_RS_SHARD_REMOTE_PREFIX}" "$1"
}

# create_rs_rclone_config <config_file> <force 0|1>
create_rs_rclone_config() {
  local config_file="${1:-${TEST_SPECIFIC_CONFIG}}"
  local force="${2:-0}"

  if [[ -f "${config_file}" && "${force}" -eq 0 ]]; then
    log_warn "config" "Config already exists: ${config_file} (skipping)"
    return 1
  fi

  local config_dir
  config_dir=$(dirname "${config_file}")
  mkdir -p "${config_dir}" || die "Cannot create ${config_dir}"

  if [[ "${RS_SHARD_TOTAL}" -lt 2 ]]; then
    die "RS_SHARD_TOTAL must be >= 2 (data_shards + parity_shards)"
  fi
  if [[ "${RS_DATA_SHARDS}" -lt 1 ]] || [[ "${RS_PARITY_SHARDS}" -lt 1 ]]; then
    die "RS_DATA_SHARDS and RS_PARITY_SHARDS must be >= 1 for this harness"
  fi

  local single_rel
  single_rel=$(make_relative_path "${DATA_DIR}/single_local")

  local remotes_line=""
  local i name rel
  for i in $(seq 1 "${RS_SHARD_TOTAL}"); do
    name=$(printf 'localrs%02d' "${i}")
    rel="_data/$(printf '%02d' "${i}")_local"
    [[ -n "${remotes_line}" ]] && remotes_line+=","
    remotes_line+="${name}:${rel}"
  done

  local minio_remotes_line=""
  local shard_remote
  for i in $(seq 1 "${RS_SHARD_TOTAL}"); do
    shard_remote=$(rs_minio_shard_remote_name "${i}")
    [[ -n "${minio_remotes_line}" ]] && minio_remotes_line+=","
    minio_remotes_line+="${shard_remote}:${RS_MINIO_BUCKET}"
  done

  log_info "config" "Writing ${config_file} (local + MinIO, k=${RS_DATA_SHARDS}, m=${RS_PARITY_SHARDS}, shards=${RS_SHARD_TOTAL})"

  {
    echo "# rclone configuration for rs integration tests"
    echo "# Generated: $(date -u +"%Y-%m-%d %H:%M:%S UTC")"
    echo "# Paths are relative to this test directory: ${SCRIPT_DIR}"
    echo "# Local remotes: --storage-type=local. MinIO remotes: --storage-type=minio (ports ${MINIO_RS_FIRST_S3_PORT}-$((MINIO_RS_FIRST_S3_PORT + RS_SHARD_TOTAL)), bucket ${RS_MINIO_BUCKET})."
    echo ""
    for i in $(seq 1 "${RS_SHARD_TOTAL}"); do
      name=$(printf 'localrs%02d' "${i}")
      echo "[${name}]"
      echo "type = local"
      echo ""
    done
    echo "[${RS_REMOTE}]"
    echo "type = rs"
    echo "remotes = ${remotes_line}"
    echo "data_shards = ${RS_DATA_SHARDS}"
    echo "parity_shards = ${RS_PARITY_SHARDS}"
    echo "use_spooling = true"
    echo ""
    echo "[${RS_SINGLE_REMOTE}]"
    echo "type = alias"
    echo "remote = ${single_rel}"
    echo ""
    echo "# --- MinIO backends (Docker; use --storage-type=minio) ---"
    echo "# S3 API host ports: ${MINIO_RS_FIRST_S3_PORT}-$((MINIO_RS_FIRST_S3_PORT + RS_SHARD_TOTAL)) (see compare_rs_env.sh)."
    echo ""
    for i in $(seq 1 "${RS_SHARD_TOTAL}"); do
      name=$(rs_minio_shard_remote_name "${i}")
      echo "[${name}]"
      echo "type = s3"
      echo "provider = Minio"
      echo "env_auth = false"
      echo "access_key_id = ${MINIO_RS_USER}"
      echo "secret_access_key = ${MINIO_RS_PASS}"
      echo "endpoint = http://127.0.0.1:$((MINIO_RS_FIRST_S3_PORT + i - 1))"
      echo "acl = private"
      echo "no_check_bucket = false"
      echo "max_retries = 1"
      echo "upload_cutoff = 5G"
      echo ""
    done
    echo "[${MINIO_RS_SINGLE_REMOTE_NAME}]"
    echo "type = s3"
    echo "provider = Minio"
    echo "env_auth = false"
    echo "access_key_id = ${MINIO_RS_USER}"
    echo "secret_access_key = ${MINIO_RS_PASS}"
    echo "endpoint = http://127.0.0.1:$((MINIO_RS_FIRST_S3_PORT + RS_SHARD_TOTAL))"
    echo "acl = private"
    echo "no_check_bucket = false"
    echo "max_retries = 1"
    echo "upload_cutoff = 5G"
    echo ""
    echo "[${RS_REMOTE_MINIO}]"
    echo "type = rs"
    echo "remotes = ${minio_remotes_line}"
    echo "data_shards = ${RS_DATA_SHARDS}"
    echo "parity_shards = ${RS_PARITY_SHARDS}"
    echo "use_spooling = true"
    echo ""
    echo "[${RS_SINGLE_REMOTE_MINIO}]"
    echo "type = alias"
    echo "remote = ${MINIO_RS_SINGLE_REMOTE_NAME}:${RS_MINIO_BUCKET}"
    echo ""
  } >"${config_file}" || die "Failed to write ${config_file}"

  log_pass "config" "Created ${config_file}"
  return 0
}

rs_minio_build_container_table() {
  MINIO_RS_CONTAINERS=()
  local i s3 console name
  for i in $(seq 1 "${RS_SHARD_TOTAL}"); do
    s3=$((MINIO_RS_FIRST_S3_PORT + i - 1))
    console=$((MINIO_RS_FIRST_CONSOLE_PORT + i - 1))
    name=$(printf '%s%02d' "${MINIO_RS_CONTAINER_PREFIX}" "${i}")
    MINIO_RS_CONTAINERS+=("${name}|${MINIO_RS_USER}|${MINIO_RS_PASS}|${s3}|${console}|${DATA_DIR}/$(printf '%02d' "${i}")_minio")
  done
  s3=$((MINIO_RS_FIRST_S3_PORT + RS_SHARD_TOTAL))
  console=$((MINIO_RS_FIRST_CONSOLE_PORT + RS_SHARD_TOTAL))
  MINIO_RS_CONTAINERS+=("${MINIO_RS_SINGLE_CONTAINER_NAME}|${MINIO_RS_USER}|${MINIO_RS_PASS}|${s3}|${console}|${DATA_DIR}/single_minio")
}

check_docker_available() {
  if ! command -v docker >/dev/null 2>&1; then
    die "Docker is required for --storage-type=minio." \
      "Docker was not found in PATH."
  fi
  if ! docker info >/dev/null 2>&1; then
    local err
    err=$(docker info 2>&1 | tail -1) || err="unknown"
    die "Cannot connect to the Docker daemon. Ensure Docker is running." \
      "Error: ${err}"
  fi
}

container_exists() {
  local name="$1"
  docker ps -a --format '{{.Names}}' | grep -Fxq "${name}"
}

container_running() {
  local name="$1"
  docker ps --format '{{.Names}}' | grep -Fxq "${name}"
}

ensure_directory() {
  local dir="$1"
  if [[ ! -d "${dir}" ]]; then
    mkdir -p "${dir}"
  fi
}

wait_for_minio_port() {
  local port="$1"
  local retries=60
  local delay=1
  while (( retries > 0 )); do
    if nc -z localhost "${port}" >/dev/null 2>&1; then
      return 0
    fi
    sleep "${delay}"
    ((retries--)) || true
  done
  return 1
}

# Wait until MinIO accepts S3 list on the given shard remote (requires RCLONE_CONFIG and RCLONE_BINARY).
wait_for_minio_rs_shard_remote_ready() {
  local remote="$1"
  local retries=30
  local delay=1
  local save_timeout="${RCLONE_TEST_TIMEOUT:-}"
  export RCLONE_TEST_TIMEOUT="${MINIO_READINESS_TIMEOUT:-20}"
  while (( retries > 0 )); do
    local output status
    output=$("${RCLONE_BINARY}" --config "${RCLONE_CONFIG}" --retries 1 ls "${remote}:${RS_MINIO_BUCKET}" 2>&1)
    status=$?
    if [[ ${status} -eq 0 ]]; then
      if [[ -n "${save_timeout}" ]]; then export RCLONE_TEST_TIMEOUT="${save_timeout}"; else unset RCLONE_TEST_TIMEOUT; fi
      return 0
    fi
    if echo "${output}" | grep -qiE "(directory not found|bucket.*not found|NoSuchBucket)"; then
      if [[ -n "${save_timeout}" ]]; then export RCLONE_TEST_TIMEOUT="${save_timeout}"; else unset RCLONE_TEST_TIMEOUT; fi
      return 0
    fi
    if echo "${output}" | grep -qiE "(connection reset|connection refused|no route to host|timeout|deadline exceeded|InternalError|0 drives provided|StatusCode: 500)"; then
      sleep "${delay}"
      ((retries--)) || true
      continue
    fi
    if [[ -n "${save_timeout}" ]]; then export RCLONE_TEST_TIMEOUT="${save_timeout}"; else unset RCLONE_TEST_TIMEOUT; fi
    return 0
  done
  if [[ -n "${save_timeout}" ]]; then export RCLONE_TEST_TIMEOUT="${save_timeout}"; else unset RCLONE_TEST_TIMEOUT; fi
  return 1
}

start_minio_rs_containers() {
  rs_minio_build_container_table
  local entry
  for entry in "${MINIO_RS_CONTAINERS[@]}"; do
    IFS='|' read -r name user pass s3_port console_port data_dir <<<"${entry}"
    ensure_directory "${data_dir}"

    if container_running "${name}"; then
      log_info "docker" "Container '${name}' already running — skipping."
      continue
    fi

    if container_exists "${name}"; then
      log_info "docker" "Starting existing container '${name}'."
      docker start "${name}" >/dev/null
      continue
    fi

    log_info "docker" "Launching container '${name}' (ports ${s3_port}/${console_port})."
    local data_dir_abs
    data_dir_abs=$(cd "${data_dir}" && pwd) || data_dir_abs="${data_dir}"
    docker run -d \
      --name "${name}" \
      -p "${s3_port}:9000" \
      -p "${console_port}:9001" \
      -e "MINIO_ROOT_USER=${user}" \
      -e "MINIO_ROOT_PASSWORD=${pass}" \
      -v "${data_dir_abs}:/data" \
      "${MINIO_IMAGE}" server /data --console-address ":9001" >/dev/null
  done
}

stop_minio_rs_containers() {
  rs_minio_build_container_table
  local entry
  for entry in "${MINIO_RS_CONTAINERS[@]}"; do
    IFS='|' read -r name _ <<<"${entry}"
    if container_running "${name}"; then
      log_info "docker" "Stopping container '${name}'."
      docker stop "${name}" >/dev/null
    fi
  done
}

remove_minio_rs_containers() {
  rs_minio_build_container_table
  local entry
  for entry in "${MINIO_RS_CONTAINERS[@]}"; do
    IFS='|' read -r name _ <<<"${entry}"
    if container_running "${name}"; then
      docker stop "${name}" >/dev/null
    fi
    if container_exists "${name}"; then
      docker rm "${name}" >/dev/null
    fi
  done
}

ensure_minio_rs_containers_ready() {
  check_docker_available
  : "${RCLONE_BINARY:=$(find_rclone_binary)}"
  export RCLONE_BINARY

  rs_minio_build_container_table
  local entry started=0
  for entry in "${MINIO_RS_CONTAINERS[@]}"; do
    IFS='|' read -r name _ _ _ _ data_dir <<<"${entry}"
    ensure_directory "${data_dir}"
    if container_running "${name}"; then
      log_info "autostart" "Container '${name}' already running."
      continue
    fi
    started=1
    if container_exists "${name}"; then
      log_info "autostart" "Starting container '${name}'."
      docker start "${name}" >/dev/null || return 1
    else
      log_info "autostart" "Container '${name}' missing; launching MinIO set."
      start_minio_rs_containers
      started=0
      break
    fi
  done

  if ((started)); then
    sleep 3
  fi

  local idx=0
  for entry in "${MINIO_RS_CONTAINERS[@]}"; do
    IFS='|' read -r name _ _ s3_port _ <<<"${entry}"
    if ! container_running "${name}"; then
      log_fail "autostart" "Container ${name} is not running. Run: docker logs ${name}"
      return 1
    fi
    log_info "autostart" "Waiting for ${name} (port ${s3_port})..."
    if ! wait_for_minio_port "${s3_port}"; then
      log_fail "autostart" "Port ${s3_port} for ${name} did not open in time."
      return 1
    fi
    ((idx++)) || true
  done

  log_info "autostart" "Ensuring S3 buckets exist (${RS_MINIO_BUCKET})..."
  local i r
  for i in $(seq 1 "${RS_SHARD_TOTAL}"); do
    r=$(rs_minio_shard_remote_name "${i}")
    "${RCLONE_BINARY}" --config "${RCLONE_CONFIG}" mkdir "${r}:${RS_MINIO_BUCKET}" 2>/dev/null || true
  done
  "${RCLONE_BINARY}" --config "${RCLONE_CONFIG}" mkdir "${MINIO_RS_SINGLE_REMOTE_NAME}:${RS_MINIO_BUCKET}" 2>/dev/null || true

  log_info "autostart" "Waiting for MinIO S3 API (rclone ls per shard)..."
  for i in $(seq 1 "${RS_SHARD_TOTAL}"); do
    r=$(rs_minio_shard_remote_name "${i}")
    if ! wait_for_minio_rs_shard_remote_ready "${r}"; then
      log_fail "autostart" "MinIO remote '${r}' did not become ready."
      return 1
    fi
  done
  if ! wait_for_minio_rs_shard_remote_ready "${MINIO_RS_SINGLE_REMOTE_NAME}"; then
    log_fail "autostart" "MinIO single remote did not become ready."
    return 1
  fi

  log_info "autostart" "MinIO containers are ready."
  return 0
}
