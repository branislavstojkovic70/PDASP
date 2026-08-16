#!/usr/bin/env bash
#
# Shared helpers for every network script.
# Source it with: . "${NETWORK_ROOT}/scripts/utils.sh"

# ---------------------------------------------------------------- colors and output
if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
  C_RESET=$'\033[0m'
  C_RED=$'\033[0;31m'
  C_GREEN=$'\033[0;32m'
  C_YELLOW=$'\033[0;33m'
  C_BLUE=$'\033[0;34m'
  C_GRAY=$'\033[0;90m'
  C_BOLD=$'\033[1m'
else
  C_RESET= C_RED= C_GREEN= C_YELLOW= C_BLUE= C_GRAY= C_BOLD=
fi

info()   { echo "${C_BLUE}==>${C_RESET} $*"; }
ok()     { echo "${C_GREEN} ok ${C_RESET} $*"; }
warn()   { echo "${C_YELLOW} !! ${C_RESET} $*" >&2; }
error()  { echo "${C_RED} XX ${C_RESET}$*" >&2; }
detail() { echo "${C_GRAY}    $*${C_RESET}"; }

header() {
  echo
  echo "${C_BOLD}${C_BLUE}==================================================================${C_RESET}"
  printf "${C_BOLD}${C_BLUE} %s${C_RESET}\n" "$1"
  echo "${C_BOLD}${C_BLUE}==================================================================${C_RESET}"
}

# Aborts with a message. Used instead of a bare `exit 1` so it is always visible
# what exactly failed.
fatal() {
  error "$1"
  exit "${2:-1}"
}

# Checks the exit status of the previous command.
#   check_status $? "message shown on failure"
check_status() {
  if [ "$1" -ne 0 ]; then
    fatal "$2"
  fi
}

# ---------------------------------------------------------------- prerequisites
require_command() {
  command -v "$1" >/dev/null 2>&1 || fatal "Missing '$1'. $2"
}

require_docker() {
  require_command docker "Install Docker Desktop."
  if ! docker info >/dev/null 2>&1; then
    fatal "The Docker daemon is not running. Start Docker Desktop and try again."
  fi
}

# Verifies the Fabric binaries are on PATH.
require_fabric_binaries() {
  local missing=()
  for binary in peer configtxgen osnadmin fabric-ca-client; do
    command -v "$binary" >/dev/null 2>&1 || missing+=("$binary")
  done
  if [ ${#missing[@]} -gt 0 ]; then
    fatal "Missing Fabric binaries: ${missing[*]}. Run ./network/install-fabric.sh"
  fi
}

# ---------------------------------------------------------------- waiting
# wait_for_port <host> <port> <description> [timeout_seconds]
wait_for_port() {
  local host=$1 port=$2 description=$3 timeout=${4:-60}
  local elapsed=0
  detail "waiting for ${description} on ${host}:${port} ..."
  while ! nc -z "$host" "$port" >/dev/null 2>&1; do
    sleep 1
    elapsed=$((elapsed + 1))
    if [ "$elapsed" -ge "$timeout" ]; then
      fatal "${description} did not come up on ${host}:${port} within ${timeout}s"
    fi
  done
  ok "${description} is reachable (${host}:${port})"
}

# wait_for_healthy <container_name> [timeout_seconds]
# Waits for the container's docker healthcheck to report "healthy".
wait_for_healthy() {
  local container=$1 timeout=${2:-90}
  local elapsed=0 status
  while true; do
    status="$(docker inspect --format '{{.State.Health.Status}}' "$container" 2>/dev/null || echo missing)"
    case "$status" in
      healthy) ok "${container} is healthy"; return 0 ;;
      missing) fatal "Container ${container} does not exist" ;;
    esac
    sleep 1
    elapsed=$((elapsed + 1))
    if [ "$elapsed" -ge "$timeout" ]; then
      error "${container} did not become healthy within ${timeout}s (status: ${status})"
      docker logs --tail 20 "$container" >&2 || true
      exit 1
    fi
  done
}

# wait_for_http <url> <description> [timeout_seconds]
wait_for_http() {
  local url=$1 description=$2 timeout=${3:-60}
  local elapsed=0
  detail "waiting for ${description} at ${url} ..."
  while ! curl -sk --max-time 2 "$url" >/dev/null 2>&1; do
    sleep 1
    elapsed=$((elapsed + 1))
    if [ "$elapsed" -ge "$timeout" ]; then
      fatal "${description} did not answer at ${url} within ${timeout}s"
    fi
  done
  ok "${description} answers (${url})"
}

# ---------------------------------------------------------------- configuration
# Loads network/.env into the environment, exporting every variable.
load_env() {
  local env_file="${NETWORK_ROOT}/.env"
  [ -f "$env_file" ] || fatal "No such file: ${env_file}"
  set -a
  # shellcheck disable=SC1090
  . "$env_file"
  set +a
}
