import { readFileSync } from 'node:fs';
import { createPrivateKey } from 'node:crypto';

import * as grpc from '@grpc/grpc-js';
import { connect, hash, signers } from '@hyperledger/fabric-gateway';

import { CHAINCODE_NAME, requirePeer } from './config.js';
import { requireIdentity } from './wallet.js';

const UTF8 = new TextDecoder();

/**
 * Opens a gateway connection.
 *
 * @param {object} options
 * @param {object} options.org        organization definition from config.js
 * @param {string} options.user       enrollment id of a wallet identity
 * @param {string} options.channel    channel name
 * @param {number} [options.peerIndex] which peer of the organization to dial
 * @returns {{contract: object, close: function}}
 */
export function openGateway({ org, user, channel, peerIndex = 0 }) {
  const identity = requireIdentity(org.key, user);
  const peer = requirePeer(org, peerIndex);
  const client = new grpc.Client(
    peer.endpoint,
    grpc.credentials.createSsl(readFileSync(peer.tlsRootCert)),
    { 'grpc.ssl_target_name_override': peer.hostAlias },
  );

  const gateway = connect({
    client,
    identity: {
      mspId: identity.mspId,
      credentials: Buffer.from(identity.certificate),
    },
    signer: signers.newPrivateKeySigner(createPrivateKey(identity.privateKey)),
    hash: hash.sha256,
    // Without deadlines a peer that is up but not responding hangs the CLI.
    evaluateOptions: () => ({ deadline: Date.now() + 15_000 }),
    endorseOptions: () => ({ deadline: Date.now() + 30_000 }),
    submitOptions: () => ({ deadline: Date.now() + 15_000 }),
    commitStatusOptions: () => ({ deadline: Date.now() + 60_000 }),
  });

  const contract = gateway.getNetwork(channel).getContract(CHAINCODE_NAME);

  return {
    contract,
    peer: peer.name,
    close() {
      gateway.close();
      client.close();
    },
  };
}

export async function query(connection, functionName, args) {
  const bytes = await connection.contract.evaluateTransaction(functionName, ...args.map(String));
  return decode(bytes);
}

export async function invoke(connection, functionName, args) {
  const bytes = await connection.contract.submitTransaction(functionName, ...args.map(String));
  return decode(bytes);
}

function decode(bytes) {
  const text = UTF8.decode(bytes).trim();
  if (text === '') return null;
  try {
    return JSON.parse(text);
  } catch {
    return text;
  }
}

export async function withGateway(options, callback) {
  const connection = openGateway(options);
  try {
    return await callback(connection);
  } finally {
    connection.close();
  }
}
