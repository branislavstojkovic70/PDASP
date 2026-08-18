import { readFileSync } from 'node:fs';

import FabricCAServices from 'fabric-ca-client';
import { User } from 'fabric-common';

import { saveIdentity, requireIdentity } from './wallet.js';

function caClient(org) {
  const tlsCert = readFileSync(org.ca.tlsCert, 'utf8');
  return new FabricCAServices(
    org.ca.url,
    { trustedRoots: tlsCert, verify: true },
    org.ca.name,
  );
}


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
    if (!String(error.message).includes('already registered')) {
      throw error;
    }
  }

  return enroll(org, enrollmentID, secret);
}

function defaultSecret(enrollmentID) {
  return `${enrollmentID}pw`;
}

export async function caInfo(org) {
  const ca = caClient(org);
  const info = await ca.newIdentityService(); 
  void info;
  return {
    organization: org.key,
    mspId: org.mspId,
    caName: org.ca.name,
    caUrl: org.ca.url,
  };
}
