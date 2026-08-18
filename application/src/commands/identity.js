import { X509Certificate } from 'node:crypto';

import { ORGS, requireOrg } from '../config.js';
import { enroll, registerAndEnroll } from '../ca.js';
import { listIdentities, loadIdentity, removeIdentity } from '../wallet.js';
import { registerCommand, requireArg } from '../cli.js';

const GROUP = 'Identity';

registerCommand({
  name: 'enroll',
  group: GROUP,
  usage: 'enroll --org <org1|org2|org3> --user <id> [--secret <pw>] [--register]',
  summary: 'Enrol an identity with the organization CA and store it in the wallet',
  async run({ flags }) {
    const org = requireOrg(String(flags.org ?? 'org1'));
    const user = String(flags.user ?? '');
    if (!user) {
      throw new Error('--user is required. Example: --org org1 --user org1user1');
    }

    const identity = flags.register
      ? await registerAndEnroll(org, user, {
          registrar: String(flags.registrar ?? 'admin'),
          role: String(flags.role ?? 'client'),
          secret: flags.secret ? String(flags.secret) : undefined,
        })
      : await enroll(org, user, flags.secret ? String(flags.secret) : undefined);

    return {
      label: identity.label,
      organization: org.key,
      organizationRole: org.label,
      mspId: identity.mspId,
      enrolledAt: identity.enrolledAt,
      registered: Boolean(flags.register),
    };
  },
});

registerCommand({
  name: 'identities',
  group: GROUP,
  usage: 'identities',
  summary: 'List the identities currently in the wallet',
  async run() {
    return listIdentities();
  },
});

registerCommand({
  name: 'whoami',
  group: GROUP,
  usage: 'whoami --org <org> --user <id>',
  summary: 'Show which identity a command would use and what it maps to',
  async run({ flags }) {
    const org = requireOrg(String(flags.org ?? 'org1'));
    const user = String(flags.user ?? 'org1user1');
    const identity = loadIdentity(org.key, user);

    return {
      organization: org.key,
      organizationRole: org.label,
      mspId: org.mspId,
      user,
      enrolled: identity !== null,
      enrolledAt: identity?.enrolledAt ?? null,
      certificateSubject: identity ? subjectOf(identity.certificate) : null,
    };
  },
});

registerCommand({
  name: 'forget',
  group: GROUP,
  usage: 'forget --org <org> --user <id>',
  summary: 'Remove an identity from the wallet',
  async run({ flags }) {
    const org = requireOrg(String(flags.org ?? 'org1'));
    const user = String(flags.user ?? '');
    if (!user) throw new Error('--user is required');
    return { label: `${user}@${org.key}`, removed: removeIdentity(org.key, user) };
  },
});

registerCommand({
  name: 'organizations',
  group: GROUP,
  usage: 'organizations',
  summary: 'List the organizations, their MSP ids and CA endpoints',
  async run() {
    return Object.values(ORGS).map((org) => ({
      key: org.key,
      role: org.label,
      mspId: org.mspId,
      caUrl: org.ca.url,
      caName: org.ca.name,
      peers: org.peers.map((peer) => peer.endpoint),
      knownIdentities: Object.keys(org.knownIdentities),
    }));
  },
});

registerCommand({
  name: 'bootstrap',
  group: GROUP,
  usage: 'bootstrap',
  summary: 'Enrol the standard identities of all three organizations at once',
  async run() {
    const enrolled = [];
    for (const org of Object.values(ORGS)) {
      for (const user of ['admin', `${org.key}admin`, `${org.key}user1`]) {
        const identity = await enroll(org, user);
        enrolled.push({ label: identity.label, mspId: identity.mspId });
      }
    }
    return enrolled;
  },
});


function subjectOf(certificatePem) {
  try {
    return new X509Certificate(certificatePem).subject.replace(/\n/g, ', ');
  } catch {
    return null;
  }
}
