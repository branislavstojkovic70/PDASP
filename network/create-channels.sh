#!/usr/bin/env bash
#
# Creates channel1 and channel2 and joins all nine peers to both.
#
# Fabric 2.5 has no system channel: a channel comes into existence by handing its
# genesis block to the orderers through the channel participation API
# (`osnadmin channel join`), after which the orderers form a RAFT cluster for that
# channel. Only then can peers join.
#
# Prerequisite: ./network-up.sh
# Next step: ./deploy-chaincode.sh

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

ARTIFACTS="${NETWORK_ROOT}/channel-artifacts"

# Admin ports of the RAFT nodes.
ORDERER_ADMIN_PORTS=("${ORDERER1_ADMIN_PORT}" "${ORDERER2_ADMIN_PORT}" "${ORDERER3_ADMIN_PORT}")
ORDERER_NAMES=("orderer.${DOMAIN}" "orderer2.${DOMAIN}" "orderer3.${DOMAIN}")

# ---------------------------------------------------------------------------
# osnadmin_join <channel> <admin_port> <orderer_name>
# ---------------------------------------------------------------------------
osnadmin_join() {
  local channel=$1 admin_port=$2 name=$3
  local output

  if output=$(osnadmin channel join \
      --channelID "${channel}" \
      --config-block "${ARTIFACTS}/${channel}.block" \
      -o "localhost:${admin_port}" \
      --ca-file "${ORDERER_CA}" \
      --client-cert "${ORDERER_ADMIN_TLS_CERT}" \
      --client-key "${ORDERER_ADMIN_TLS_KEY}" 2>&1); then
    detail "${name} -> ${channel}"
    return 0
  fi

  if echo "${output}" | grep -q "already exists"; then
    detail "${name} is already on ${channel}"
    return 0
  fi

  error "osnadmin channel join: ${name} -> ${channel}"
  echo "${output}" | tail -10 >&2
  exit 1
}

# ---------------------------------------------------------------------------
# create_channel <channel>
# ---------------------------------------------------------------------------
create_channel() {
  local channel=$1
  info "Channel ${channel}: joining RAFT nodes"

  [ -f "${ARTIFACTS}/${channel}.block" ] \
    || fatal "No genesis block at ${ARTIFACTS}/${channel}.block. Run network-up.sh"

  local i
  for i in 0 1 2; do
    osnadmin_join "${channel}" "${ORDERER_ADMIN_PORTS[$i]}" "${ORDERER_NAMES[$i]}"
  done

  wait_for_leader "${channel}"
}

# ---------------------------------------------------------------------------
# wait_for_leader <channel>
#
# Until RAFT elects a leader the channel accepts no transactions and
# `peer channel join` can fail. The status is read from the channel participation
# API of the first orderer.
# ---------------------------------------------------------------------------
wait_for_leader() {
  local channel=$1
  local elapsed=0 status

  while [ "${elapsed}" -lt 30 ]; do
    status=$(osnadmin channel list \
      --channelID "${channel}" \
      -o "localhost:${ORDERER1_ADMIN_PORT}" \
      --ca-file "${ORDERER_CA}" \
      --client-cert "${ORDERER_ADMIN_TLS_CERT}" \
      --client-key "${ORDERER_ADMIN_TLS_KEY}" 2>/dev/null \
      | sed -n '/^{/,$p' | jq -r '.status // "unknown"' 2>/dev/null || echo unknown)

    if [ "${status}" = "active" ]; then
      ok "channel ${channel} is active on the RAFT cluster"
      return 0
    fi
    sleep 1
    elapsed=$((elapsed + 1))
  done

  warn "channel ${channel} did not report status 'active' within 30s, continuing (peer join retries)"
}

# ---------------------------------------------------------------------------
# join_peers <channel>
# ---------------------------------------------------------------------------
join_peers() {
  local channel=$1
  info "Channel ${channel}: joining peers"

  local org p
  for org in 1 2 3; do
    for p in 0 1 2; do
      set_peer "${org}" "${p}"

      # Re-running the script must not fail on peers that already joined.
      if peer channel list 2>/dev/null | grep -qx "${channel}"; then
        detail "peer${p}.org${org} is already on ${channel}"
        continue
      fi

      retry 5 3 "peer${p}.org${org} -> ${channel}" -- \
        peer channel join -b "${ARTIFACTS}/${channel}.block" \
        || exit 1
      detail "peer${p}.org${org} -> ${channel}"
    done
  done
}

# ---------------------------------------------------------------------------
# verify <channel>  -- every peer must report the channel in `peer channel list`
# ---------------------------------------------------------------------------
verify() {
  local channel=$1
  local org p failures=0

  for org in 1 2 3; do
    for p in 0 1 2; do
      set_peer "${org}" "${p}"
      if ! peer channel list 2>/dev/null | grep -qx "${channel}"; then
        error "peer${p}.org${org} is NOT on channel ${channel}"
        failures=$((failures + 1))
      fi
    done
  done

  if [ "${failures}" -gt 0 ]; then
    fatal "${failures} peers failed to join channel ${channel}"
  fi
  ok "all 9 peers are on channel ${channel}"
}

# ---------------------------------------------------------------------------
print_summary() {
  header "Summary"

  local org p
  printf '%-26s %s\n' "PEER" "CHANNELS"
  for org in 1 2 3; do
    for p in 0 1 2; do
      set_peer "${org}" "${p}"
      printf '%-26s %s\n' "peer${p}.org${org}.${DOMAIN}" \
        "$(peer channel list 2>/dev/null | tail -n +2 | tr '\n' ' ')"
    done
  done

  echo
  info "Block height on peer0.org1 per channel:"
  set_peer 1 0
  local channel
  for channel in "${CHANNEL1}" "${CHANNEL2}"; do
    detail "${channel}: $(peer channel getinfo -c "${channel}" 2>/dev/null | sed -n 's/.*"height"://p' | cut -d, -f1) blocks"
  done

  echo
  cat <<NEXT
Next step:

    ./network/deploy-chaincode.sh

NEXT
}

# ---------------------------------------------------------------------------
main() {
  header "Creating channels ${CHANNEL1} and ${CHANNEL2}"

  local channel
  for channel in "${CHANNEL1}" "${CHANNEL2}"; do
    create_channel "${channel}"
    join_peers "${channel}"
    verify "${channel}"
    echo
  done

  print_summary
}

main "$@"
