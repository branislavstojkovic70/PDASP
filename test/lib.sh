#!/usr/bin/env bash
#
# Shared helpers for the test scripts.
#
# Every test drives the console application exactly the way a user would, so what
# is being tested is the whole path: SDK, gateway, endorsement, ordering, commit
# and the CouchDB state database.
#
# Source it with: . "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

set -uo pipefail

TEST_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${TEST_DIR}/.." && pwd)"
APP="${REPO_ROOT}/application/src/index.js"

# Every run uses ids derived from the clock, so the suite can be run repeatedly
# against the same ledger without colliding with its own earlier records.
RUN_ID="${RUN_ID:-$(date +%H%M%S)}"

# Defaults, overridable per test.
ORG="${ORG:-org1}"
USER_ID="${USER_ID:-org1user1}"
CHANNEL="${CHANNEL:-channel1}"

PASSED=0
FAILED=0
FAILURES=()

# ---------------------------------------------------------------- output
if [ -t 2 ] && [ -z "${NO_COLOR:-}" ]; then
  T_RESET=$'\033[0m'; T_RED=$'\033[0;31m'; T_GREEN=$'\033[0;32m'
  T_YELLOW=$'\033[0;33m'; T_BLUE=$'\033[0;34m'; T_BOLD=$'\033[1m'
else
  T_RESET= T_RED= T_GREEN= T_YELLOW= T_BLUE= T_BOLD=
fi

suite() {
  echo >&2
  echo "${T_BOLD}${T_BLUE}== $* ==${T_RESET}" >&2
}

step() { echo "${T_BLUE}--${T_RESET} $*" >&2; }

pass() {
  PASSED=$((PASSED + 1))
  echo "${T_GREEN} ok ${T_RESET} $*" >&2
}

fail() {
  FAILED=$((FAILED + 1))
  FAILURES+=("$*")
  echo "${T_RED} XX ${T_RESET} $*" >&2
}

skip() { echo "${T_YELLOW} -- ${T_RESET} skipped: $*" >&2; }

# ---------------------------------------------------------------- running the app
# app <command> [args...]
#
# Runs the console application with the current identity and channel, and prints
# the JSON result on stdout. Notes written by the application go to stderr, which
# is why the caller can pipe this straight into jq.
app() {
  node "${APP}" "$@" --org "${ORG}" --user "${USER_ID}" --channel "${CHANNEL}" --compact
}

# app_plain <command> [args...] runs without the connection flags, for commands
# such as `identities` that do not touch the ledger.
app_plain() {
  node "${APP}" "$@" --compact
}

# app_expect_failure <command> [args...]
#
# Runs a command that must fail and prints the error message on stdout. Used for
# the error handling tests, which the assignment asks for explicitly.
app_expect_failure() {
  local output status
  output=$(node "${APP}" "$@" --org "${ORG}" --user "${USER_ID}" --channel "${CHANNEL}" 2>&1 >/dev/null)
  status=$?
  if [ "${status}" -eq 0 ]; then
    echo "COMMAND_UNEXPECTEDLY_SUCCEEDED"
    return 1
  fi
  echo "${output}"
}

# ---------------------------------------------------------------- assertions
assert_eq() {
  local expected=$1 actual=$2 description=$3
  if [ "${expected}" = "${actual}" ]; then
    pass "${description}"
  else
    fail "${description}: expected '${expected}', got '${actual}'"
  fi
}

assert_ne() {
  local unexpected=$1 actual=$2 description=$3
  if [ "${unexpected}" != "${actual}" ]; then
    pass "${description}"
  else
    fail "${description}: value should not have been '${unexpected}'"
  fi
}

assert_contains() {
  local haystack=$1 needle=$2 description=$3
  if echo "${haystack}" | grep -qF "${needle}"; then
    pass "${description}"
  else
    fail "${description}: '${needle}' not found in: $(echo "${haystack}" | head -c 200)"
  fi
}

assert_not_empty() {
  local value=$1 description=$2
  if [ -n "${value}" ] && [ "${value}" != "null" ] && [ "${value}" != "[]" ]; then
    pass "${description}"
  else
    fail "${description}: value was empty"
  fi
}

# assert_json <json> <jq_expression> <expected> <description>
assert_json() {
  local json=$1 expression=$2 expected=$3 description=$4
  local actual
  actual=$(echo "${json}" | jq -r "${expression}" 2>/dev/null)
  assert_eq "${expected}" "${actual}" "${description}"
}

# assert_numeric_eq compares two numbers, tolerating 1 vs 1.0 style differences.
assert_numeric_eq() {
  local expected=$1 actual=$2 description=$3
  if awk -v a="${expected}" -v b="${actual}" 'BEGIN { exit (a - b < 0.005 && b - a < 0.005) ? 0 : 1 }'; then
    pass "${description}"
  else
    fail "${description}: expected ${expected}, got ${actual}"
  fi
}

# ---------------------------------------------------------------- prerequisites
require_tools() {
  local missing=()
  command -v node >/dev/null 2>&1 || missing+=(node)
  command -v jq >/dev/null 2>&1 || missing+=(jq)
  if [ ${#missing[@]} -gt 0 ]; then
    echo "${T_RED}Missing tools: ${missing[*]}${T_RESET}" >&2
    exit 1
  fi
  if [ ! -f "${APP}" ]; then
    echo "${T_RED}Console application not found at ${APP}${T_RESET}" >&2
    exit 1
  fi
}

# Ensures the wallet holds the identities the suite acts as.
require_identities() {
  if [ "$(app_plain identities | jq 'length')" -lt 9 ]; then
    step "wallet is incomplete, enrolling the standard identities"
    app_plain bootstrap >/dev/null || {
      echo "${T_RED}Enrolment failed. Is the network up? ./network/network-up.sh${T_RESET}" >&2
      exit 1
    }
  fi
}

# ---------------------------------------------------------------- summary
summary() {
  local title=$1
  echo >&2
  if [ "${FAILED}" -eq 0 ]; then
    echo "${T_GREEN}${T_BOLD}${title}: ${PASSED} passed${T_RESET}" >&2
    return 0
  fi

  echo "${T_RED}${T_BOLD}${title}: ${PASSED} passed, ${FAILED} failed${T_RESET}" >&2
  for failure in "${FAILURES[@]}"; do
    echo "${T_RED}  - ${failure}${T_RESET}" >&2
  done
  return 1
}
