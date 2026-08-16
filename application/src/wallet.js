// Filesystem wallet: one JSON file per identity under application/wallet/.
//
// An identity is the X.509 certificate the CA issued plus its private key and the
// MSP id of the organization that owns it. That is everything the Gateway needs to
// sign a transaction proposal, so there is no reason to pull in a heavier wallet
// implementation.

import { mkdirSync, readdirSync, readFileSync, writeFileSync, existsSync, rmSync } from 'node:fs';
import { join } from 'node:path';

import { WALLET_DIR } from './config.js';

/** Wallet file names are <user>@<org>.id, matching how Fabric names identities. */
function identityPath(org, user) {
  return join(WALLET_DIR, `${user}@${org}.id`);
}

export function identityLabel(org, user) {
  return `${user}@${org}`;
}

/**
 * Stores an identity.
 *
 * @param {string} org      organization key, for example "org1"
 * @param {string} user     enrollment id
 * @param {object} identity { mspId, certificate, privateKey }
 */
export function saveIdentity(org, user, identity) {
  mkdirSync(WALLET_DIR, { recursive: true });
  const record = {
    label: identityLabel(org, user),
    org,
    user,
    mspId: identity.mspId,
    certificate: identity.certificate,
    privateKey: identity.privateKey,
    enrolledAt: new Date().toISOString(),
  };
  writeFileSync(identityPath(org, user), JSON.stringify(record, null, 2), {
    // The file holds a private key, so it must not be world readable.
    mode: 0o600,
  });
  return record;
}

/** Returns the stored identity, or null when it was never enrolled. */
export function loadIdentity(org, user) {
  const path = identityPath(org, user);
  if (!existsSync(path)) return null;
  return JSON.parse(readFileSync(path, 'utf8'));
}

/** Like loadIdentity but explains how to obtain the identity when it is missing. */
export function requireIdentity(org, user) {
  const identity = loadIdentity(org, user);
  if (!identity) {
    throw new Error(
      `identity '${identityLabel(org, user)}' is not in the wallet. ` +
      `Enroll it first:  node src/index.js enroll --org ${org} --user ${user}`);
  }
  return identity;
}

export function hasIdentity(org, user) {
  return existsSync(identityPath(org, user));
}

export function removeIdentity(org, user) {
  const path = identityPath(org, user);
  if (!existsSync(path)) return false;
  rmSync(path);
  return true;
}

/** Lists every stored identity, without the private keys. */
export function listIdentities() {
  if (!existsSync(WALLET_DIR)) return [];
  return readdirSync(WALLET_DIR)
    .filter((name) => name.endsWith('.id'))
    .map((name) => JSON.parse(readFileSync(join(WALLET_DIR, name), 'utf8')))
    .map(({ label, org, user, mspId, enrolledAt }) => ({ label, org, user, mspId, enrolledAt }))
    .sort((a, b) => a.label.localeCompare(b.label));
}
