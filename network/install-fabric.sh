#!/usr/bin/env bash
#
# One time environment setup:
#
#   1) download the Hyperledger Fabric binaries (peer, configtxgen, osnadmin, ...)
#   2) download fabric-ca-client
#   3) pull every docker image the network needs
#
# Binaries land in network/bin/, default configs in network/config/. Both are
# gitignored: tooling is not committed to the repository.
#
# Usage:
#   ./install-fabric.sh              # everything
#   ./install-fabric.sh binaries     # binaries only
#   ./install-fabric.sh images       # docker images only

set -euo pipefail

NETWORK_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "${NETWORK_ROOT}/scripts/utils.sh"
load_env

BIN_DIR="${NETWORK_ROOT}/bin"
CONFIG_DIR="${NETWORK_ROOT}/config"

# ---------------------------------------------------------------- platform
detect_platform() {
  local os arch
  case "$(uname -s)" in
    Darwin) os=darwin ;;
    Linux)  os=linux ;;
    *)      fatal "Unsupported operating system: $(uname -s)" ;;
  esac
  case "$(uname -m)" in
    x86_64|amd64)  arch=amd64 ;;
    arm64|aarch64) arch=arm64 ;;
    *)             fatal "Unsupported architecture: $(uname -m)" ;;
  esac
  echo "${os}-${arch}"
}

PLATFORM="$(detect_platform)"

# ---------------------------------------------------------------- download
# download_and_extract <url> <description>
download_and_extract() {
  local url=$1 description=$2
  local tmp
  tmp="$(mktemp -t pdasp-fabric)"
  detail "downloading ${description}"
  detail "${url}"
  if ! curl -fL --progress-bar -o "$tmp" "$url"; then
    rm -f "$tmp"
    fatal "Download failed: ${url}"
  fi
  tar -xzf "$tmp" -C "$NETWORK_ROOT"
  rm -f "$tmp"
  ok "${description} extracted"
}

install_binaries() {
  header "Fabric binaries: v${FABRIC_VERSION} / CA v${FABRIC_CA_VERSION} (${PLATFORM})"
  require_command curl "Install curl."
  require_command tar "Install tar."

  mkdir -p "$BIN_DIR" "$CONFIG_DIR"

  # The Fabric archive contains bin/ and config/, so it extracts straight into network/.
  download_and_extract \
    "https://github.com/hyperledger/fabric/releases/download/v${FABRIC_VERSION}/hyperledger-fabric-${PLATFORM}-${FABRIC_VERSION}.tar.gz" \
    "hyperledger-fabric ${FABRIC_VERSION}"

  # The fabric-ca archive only contains bin/fabric-ca-client and bin/fabric-ca-server.
  download_and_extract \
    "https://github.com/hyperledger/fabric-ca/releases/download/v${FABRIC_CA_VERSION}/hyperledger-fabric-ca-${PLATFORM}-${FABRIC_CA_VERSION}.tar.gz" \
    "hyperledger-fabric-ca ${FABRIC_CA_VERSION}"

  # macOS Gatekeeper quarantine: without clearing it the binaries cannot execute.
  if [ "$(uname -s)" = "Darwin" ]; then
    xattr -dr com.apple.quarantine "$BIN_DIR" 2>/dev/null || true
  fi
  chmod +x "$BIN_DIR"/* 2>/dev/null || true

  echo
  info "Installed versions:"
  "${BIN_DIR}/peer" version | sed -n '1,3p' | sed 's/^/    /'
  "${BIN_DIR}/fabric-ca-client" version | sed -n '1,2p' | sed 's/^/    /'
}

# ---------------------------------------------------------------- docker images
install_images() {
  header "Docker images"
  require_docker

  local images=(
    "hyperledger/fabric-peer:${IMAGE_TAG}"
    "hyperledger/fabric-orderer:${IMAGE_TAG}"
    "hyperledger/fabric-tools:${IMAGE_TAG}"
    "hyperledger/fabric-ccenv:${IMAGE_TAG}"
    "hyperledger/fabric-baseos:${IMAGE_TAG}"
    "hyperledger/fabric-ca:${CA_IMAGE_TAG}"
    "couchdb:${COUCHDB_VERSION}"
  )

  for image in "${images[@]}"; do
    if docker image inspect "$image" >/dev/null 2>&1; then
      detail "already present: ${image}"
    else
      info "docker pull ${image}"
      docker pull -q "$image" || fatal "Failed to pull image ${image}"
    fi
  done

  echo
  info "Local Fabric images:"
  docker images --format '    {{.Repository}}:{{.Tag}}  ({{.Size}})' \
    | grep -E 'hyperledger|couchdb' | sort
}

# ---------------------------------------------------------------- wrap up
print_next_steps() {
  header "Next step"
  cat <<INSTRUCTIONS
Put the Fabric binaries on PATH. The network scripts do this themselves; this is
only needed for running \`peer\` by hand from a terminal:

    export PATH="${BIN_DIR}:\$PATH"
    export FABRIC_CFG_PATH="${CONFIG_DIR}"

Then:

    ./network/network-up.sh

INSTRUCTIONS
}

# ---------------------------------------------------------------- main
case "${1:-all}" in
  binaries) install_binaries ;;
  images)   install_images ;;
  all)      install_binaries; install_images; print_next_steps ;;
  *)        fatal "Unknown argument '$1'. Expected: binaries | images | all" ;;
esac
