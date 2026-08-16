// Network configuration for the console application.
//
// Everything is derived from network/.env rather than duplicated here, so ports
// and names cannot drift apart from what the network scripts actually start.

import { readFileSync, existsSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const SRC_DIR = dirname(fileURLToPath(import.meta.url));
export const APP_ROOT = resolve(SRC_DIR, '..');
export const REPO_ROOT = resolve(APP_ROOT, '..');
export const NETWORK_ROOT = join(REPO_ROOT, 'network');
export const WALLET_DIR = join(APP_ROOT, 'wallet');

/**
 * Parses network/.env. The format is plain KEY=VALUE, so a full dotenv parser
 * would be overkill; surrounding quotes are stripped because ENDORSEMENT_POLICY
 * is quoted for the shell.
 */
function readEnvFile() {
  const path = join(NETWORK_ROOT, '.env');
  if (!existsSync(path)) {
    throw new Error(`network/.env not found at ${path}`);
  }

  const values = {};
  for (const rawLine of readFileSync(path, 'utf8').split('\n')) {
    const line = rawLine.trim();
    if (line === '' || line.startsWith('#')) continue;

    const separator = line.indexOf('=');
    if (separator === -1) continue;

    const key = line.slice(0, separator).trim();
    let value = line.slice(separator + 1).trim();
    if ((value.startsWith('"') && value.endsWith('"')) ||
        (value.startsWith("'") && value.endsWith("'"))) {
      value = value.slice(1, -1);
    }
    values[key] = value;
  }
  return values;
}

const env = readEnvFile();

export const DOMAIN = env.DOMAIN;
export const CHAINCODE_NAME = env.CHAINCODE_NAME;
export const CHANNELS = [env.CHANNEL1, env.CHANNEL2];
export const DEFAULT_CHANNEL = env.CHANNEL1;

/**
 * One entry per peer organization.
 *
 * `peers` lists every peer of the organization. The application talks to peer0 by
 * default; the others are there so a test can point at a different peer and show
 * that any of them serves the Gateway.
 */
function buildOrg(number) {
  const org = `org${number}`;
  const orgDomain = `${org}.${DOMAIN}`;
  const orgDir = join(NETWORK_ROOT, 'organizations', 'peerOrganizations', orgDomain);

  const peers = [0, 1, 2].map((index) => {
    const name = `peer${index}.${orgDomain}`;
    return {
      name,
      // The certificate is issued for the container name, so the gRPC channel has
      // to override the TLS target name even though we dial localhost.
      endpoint: `localhost:${env[`PEER${index}_ORG${number}_PORT`]}`,
      hostAlias: name,
      tlsRootCert: join(orgDir, 'peers', name, 'tls', 'ca.crt'),
    };
  });

  return {
    key: org,
    label: ORG_LABELS[org],
    mspId: env[`ORG${number}_MSP`],
    domain: orgDomain,
    directory: orgDir,
    peers,
    ca: {
      name: `ca-${org}`,
      url: `https://localhost:${env[`CA_ORG${number}_PORT`]}`,
      // Trust anchor for the enrollment call. It has to be ca-cert.pem, the CA
      // root, and not tls-cert.pem: the latter is the server's leaf certificate,
      // issued by that root. The Go fabric-ca-client accepts a leaf as a trusted
      // certificate, but node's TLS stack wants the issuer, and handing it the
      // leaf fails with "unable to verify the first certificate".
      tlsCert: join(NETWORK_ROOT, 'organizations', 'fabric-ca', org, 'ca-cert.pem'),
    },
    // Identities that network/scripts/register-enroll.sh already registered, with
    // the secrets it used. They can be enrolled without a registrar.
    knownIdentities: {
      admin: 'adminpw',
      [`${org}admin`]: `${org}adminpw`,
      [`${org}user1`]: `${org}user1pw`,
    },
  };
}

// The business meaning of each organization, see ARCHITECTURE.md section 2.2.
const ORG_LABELS = {
  org1: 'Merchants',
  org2: 'Customers',
  org3: 'Regulator',
};

export const ORGS = {
  org1: buildOrg(1),
  org2: buildOrg(2),
  org3: buildOrg(3),
};

export const DEFAULT_ORG = 'org1';
export const DEFAULT_USER = 'org1user1';

/** Returns the organization definition or throws with the valid choices listed. */
export function requireOrg(key) {
  const org = ORGS[key];
  if (!org) {
    throw new Error(`unknown organization '${key}', expected one of: ${Object.keys(ORGS).join(', ')}`);
  }
  return org;
}

/** Returns the channel name or throws with the valid choices listed. */
export function requireChannel(name) {
  if (!CHANNELS.includes(name)) {
    throw new Error(`unknown channel '${name}', expected one of: ${CHANNELS.join(', ')}`);
  }
  return name;
}

/** Returns a peer of the organization by index, defaulting to peer0. */
export function requirePeer(org, index = 0) {
  const peer = org.peers[index];
  if (!peer) {
    throw new Error(`organization ${org.key} has no peer with index ${index}`);
  }
  return peer;
}
