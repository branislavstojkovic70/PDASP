#!/usr/bin/env bash
#
# Brings the whole Fabric network up from scratch:
#
#   1) Fabric CA servers (4 containers)
#   2) crypto material for every organization, issued by those CAs
#   3) genesis blocks for channel1 and channel2
#   4) orderers, peers, CouchDB instances and the CLI (22 containers)
#
# The script is destructive by design: it always starts from a clean network,
# because mixing old crypto material with fresh genesis blocks produces MSP errors
# that are very hard to read. To stop without restarting: ./network-down.sh
#
# Next step after this script: ./create-channels.sh

set -euo pipefail

NETWORK_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "${NETWORK_ROOT}/scripts/utils.sh"
load_env

export PATH="${NETWORK_ROOT}/bin:$PATH"
export FABRIC_CFG_PATH="${NETWORK_ROOT}/configtx"

COMPOSE=(docker compose --env-file "${NETWORK_ROOT}/.env" -p "${COMPOSE_PROJECT_NAME}")
COMPOSE_CA=("${COMPOSE[@]}" -f "${NETWORK_ROOT}/compose/compose-ca.yaml")
COMPOSE_ALL=("${COMPOSE_CA[@]}" -f "${NETWORK_ROOT}/compose/compose-net.yaml")

# ---------------------------------------------------------------------------
step_prerequisites() {
  header "1/5  Checking prerequisites"
  require_docker
  require_fabric_binaries
  require_command jq "Install jq (brew install jq). create-channels.sh needs it."
  ok "docker daemon is running"
  ok "Fabric binaries: $(peer version | sed -n 's/ Version: //p' | head -1)"
}

# ---------------------------------------------------------------------------
step_cleanup() {
  header "2/5  Removing previous state"
  "${NETWORK_ROOT}/network-down.sh" --quiet
  ok "previous network and artifacts removed"
}

# ---------------------------------------------------------------------------
step_ca() {
  header "3/5  Fabric CA servers"
  "${COMPOSE_CA[@]}" up -d
  for ca in ca.org1 ca.org2 ca.org3 ca.orderer; do
    wait_for_healthy "${ca}.${DOMAIN}" 90
  done

  info "Generating crypto material"
  "${NETWORK_ROOT}/scripts/register-enroll.sh"
}

# ---------------------------------------------------------------------------
step_genesis() {
  header "4/5  Channel genesis blocks"
  mkdir -p "${NETWORK_ROOT}/channel-artifacts"
  local channel
  for channel in "${CHANNEL1}" "${CHANNEL2}"; do
    configtxgen -profile PdaspChannel \
      -channelID "${channel}" \
      -outputBlock "${NETWORK_ROOT}/channel-artifacts/${channel}.block" >/dev/null 2>&1 \
      || fatal "configtxgen failed for channel ${channel}"
    ok "${channel}.block"
  done
}

# ---------------------------------------------------------------------------
step_network() {
  header "5/5  Orderers, peers and CouchDB"
  "${COMPOSE_ALL[@]}" up -d

  # The orderer admin port is the first sign a RAFT node is ready to accept a channel.
  wait_for_port localhost "${ORDERER1_ADMIN_PORT}" "orderer.${DOMAIN} (admin)" 90
  wait_for_port localhost "${ORDERER2_ADMIN_PORT}" "orderer2.${DOMAIN} (admin)" 90
  wait_for_port localhost "${ORDERER3_ADMIN_PORT}" "orderer3.${DOMAIN} (admin)" 90

  local org p port_name
  for org in 1 2 3; do
    for p in 0 1 2; do
      port_name="PEER${p}_ORG${org}_PORT"
      wait_for_port localhost "${!port_name}" "peer${p}.org${org}.${DOMAIN}" 90
    done
  done
}

# ---------------------------------------------------------------------------
print_summary() {
  header "Network is up"
  docker ps --filter "label=pdasp.role" \
    --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}' | head -30
  echo
  info "Containers running: $(docker ps -q --filter "label=pdasp.role" | wc -l | tr -d ' ')"
  echo
  cat <<NEXT
Next steps:

    ./network/create-channels.sh      # creates ${CHANNEL1} and ${CHANNEL2}, joins all 9 peers
    ./network/deploy-chaincode.sh     # installs and commits the '${CHAINCODE_NAME}' chaincode

CouchDB Fauxton (peer0.org1):  http://localhost:${COUCHDB0_PORT}/_utils
   user: ${COUCHDB_USER}  password: ${COUCHDB_PASSWORD}

NEXT
}

# ---------------------------------------------------------------------------
main() {
  local started
  started=$(date +%s)

  step_prerequisites
  step_cleanup
  step_ca
  step_genesis
  step_network
  print_summary

  ok "Elapsed: $(( $(date +%s) - started ))s"
}

main "$@"
