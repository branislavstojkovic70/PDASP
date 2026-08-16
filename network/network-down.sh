#!/usr/bin/env bash
#
# Stops the network and deletes everything that was generated.
#
#   ./network-down.sh          # stop the network, delete crypto material and artifacts
#   ./network-down.sh --quiet  # no headers or summary (used by network-up.sh)
#   ./network-down.sh --keep-crypto   # stop containers, keep organizations/
#
# Chaincode is removed too: the peer creates a separate dev-peer* container and
# image for every installed chaincode, and those survive `compose down` because
# compose did not create them.

set -euo pipefail

NETWORK_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "${NETWORK_ROOT}/scripts/utils.sh"
load_env

QUIET=false
KEEP_CRYPTO=false
for arg in "$@"; do
  case "$arg" in
    --quiet)        QUIET=true ;;
    --keep-crypto)  KEEP_CRYPTO=true ;;
    *) fatal "Unknown argument '$arg'. Expected: --quiet | --keep-crypto" ;;
  esac
done

$QUIET || header "Shutting the network down"

COMPOSE=(docker compose --env-file "${NETWORK_ROOT}/.env" -p "${COMPOSE_PROJECT_NAME}"
         -f "${NETWORK_ROOT}/compose/compose-ca.yaml"
         -f "${NETWORK_ROOT}/compose/compose-net.yaml")

# ---------------------------------------------------------------- containers
if docker info >/dev/null 2>&1; then
  # -v also removes the named volumes holding ledger data. Without it the old
  # ledger would survive and clash with the new genesis block on the next start.
  "${COMPOSE[@]}" down --volumes --remove-orphans >/dev/null 2>&1 || true
  $QUIET || ok "containers and volumes removed"

  # Chaincode containers and images created by the peer, not by compose.
  cc_containers="$(docker ps -aq --filter "name=dev-peer" 2>/dev/null || true)"
  if [ -n "$cc_containers" ]; then
    docker rm -f $cc_containers >/dev/null 2>&1 || true
    $QUIET || ok "chaincode containers removed"
  fi
  cc_images="$(docker images -q --filter "reference=dev-peer*" 2>/dev/null || true)"
  if [ -n "$cc_images" ]; then
    docker rmi -f $cc_images >/dev/null 2>&1 || true
    $QUIET || ok "chaincode images removed"
  fi
else
  $QUIET || warn "docker daemon is not running, skipping container removal"
fi

# ---------------------------------------------------------------- artifacts
rm -f "${NETWORK_ROOT}"/channel-artifacts/*.block
rm -f "${NETWORK_ROOT}"/*.tar.gz
$QUIET || ok "genesis blocks and chaincode packages deleted"

if $KEEP_CRYPTO; then
  $QUIET || warn "crypto material kept (--keep-crypto)"
else
  # CA databases and certificates. .gitkeep is preserved so the directory stays
  # tracked in the repository.
  find "${NETWORK_ROOT}/organizations/fabric-ca" -mindepth 1 -maxdepth 1 \
       ! -name '.gitkeep' -exec rm -rf {} + 2>/dev/null || true
  rm -rf "${NETWORK_ROOT}/organizations/peerOrganizations" \
         "${NETWORK_ROOT}/organizations/ordererOrganizations"
  $QUIET || ok "crypto material deleted"

  # The console application identities were derived from the CAs just deleted.
  if [ -d "${NETWORK_ROOT}/../application/wallet" ]; then
    rm -rf "${NETWORK_ROOT}/../application/wallet"
    $QUIET || ok "console application wallet deleted"
  fi
fi

if ! $QUIET; then
  remaining="$(docker ps -q --filter "label=pdasp.role" 2>/dev/null | wc -l | tr -d ' ')"
  echo
  ok "Network is down (pdasp containers still running: ${remaining})"
fi
