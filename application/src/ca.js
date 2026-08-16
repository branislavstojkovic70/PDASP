// Enrollment and registration against Fabric CA.
//
// This is the "enroll / login" half of the assignment. Enrolling means: generate a
// key pair locally, send a certificate signing request to the CA over TLS,
// authenticate with an enrollment id and secret, and receive an X.509 certificate.
// The certificate plus the private key becomes a wallet identity.

import { readFileSync } from 'node:fs';

import FabricCAServices from 'fabric-ca-client';
import { User } from 'fabric-common';

import { saveIdentity, requireIdentity } from './wallet.js';

/** Builds a CA client for an organization. */
function caClient(org) {
  const tlsCert = readFileSync(org.ca.tlsCert, 'utf8');
  return new FabricCAServices(
    org.ca.url,
    { trustedRoots: tlsCert, verify: true },
    org.ca.name,
  );
}

/**
 * Enrolls an identity that the CA already knows and stores it in the wallet.
 *
 * network/scripts/register-enroll.sh registers admin, <org>admin and <org>user1
 * with fixed secrets, so those can be enrolled directly. Any other id has to be
 * registered first, see registerAndEnroll.
 */
export async function enroll(org, enrollmentID, enrollmentSecret) {
  const secret = enrollmentSecret ?? org.knownIdentities[enrollmentID];
  if (!secret) {
    throw new Error(
      `no secret known for '${enrollmentID}'. Pass --secret, or register the ` +
      `identity first with --register.`);
  }

  const ca = caClient(org);
  const enrollment = await ca.enroll({ enrollmentID, enrollmentSecret: secret });

  return saveIdentity(org.key, enrollmentID, {
    mspId: org.mspId,
    certificate: enrollment.certificate,
    privateKey: enrollment.key.toBytes(),
  });
}

/**
 * Registers a brand new identity and enrolls it.
 *
 * Registration needs a registrar: an identity the CA already trusts to create
 * others. The bootstrap admin of the organization is used, which is why it has to
 * be enrolled into the wallet first.
 *
 * The affiliation is left empty on purpose. The CA creates a default affiliation
 * tree (org1.department1 and so on) that has nothing to do with our organizations,
 * and an empty affiliation means the root, which the bootstrap admin owns.
 */
export async function registerAndEnroll(org, enrollmentID, options = {}) {
  const {
    registrar = 'admin',
    role = 'client',
    secret = defaultSecret(enrollmentID),
    attributes = [],
  } = options;

  const registrarIdentity = requireIdentity(org.key, registrar);
  const registrarUser = User.createUser(
    registrar,
    '',
    registrarIdentity.mspId,
    registrarIdentity.certificate,
    registrarIdentity.privateKey,
  );

  const ca = caClient(org);
  try {
    await ca.register({
      enrollmentID,
      enrollmentSecret: secret,
      role,
      affiliation: '',
      maxEnrollments: -1,
      attrs: attributes,
    }, registrarUser);
  } catch (error) {
    // Re-running a test script must not fail on an identity that already exists;
    // the secret is deterministic, so enrolling it again works.
    if (!String(error.message).includes('already registered')) {
      throw error;
    }
  }

  return enroll(org, enrollmentID, secret);
}

/**
 * Derives a deterministic secret from the enrollment id.
 *
 * Deterministic on purpose: a shell test script can register a user, exit, and a
 * later script can enroll the same user again without having stored anything.
 */
function defaultSecret(enrollmentID) {
  return `${enrollmentID}pw`;
}

/** Reads the CA information, useful as a connectivity check. */
export async function caInfo(org) {
  const ca = caClient(org);
  const info = await ca.newIdentityService(); // forces the client to initialize
  void info;
  return {
    organization: org.key,
    mspId: org.mspId,
    caName: org.ca.name,
    caUrl: org.ca.url,
  };
}
