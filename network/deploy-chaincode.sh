#!/usr/bin/env bash
#
# Deploys the 'trade' chaincode to both channels through the Fabric 2.x lifecycle:
#
#   package -> install (all 9 peers) -> approveformyorg (3 orgs, per channel)
#           -> checkcommitreadiness -> commit (per channel) -> InitLedger
#
# Installation is bound to a peer while approval and commit are bound to a channel,
# so the package is installed once and approved and committed twice.
#
# Re-running acts as an upgrade: the script reads the sequence currently committed
# on the channel and continues with the next one.
#
# Prerequisites: ./network-up.sh && ./create-channels.sh

set -euo pipefail

NETWORK_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "${NETWORK_ROOT}/scripts/utils.sh"
load_env
. "${NETWORK_ROOT}/scripts/env-var.sh"

export PATH="${NETWORK_ROOT}/bin:$PATH"
export FABRIC_CFG_PATH="${NETWORK_ROOT}/config"

require_docker
require_fabric_binaries
require_command jq "Install jq (brew install jq)."

CHAINCODE_SRC="$(cd "${NETWORK_ROOT}/${CHAINCODE_PATH}" && pwd)"
PACKAGE="${NETWORK_ROOT}/${CHAINCODE_NAME}.tar.gz"
LABEL="${CHAINCODE_NAME}_${CHAINCODE_VERSION}"
ORDERER="localhost:${ORDERER1_PORT}"
ORDERER_NAME="orderer.${DOMAIN}"

PACKAGE_ID=""
SEQUENCE=""

# Endorser arguments reused by commit and invoke.
ENDORSER_ARGS=()
one_peer_per_org_args ENDORSER_ARGS

# ---------------------------------------------------------------------------
step_vendor() {
  header "1/6  Preparing the source"

  require_command go "Install Go (brew install go)."

  # The peer builds Go chaincode inside a fabric-ccenv container. Without a
  # vendor/ directory that build has to fetch dependencies over the network, which
  # is slow and fragile, so the dependencies are packaged with the source.
  info "go mod vendor"
  (cd "${CHAINCODE_SRC}" && go mod vendor >/dev/null 2>&1) \
    || fatal "go mod vendor failed in ${CHAINCODE_SRC}"
  ok "dependencies vendored ($(du -sh "${CHAINCODE_SRC}/vendor" | cut -f1 | tr -d ' '))"

  info "go test"
  (cd "${CHAINCODE_SRC}" && go test ./... >/dev/null 2>&1) \
    || fatal "chaincode unit tests fail, deployment aborted"
  ok "unit tests pass"
}

# ---------------------------------------------------------------------------
step_package() {
  header "2/6  Packaging"

  rm -f "${PACKAGE}"
  peer lifecycle chaincode package "${PACKAGE}" \
    --path "${CHAINCODE_SRC}" \
    --lang "${CHAINCODE_LANG}" \
    --label "${LABEL}" \
    || fatal "chaincode packaging failed"

  ok "${CHAINCODE_NAME}.tar.gz ($(du -h "${PACKAGE}" | cut -f1 | tr -d ' ')), label ${LABEL}"

  # The CouchDB indexes must be inside the package, otherwise rich queries run
  # without an index. They sit next to code.tar.gz at the root of the outer archive.
  local index_count
  index_count=$(tar -tzf "${PACKAGE}" 2>/dev/null \
    | grep -c 'META-INF/statedb/couchdb/indexes/.*\.json' || true)
  if [ "${index_count}" -gt 0 ]; then
    ok "${index_count} CouchDB indexes are in the package"
  else
    fatal "no CouchDB indexes in the package, check chaincode/trade/META-INF"
  fi
}

# ---------------------------------------------------------------------------
step_install() {
  header "3/6  Installing on all 9 peers"

  local org p output
  for org in 1 2 3; do
    for p in 0 1 2; do
      set_peer "${org}" "${p}"

      if peer lifecycle chaincode queryinstalled --output json 2>/dev/null \
         | jq -e --arg label "${LABEL}" \
             '.installed_chaincodes // [] | any(.label == $label)' >/dev/null; then
        detail "peer${p}.org${org}: already installed"
        continue
      fi

      if ! output=$(peer lifecycle chaincode install "${PACKAGE}" 2>&1); then
        error "installation on peer${p}.org${org} failed"
        echo "${output}" | tail -5 >&2
        exit 1
      fi
      detail "peer${p}.org${org}: installed"
    done
  done

  # The package ID is a hash of the package and is identical on every peer.
  set_peer 1 0
  PACKAGE_ID=$(peer lifecycle chaincode queryinstalled --output json \
    | jq -r --arg label "${LABEL}" \
        '.installed_chaincodes[] | select(.label == $label) | .package_id')
  [ -n "${PACKAGE_ID}" ] || fatal "no package ID found for label ${LABEL}"
  ok "package ID: ${PACKAGE_ID}"
}

# ---------------------------------------------------------------------------
# determine_sequence <channel>
#
# The first deployment is sequence 1. Every later one must be exactly one higher
# than the currently committed sequence, otherwise Fabric rejects the definition.
# ---------------------------------------------------------------------------
determine_sequence() {
  local channel=$1
  set_peer 1 0

  local current
  current=$(peer lifecycle chaincode querycommitted \
    --channelID "${channel}" --name "${CHAINCODE_NAME}" --output json 2>/dev/null \
    | jq -r '.sequence // empty' || true)

  if [ -z "${current}" ]; then
    SEQUENCE=1
    detail "${channel}: first deployment, sequence 1"
  else
    SEQUENCE=$((current + 1))
    detail "${channel}: existing sequence ${current}, new sequence ${SEQUENCE}"
  fi
}

# ---------------------------------------------------------------------------
step_approve() {
  local channel=$1
  info "Channel ${channel}: approving (sequence ${SEQUENCE})"

  local org output
  for org in 1 2 3; do
    set_peer "${org}" 0
    if ! output=$(peer lifecycle chaincode approveformyorg \
        -o "${ORDERER}" --ordererTLSHostnameOverride "${ORDERER_NAME}" \
        --tls --cafile "${ORDERER_CA}" \
        --channelID "${channel}" \
        --name "${CHAINCODE_NAME}" \
        --version "${CHAINCODE_VERSION}" \
        --package-id "${PACKAGE_ID}" \
        --sequence "${SEQUENCE}" \
        --signature-policy "${ENDORSEMENT_POLICY}" 2>&1); then
      error "approveformyorg for Org${org}MSP on ${channel} failed"
      echo "${output}" | tail -8 >&2
      exit 1
    fi
    detail "Org${org}MSP approved"
  done
}

# ---------------------------------------------------------------------------
step_check_readiness() {
  local channel=$1
  set_peer 1 0

  local readiness
  readiness=$(peer lifecycle chaincode checkcommitreadiness \
    --channelID "${channel}" \
    --name "${CHAINCODE_NAME}" \
    --version "${CHAINCODE_VERSION}" \
    --sequence "${SEQUENCE}" \
    --signature-policy "${ENDORSEMENT_POLICY}" \
    --output json 2>/dev/null) || fatal "checkcommitreadiness on ${channel} failed"

  local not_approved
  not_approved=$(echo "${readiness}" | jq -r '.approvals | to_entries[] | select(.value == false) | .key')
  if [ -n "${not_approved}" ]; then
    error "organizations that have not approved on ${channel}: ${not_approved}"
    echo "${readiness}" >&2
    exit 1
  fi
  ok "all three organizations approved the definition"
}

# ---------------------------------------------------------------------------
step_commit() {
  local channel=$1
  info "Channel ${channel}: committing the definition"

  local output
  # LifecycleEndorsement is MAJORITY, so the commit must be signed by at least two
  # of three organizations. peer0 from all three is sent.
  if ! output=$(peer lifecycle chaincode commit \
      -o "${ORDERER}" --ordererTLSHostnameOverride "${ORDERER_NAME}" \
      --tls --cafile "${ORDERER_CA}" \
      --channelID "${channel}" \
      --name "${CHAINCODE_NAME}" \
      --version "${CHAINCODE_VERSION}" \
      --sequence "${SEQUENCE}" \
      --signature-policy "${ENDORSEMENT_POLICY}" \
      "${ENDORSER_ARGS[@]}" 2>&1); then
    error "commit on ${channel} failed"
    echo "${output}" | tail -10 >&2
    exit 1
  fi

  set_peer 1 0
  local definition
  definition=$(peer lifecycle chaincode querycommitted \
    --channelID "${channel}" --name "${CHAINCODE_NAME}" --output json)
  ok "committed: v$(echo "${definition}" | jq -r .version), sequence $(echo "${definition}" | jq -r .sequence)"
  detail "endorsement policy: ${ENDORSEMENT_POLICY}"
}

# ---------------------------------------------------------------------------
step_init() {
  local channel=$1
  info "Channel ${channel}: InitLedger"

  local output
  if ! output=$(peer chaincode invoke \
      -o "${ORDERER}" --ordererTLSHostnameOverride "${ORDERER_NAME}" \
      --tls --cafile "${ORDERER_CA}" \
      -C "${channel}" -n "${CHAINCODE_NAME}" \
      "${ENDORSER_ARGS[@]}" \
      --waitForEvent \
      -c '{"function":"InitLedger","Args":[]}' 2>&1); then
    error "InitLedger on ${channel} failed"
    echo "${output}" | tail -10 >&2
    exit 1
  fi

  local payload
  payload=$(echo "${output}" | sed -n 's/.*payload:"\(.*\)".*/\1/p' | head -1)
  ok "InitLedger executed${payload:+: ${payload}}"
}

# ---------------------------------------------------------------------------
step_verify() {
  header "6/6  Verification"

  set_peer 1 0
  local channel count
  for channel in "${CHANNEL1}" "${CHANNEL2}"; do
    count=$(peer chaincode query -C "${channel}" -n "${CHAINCODE_NAME}" \
      -c '{"function":"GetAllProducts","Args":[]}' 2>/dev/null | jq 'length' || echo 0)
    if [ "${count}" -gt 0 ]; then
      ok "${channel}: ${count} products in the world state"
    else
      error "${channel}: the query returned no products"
      exit 1
    fi
  done

  echo
  cat <<NEXT
Chaincode '${CHAINCODE_NAME}' is running on both channels.

Next step:

    cd application && npm install && node src/index.js

NEXT
}

# ---------------------------------------------------------------------------
main() {
  header "Deploying chaincode '${CHAINCODE_NAME}'"

  step_vendor
  step_package
  step_install

  header "4/6  Approving and committing per channel"
  local channel
  for channel in "${CHANNEL1}" "${CHANNEL2}"; do
    determine_sequence "${channel}"
    step_approve "${channel}"
    step_check_readiness "${channel}"
    step_commit "${channel}"
    echo
  done

  header "5/6  Setting the initial state"
  for channel in "${CHANNEL1}" "${CHANNEL2}"; do
    step_init "${channel}"
  done

  step_verify
}

main "$@"
