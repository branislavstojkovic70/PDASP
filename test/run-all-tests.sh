#!/usr/bin/env bash
#
# Runs the whole test suite against a running network.
#
#   ./test/run-all-tests.sh              run everything
#   ./test/run-all-tests.sh 05 08        run only the scripts whose number matches
#
# Every script drives the console application the way a user would, so a passing
# run exercises the SDK, the gateway, endorsement by two of three organizations,
# ordering, commit and the CouchDB state database.
#
# Prerequisites:
#   ./network/network-up.sh
#   ./network/create-channels.sh
#   ./network/deploy-chaincode.sh

set -uo pipefail

TEST_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${TEST_DIR}/.." && pwd)"

# Shared across every script, so all records created by one run carry the same
# suffix and can be told apart from earlier runs.
export RUN_ID="${RUN_ID:-$(date +%H%M%S)}"

if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
  R=$'\033[0m'; RED=$'\033[0;31m'; GREEN=$'\033[0;32m'; BLUE=$'\033[0;34m'; BOLD=$'\033[1m'
else
  R= RED= GREEN= BLUE= BOLD=
fi

# ---------------------------------------------------------------- selection
scripts=()
if [ $# -gt 0 ]; then
  for pattern in "$@"; do
    while IFS= read -r script; do
      scripts+=("${script}")
    done < <(find "${TEST_DIR}" -maxdepth 1 -name "test-${pattern}*.sh" | sort)
  done
  if [ ${#scripts[@]} -eq 0 ]; then
    echo "${RED}No test script matches: $*${R}" >&2
    exit 1
  fi
else
  while IFS= read -r script; do
    scripts+=("${script}")
  done < <(find "${TEST_DIR}" -maxdepth 1 -name 'test-*.sh' | sort)
fi

# ---------------------------------------------------------------- prerequisites
echo
echo "${BOLD}${BLUE}==================================================================${R}"
echo "${BOLD}${BLUE} PDASP test suite, run ${RUN_ID}${R}"
echo "${BOLD}${BLUE}==================================================================${R}"

for tool in node jq docker; do
  command -v "${tool}" >/dev/null 2>&1 || { echo "${RED}Missing tool: ${tool}${R}" >&2; exit 1; }
done

if ! docker ps --filter 'label=pdasp.role=peer' --format '{{.Names}}' | grep -q peer0.org1; then
  echo "${RED}The network is not running. Start it with:${R}" >&2
  echo "  ./network/network-up.sh && ./network/create-channels.sh && ./network/deploy-chaincode.sh" >&2
  exit 1
fi

if [ ! -d "${REPO_ROOT}/application/node_modules" ]; then
  echo "${BLUE}Installing the console application dependencies${R}"
  (cd "${REPO_ROOT}/application" && npm install --no-audit --no-fund) >/dev/null
fi

# ---------------------------------------------------------------- run
started=$(date +%s)
declare -a results=()
failed=0

for script in "${scripts[@]}"; do
  name="$(basename "${script}")"
  if bash "${script}"; then
    results+=("${GREEN} ok ${R} ${name}")
  else
    results+=("${RED} XX ${R} ${name}")
    failed=$((failed + 1))
  fi
done

# ---------------------------------------------------------------- summary
echo
echo "${BOLD}${BLUE}==================================================================${R}"
echo "${BOLD}${BLUE} Summary${R}"
echo "${BOLD}${BLUE}==================================================================${R}"
for line in "${results[@]}"; do
  echo "${line}"
done

echo
echo "Scripts: ${#scripts[@]}, failed: ${failed}, elapsed: $(( $(date +%s) - started ))s"

if [ "${failed}" -gt 0 ]; then
  echo "${RED}${BOLD}TEST SUITE FAILED${R}"
  exit 1
fi
echo "${GREEN}${BOLD}TEST SUITE PASSED${R}"
