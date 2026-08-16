#!/usr/bin/env bash
#
# Sets the CORE_PEER_* variables for the `peer` CLI running on the host machine
# (not inside the CLI container), which is why every address is localhost with the
# published port.
#
# Source it with: . "${NETWORK_ROOT}/scripts/env-var.sh"
# Requires NETWORK_ROOT to be set and .env to be loaded already.

ORGANIZATIONS="${NETWORK_ROOT}/organizations"

# TLS root of the orderer organization, needed for every call to an orderer.
ORDERER_CA="${ORGANIZATIONS}/ordererOrganizations/${DOMAIN}/tlsca/tlsca.${DOMAIN}-cert.pem"

# The orderer admin endpoint requires mutual TLS, so the client sends its own pair.
ORDERER_ADMIN_TLS_CERT="${ORGANIZATIONS}/ordererOrganizations/${DOMAIN}/users/Admin@${DOMAIN}/tls/server.crt"
ORDERER_ADMIN_TLS_KEY="${ORGANIZATIONS}/ordererOrganizations/${DOMAIN}/users/Admin@${DOMAIN}/tls/server.key"

# ---------------------------------------------------------------------------
# peer_tls_root <org> <peer>  -- path to the peer's TLS root certificate
# ---------------------------------------------------------------------------
peer_tls_root() {
  local org=$1 p=$2
  echo "${ORGANIZATIONS}/peerOrganizations/org${org}.${DOMAIN}/peers/peer${p}.org${org}.${DOMAIN}/tls/ca.crt"
}

# ---------------------------------------------------------------------------
# peer_port <org> <peer>  -- port published on the host
# ---------------------------------------------------------------------------
peer_port() {
  local org=$1 p=$2
  local name="PEER${p}_ORG${org}_PORT"
  echo "${!name}"
}

# ---------------------------------------------------------------------------
# set_peer <org> <peer>
#
# The identity is always the organization Admin: joining a channel and installing
# or approving chaincode all require admin rights. Regular users (User1) are used
# from the console application.
# ---------------------------------------------------------------------------
set_peer() {
  local org=$1 p=$2
  local org_domain="org${org}.${DOMAIN}"

  export CORE_PEER_TLS_ENABLED=true
  export CORE_PEER_LOCALMSPID="Org${org}MSP"
  CORE_PEER_TLS_ROOTCERT_FILE="$(peer_tls_root "${org}" "${p}")"
  export CORE_PEER_TLS_ROOTCERT_FILE
  export CORE_PEER_MSPCONFIGPATH="${ORGANIZATIONS}/peerOrganizations/${org_domain}/users/Admin@${org_domain}/msp"
  export CORE_PEER_ADDRESS="localhost:$(peer_port "${org}" "${p}")"
}

# ---------------------------------------------------------------------------
# Endorser arguments.
#
# These print one argument per line, and the caller collects them into an array:
#
#   args=(); while IFS= read -r line; do args+=("$line"); done < <(one_peer_per_org_args)
#   peer chaincode invoke ... "${args[@]}"
#
# One line per argument rather than one space separated string, because the
# certificate paths must not go through word splitting. A bash nameref would be
# tidier but requires bash 4.3, and macOS still ships bash 3.2.
# ---------------------------------------------------------------------------

# all_peer_args prints the --peerAddresses/--tlsRootCertFiles pairs for all nine peers.
all_peer_args() {
  local org p
  for org in 1 2 3; do
    for p in 0 1 2; do
      printf '%s\n' --peerAddresses "localhost:$(peer_port "${org}" "${p}")" \
                    --tlsRootCertFiles "$(peer_tls_root "${org}" "${p}")"
    done
  done
}

# one_peer_per_org_args prints the smallest set of endorsers that satisfies
# OutOf(2, ...): peer0 from each organization. Faster than all nine and still
# enough for the policy.
one_peer_per_org_args() {
  local org
  for org in 1 2 3; do
    printf '%s\n' --peerAddresses "localhost:$(peer_port "${org}" 0)" \
                  --tlsRootCertFiles "$(peer_tls_root "${org}" 0)"
  done
}

# collect_args <function> reads the lines a generator prints into COLLECTED_ARGS.
collect_args() {
  COLLECTED_ARGS=()
  local line
  while IFS= read -r line; do
    COLLECTED_ARGS+=("${line}")
  done < <("$1")
}

# ---------------------------------------------------------------------------
# retry <attempts> <pause_seconds> <description> -- <command...>
#
# RAFT needs a moment to elect a leader after a channel is joined, so the first
# `peer channel join` can fail without anything actually being wrong.
# ---------------------------------------------------------------------------
retry() {
  local attempts=$1 pause=$2 description=$3
  shift 3
  [ "$1" = "--" ] && shift

  local i output
  for (( i = 1; i <= attempts; i++ )); do
    if output=$("$@" 2>&1); then
      return 0
    fi
    if [ "$i" -lt "$attempts" ]; then
      detail "${description}: attempt ${i}/${attempts} failed, retrying in ${pause}s"
      sleep "${pause}"
    fi
  done

  error "${description} failed after ${attempts} attempts"
  echo "${output}" | tail -10 >&2
  return 1
}
