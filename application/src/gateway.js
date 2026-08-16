// Connecting to the network through the Peer Gateway service.
//
// @hyperledger/fabric-gateway talks gRPC to a single peer, which then handles
// endorsement collection, ordering and commit status on the client's behalf. That
// is why no connection profile listing every peer and orderer is needed: the
// endorsement policy OutOf(2, ...) is satisfied by the gateway peer, which gathers
// the signatures from the other organizations itself.

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

  // The peer certificate is issued for the container name (peer0.org1.pdasp.com)
  // while the connection goes to localhost, so the TLS target name is overridden.
  // Without this the handshake fails on a hostname mismatch.
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

/**
 * Runs a read only chaincode function (evaluateTransaction).
 *
 * Evaluate does not go to the orderer and does not change state, which is what
 * every search in this application needs. It is also the only mode in which
 * Fabric allows the paginated CouchDB query.
 */
export async function query(connection, functionName, args) {
  const bytes = await connection.contract.evaluateTransaction(functionName, ...args.map(String));
  return decode(bytes);
}

/**
 * Runs a state changing chaincode function (submitTransaction).
 *
 * submitTransaction returns only after the transaction has been committed to the
 * ledger, so whatever it returns is already final.
 */
export async function invoke(connection, functionName, args) {
  const bytes = await connection.contract.submitTransaction(functionName, ...args.map(String));
  return decode(bytes);
}

/**
 * Chaincode always answers with JSON, as the assignment requires. An empty payload
 * is returned as null rather than crashing the parser.
 */
function decode(bytes) {
  const text = UTF8.decode(bytes).trim();
  if (text === '') return null;
  try {
    return JSON.parse(text);
  } catch {
    // A function that returns a bare string rather than a document.
    return text;
  }
}

/** Opens a connection, runs the callback and always closes the connection. */
export async function withGateway(options, callback) {
  const connection = openGateway(options);
  try {
    return await callback(connection);
  } finally {
    connection.close();
  }
}
