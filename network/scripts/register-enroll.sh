#!/usr/bin/env bash
#
# Generates all crypto material through the Fabric CA servers.
# `cryptogen` is deliberately not used: the assignment requires a CA.
#
# For each peer organization (org1, org2, org3):
#   - enroll the CA bootstrap (registrar) identity
#   - register and enroll peer0, peer1, peer2, Admin and User1
#   - produce both MSP and TLS material for every node
# For the orderer organization:
#   - register and enroll orderer, orderer2, orderer3 and Admin
#
# Called by network-up.sh. Can also run standalone while the CA containers are up.

set -euo pipefail

NETWORK_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
. "${NETWORK_ROOT}/scripts/utils.sh"
load_env

export PATH="${NETWORK_ROOT}/bin:$PATH"
require_fabric_binaries

ORGANIZATIONS_DIR="${NETWORK_ROOT}/organizations"
CA_DIR="${ORGANIZATIONS_DIR}/fabric-ca"
# Home directories of the registrar identities. Kept apart from the organization
# MSP so that the MSP contains only what belongs there: config.yaml plus roots.
REGISTRAR_DIR="${CA_DIR}/_registrar"

# ---------------------------------------------------------------------------
# config.yaml with the NodeOU definitions.
#
# Without this Fabric would not know which certificate belongs to an admin, which
# to a peer and which to a plain client, and admin certificates would have to be
# copied into `admincerts/` by hand. With NodeOUs the role is read from the OU
# field of the certificate, which the CA sets based on `--id.type` at registration.
# ---------------------------------------------------------------------------
write_config_yaml() {
  local msp_dir=$1
  cat > "${msp_dir}/config.yaml" <<'YAML'
NodeOUs:
  Enable: true
  ClientOUIdentifier:
    Certificate: cacerts/ca-cert.pem
    OrganizationalUnitIdentifier: client
  PeerOUIdentifier:
    Certificate: cacerts/ca-cert.pem
    OrganizationalUnitIdentifier: peer
  AdminOUIdentifier:
    Certificate: cacerts/ca-cert.pem
    OrganizationalUnitIdentifier: admin
  OrdererOUIdentifier:
    Certificate: cacerts/ca-cert.pem
    OrganizationalUnitIdentifier: orderer
YAML
}

# ---------------------------------------------------------------------------
# enroll_msp <name> <secret> <ca_port> <ca_name> <ca_tls_cert> <target_msp_dir>
#
# Enrolls an identity and tidies up the MSP directory:
#   - `cacerts/` gets the stable name ca-cert.pem, because the name the CA
#     generates embeds host and port, which would break the config.yaml reference
#   - config.yaml with the NodeOU definitions is written alongside
# ---------------------------------------------------------------------------
enroll_msp() {
  local name=$1 secret=$2 ca_port=$3 ca_name=$4 ca_tls=$5 msp_dir=$6

  fabric-ca-client enroll \
    -u "https://${name}:${secret}@localhost:${ca_port}" \
    --caname "${ca_name}" \
    -M "${msp_dir}" \
    --tls.certfiles "${ca_tls}" >/dev/null 2>&1 \
    || fatal "MSP enrollment of '${name}' on ${ca_name} failed"

  # The CA stores the root certificate under a name such as localhost-7054-ca-org1.pem.
  mv "${msp_dir}"/cacerts/*.pem "${msp_dir}/cacerts/ca-cert.pem"
  write_config_yaml "${msp_dir}"
}

# ---------------------------------------------------------------------------
# enroll_tls <name> <secret> <ca_port> <ca_name> <ca_tls_cert> <target_tls_dir> <csr_hosts>
#
# Enrolling with `--enrollment.profile tls` yields the certificate used for the
# node's TLS handshake. Fabric expects fixed file names (server.crt, server.key,
# ca.crt) while the CA writes randomized ones, hence the renaming.
# The CSR hosts must cover both the DNS name inside the docker network and
# localhost, since nodes are reached from containers and from the host.
# ---------------------------------------------------------------------------
enroll_tls() {
  local name=$1 secret=$2 ca_port=$3 ca_name=$4 ca_tls=$5 tls_dir=$6 csr_hosts=$7

  fabric-ca-client enroll \
    -u "https://${name}:${secret}@localhost:${ca_port}" \
    --caname "${ca_name}" \
    -M "${tls_dir}" \
    --enrollment.profile tls \
    --csr.hosts "${csr_hosts}" \
    --tls.certfiles "${ca_tls}" >/dev/null 2>&1 \
    || fatal "TLS enrollment of '${name}' on ${ca_name} failed"

  cp "${tls_dir}"/tlscacerts/* "${tls_dir}/ca.crt"
  cp "${tls_dir}"/signcerts/*  "${tls_dir}/server.crt"
  cp "${tls_dir}"/keystore/*   "${tls_dir}/server.key"
  rm -rf "${tls_dir}/tlscacerts" "${tls_dir}/signcerts" "${tls_dir}/keystore" \
         "${tls_dir}/cacerts" "${tls_dir}/user" "${tls_dir}/IssuerPublicKey" \
         "${tls_dir}/IssuerRevocationPublicKey"
}

# ---------------------------------------------------------------------------
# register_identity <name> <secret> <type> <ca_name> <ca_tls_cert>
#
# Idempotent: if the identity already exists the CA returns an error which is
# deliberately swallowed so the script can be run again.
# ---------------------------------------------------------------------------
register_identity() {
  local name=$1 secret=$2 type=$3 ca_name=$4 ca_tls=$5
  local output
  if output=$(fabric-ca-client register \
      --caname "${ca_name}" \
      --id.name "${name}" \
      --id.secret "${secret}" \
      --id.type "${type}" \
      --tls.certfiles "${ca_tls}" 2>&1); then
    return 0
  fi
  if echo "${output}" | grep -q "already registered"; then
    detail "identity '${name}' is already registered, skipping"
    return 0
  fi
  error "registration of identity '${name}' (${type}) on ${ca_name} failed"
  echo "${output}" | tail -5 >&2
  exit 1
}

# ---------------------------------------------------------------------------
# process_peer_org <org_number> <ca_port>
# ---------------------------------------------------------------------------
process_peer_org() {
  local n=$1 ca_port=$2
  local org="org${n}"
  local org_domain="${org}.${DOMAIN}"
  local ca_name="ca-${org}"
  local ca_tls="${CA_DIR}/${org}/tls-cert.pem"
  local ca_root="${CA_DIR}/${org}/ca-cert.pem"
  local org_dir="${ORGANIZATIONS_DIR}/peerOrganizations/${org_domain}"

  info "Organization ${org} (${org_domain}) via CA ${ca_name} on port ${ca_port}"
  [ -f "${ca_tls}" ] || fatal "No CA TLS certificate at ${ca_tls}. Are the CA containers running?"

  mkdir -p "${org_dir}"/{msp/cacerts,msp/tlscacerts,peers,users,ca,tlsca}

  # --- organization MSP: root certificates plus NodeOU rules -----------------
  # A single CA issues both identity and TLS certificates, so the root is shared.
  cp "${ca_root}" "${org_dir}/msp/cacerts/ca-cert.pem"
  cp "${ca_root}" "${org_dir}/msp/tlscacerts/tlsca-cert.pem"
  cp "${ca_root}" "${org_dir}/ca/ca.${org_domain}-cert.pem"
  cp "${ca_root}" "${org_dir}/tlsca/tlsca.${org_domain}-cert.pem"
  write_config_yaml "${org_dir}/msp"

  # --- registrar (the CA bootstrap admin) -----------------------------------
  export FABRIC_CA_CLIENT_HOME="${REGISTRAR_DIR}/${org}"
  mkdir -p "${FABRIC_CA_CLIENT_HOME}"
  fabric-ca-client enroll \
    -u "https://admin:adminpw@localhost:${ca_port}" \
    --caname "${ca_name}" \
    --tls.certfiles "${ca_tls}" >/dev/null 2>&1 \
    || fatal "Enrollment of the registrar identity on ${ca_name} failed"

  # --- register every identity of the organization --------------------------
  local p
  for p in 0 1 2; do
    register_identity "peer${p}.${org}" "peer${p}pw" peer "${ca_name}" "${ca_tls}"
  done
  register_identity "${org}admin" "${org}adminpw" admin  "${ca_name}" "${ca_tls}"
  register_identity "${org}user1" "${org}user1pw" client "${ca_name}" "${ca_tls}"

  # --- peer nodes: MSP plus TLS ---------------------------------------------
  for p in 0 1 2; do
    local peer_name="peer${p}.${org_domain}"
    local peer_dir="${org_dir}/peers/${peer_name}"
    enroll_msp "peer${p}.${org}" "peer${p}pw" "${ca_port}" "${ca_name}" "${ca_tls}" \
               "${peer_dir}/msp"
    enroll_tls "peer${p}.${org}" "peer${p}pw" "${ca_port}" "${ca_name}" "${ca_tls}" \
               "${peer_dir}/tls" "${peer_name},peer${p}-${org},localhost,127.0.0.1"
    detail "${peer_name}: MSP + TLS"
  done

  # --- users ----------------------------------------------------------------
  enroll_msp "${org}admin" "${org}adminpw" "${ca_port}" "${ca_name}" "${ca_tls}" \
             "${org_dir}/users/Admin@${org_domain}/msp"
  enroll_msp "${org}user1" "${org}user1pw" "${ca_port}" "${ca_name}" "${ca_tls}" \
             "${org_dir}/users/User1@${org_domain}/msp"
  detail "Admin@${org_domain}, User1@${org_domain}"

  ok "${org}: 3 peers and 2 users"
}

# ---------------------------------------------------------------------------
# process_orderer_org
# ---------------------------------------------------------------------------
process_orderer_org() {
  local ca_port="${CA_ORDERER_PORT}"
  local ca_name="ca-orderer"
  local ca_tls="${CA_DIR}/orderer/tls-cert.pem"
  local ca_root="${CA_DIR}/orderer/ca-cert.pem"
  local org_dir="${ORGANIZATIONS_DIR}/ordererOrganizations/${DOMAIN}"

  info "Orderer organization (${DOMAIN}) via CA ${ca_name} on port ${ca_port}"
  [ -f "${ca_tls}" ] || fatal "No CA TLS certificate at ${ca_tls}. Are the CA containers running?"

  mkdir -p "${org_dir}"/{msp/cacerts,msp/tlscacerts,orderers,users,tlsca}

  cp "${ca_root}" "${org_dir}/msp/cacerts/ca-cert.pem"
  cp "${ca_root}" "${org_dir}/msp/tlscacerts/tlsca-cert.pem"
  cp "${ca_root}" "${org_dir}/tlsca/tlsca.${DOMAIN}-cert.pem"
  write_config_yaml "${org_dir}/msp"

  export FABRIC_CA_CLIENT_HOME="${REGISTRAR_DIR}/orderer"
  mkdir -p "${FABRIC_CA_CLIENT_HOME}"
  fabric-ca-client enroll \
    -u "https://admin:adminpw@localhost:${ca_port}" \
    --caname "${ca_name}" \
    --tls.certfiles "${ca_tls}" >/dev/null 2>&1 \
    || fatal "Enrollment of the registrar identity on ${ca_name} failed"

  # Three node RAFT cluster: orderer, orderer2, orderer3.
  local i name node node_dir
  for i in 1 2 3; do
    [ "$i" -eq 1 ] && name="orderer" || name="orderer${i}"
    register_identity "${name}" "${name}pw" orderer "${ca_name}" "${ca_tls}"
  done
  register_identity "ordereradmin" "ordereradminpw" admin "${ca_name}" "${ca_tls}"

  for i in 1 2 3; do
    [ "$i" -eq 1 ] && name="orderer" || name="orderer${i}"
    node="${name}.${DOMAIN}"
    node_dir="${org_dir}/orderers/${node}"
    enroll_msp "${name}" "${name}pw" "${ca_port}" "${ca_name}" "${ca_tls}" \
               "${node_dir}/msp"
    enroll_tls "${name}" "${name}pw" "${ca_port}" "${ca_name}" "${ca_tls}" \
               "${node_dir}/tls" "${node},${name},localhost,127.0.0.1"
    detail "${node}: MSP + TLS"
  done

  enroll_msp "ordereradmin" "ordereradminpw" "${ca_port}" "${ca_name}" "${ca_tls}" \
             "${org_dir}/users/Admin@${DOMAIN}/msp"
  # osnadmin authenticates to the orderer admin port with mutual TLS, so the admin
  # identity needs a TLS key pair as well.
  enroll_tls "ordereradmin" "ordereradminpw" "${ca_port}" "${ca_name}" "${ca_tls}" \
             "${org_dir}/users/Admin@${DOMAIN}/tls" "localhost,127.0.0.1,${DOMAIN}"
  detail "Admin@${DOMAIN}: MSP + TLS (for osnadmin)"

  ok "orderer org: 3 RAFT nodes and an admin"
}

# ---------------------------------------------------------------------------
main() {
  header "Generating crypto material through Fabric CA"

  mkdir -p "${REGISTRAR_DIR}"

  process_peer_org 1 "${CA_ORG1_PORT}"
  process_peer_org 2 "${CA_ORG2_PORT}"
  process_peer_org 3 "${CA_ORG3_PORT}"
  process_orderer_org

  echo
  ok "Crypto material generated under ${ORGANIZATIONS_DIR}"
}

main "$@"
