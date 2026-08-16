// Interactive menu, shown when the application is started without arguments.
//
// The menu is a thin layer over the same commands the command line exposes: it
// collects the arguments, then calls the registered command. Nothing is
// implemented twice, so a fix in a command applies to both faces of the
// application.

import { createInterface } from 'node:readline/promises';
import { stdin, stdout } from 'node:process';

import { CHANNELS, DEFAULT_CHANNEL, DEFAULT_ORG, ORGS } from './config.js';
import { getCommand } from './cli.js';
import { describeError, detail, heading, note, printError, printResult, printTable, success } from './output.js';
import { listIdentities } from './wallet.js';

// The session keeps the identity and channel so they are not retyped every time.
const session = {
  org: DEFAULT_ORG,
  user: 'org1user1',
  channel: DEFAULT_CHANNEL,
};

/**
 * Every menu entry names a command and how to gather its arguments.
 *
 * `prompts` are the positional arguments in order. `flags` are collected as named
 * flags. An entry with neither runs the command straight away.
 */
const ENTRIES = [
  { key: '1', title: 'Enroll or login (choose organization and user)', handler: chooseIdentity },
  { key: '2', title: 'Switch channel', handler: chooseChannel },

  {
    key: '3',
    title: 'Register a merchant',
    command: 'create-merchant',
    prompts: ['merchant id', 'name', 'type (see the catalogue)', 'tax id', 'opening balance'],
  },
  {
    key: '4',
    title: 'Add a product to a merchant',
    command: 'add-product',
    prompts: ['merchant id', 'product code', 'name', 'price', 'quantity', 'expiry date (YYYY-MM-DD, may be empty)'],
  },
  {
    key: '5',
    title: 'Register a customer',
    command: 'create-customer',
    prompts: ['customer id', 'first name', 'last name', 'email', 'opening balance'],
  },
  {
    key: '6',
    title: 'Buy a product',
    command: 'buy',
    prompts: ['customer id', 'merchant id', 'product code', 'quantity'],
  },
  {
    key: '7',
    title: 'Pay money into an account',
    command: 'deposit',
    prompts: ['merchant or customer', 'id', 'amount'],
  },

  { key: '8', title: 'Search products', handler: searchMenu },
  {
    key: '9',
    title: 'List invoices of a customer',
    command: 'invoices',
    flagPrompts: [['customer', 'customer id']],
  },

  { key: '10', title: 'List merchants', command: 'merchants' },
  { key: '11', title: 'List customers', command: 'customers' },
  { key: '12', title: 'List products', command: 'products' },
  { key: '13', title: 'List the merchant type catalogue', command: 'merchant-types' },

  { key: '0', title: 'Exit', handler: () => 'exit' },
];

const SEARCH_ENTRIES = [
  { key: '1', title: 'By name', command: 'search-name', prompts: ['text to look for'] },
  { key: '2', title: 'By code', command: 'search-code', prompts: ['text to look for'] },
  { key: '3', title: 'By merchant type', command: 'search-type', prompts: ['merchant type'] },
  { key: '4', title: 'By price range', command: 'search-price', prompts: ['from', 'to (0 for no limit)'] },
  {
    key: '5',
    title: 'Combined (any mix of the above)',
    command: 'search',
    flagPrompts: [
      ['name', 'name contains (empty to skip)'],
      ['type', 'merchant types, comma separated (empty to skip)'],
      ['price-from', 'minimum price (empty to skip)'],
      ['price-to', 'maximum price (empty to skip)'],
      ['sort', 'sort by price asc or desc (empty to skip)'],
    ],
  },
  { key: '6', title: 'Expiring before a date', command: 'expiring', prompts: ['date (YYYY-MM-DD)'] },
  { key: '0', title: 'Back', handler: () => 'back' },
];

let rl;
let closing;

/** Thrown when stdin ends, so every pending prompt unwinds to runMenu. */
class InputClosed extends Error {}

/**
 * Asks a question.
 *
 * The abort signal matters: readline's promise never settles once stdin has
 * ended, so without it Ctrl-D or a piped script that runs out of lines leaves
 * node warning about an unsettled top level await instead of exiting.
 */
async function ask(prompt) {
  try {
    return await rl.question(prompt, { signal: closing.signal });
  } catch (error) {
    if (error?.name === 'AbortError') throw new InputClosed();
    throw error;
  }
}

export async function runMenu() {
  rl = createInterface({ input: stdin, output: stdout });
  closing = new AbortController();
  rl.on('close', () => closing.abort());

  try {
    printBanner();
    for (;;) {
      const outcome = await showMenu(ENTRIES, 'Main menu');
      if (outcome === 'exit') break;
    }
  } catch (error) {
    if (!(error instanceof InputClosed)) throw error;
    process.stderr.write('\n');
    detail('input ended, exiting');
  } finally {
    rl.close();
  }
  return 0;
}

function printBanner() {
  heading('PDASP trading system, console client');
  detail('Every action runs against the Fabric network through the SDK.');
  detail('Results are printed as JSON.');
}

/** Renders a menu, reads a choice and dispatches it. */
async function showMenu(entries, title) {
  heading(`${title}   [${session.user}@${session.org} on ${session.channel}]`);
  for (const entry of entries) {
    process.stderr.write(`  ${entry.key.padStart(2)}) ${entry.title}\n`);
  }

  const choice = (await ask('\nChoice: ')).trim();
  const entry = entries.find((candidate) => candidate.key === choice);
  if (!entry) {
    printError(`no such option: '${choice}'`);
    return undefined;
  }

  if (entry.handler) {
    return entry.handler();
  }

  await runEntry(entry);
  return undefined;
}

/** Collects the arguments an entry needs and runs its command. */
async function runEntry(entry) {
  const definition = getCommand(entry.command);
  if (!definition) {
    printError(`command '${entry.command}' is not registered`);
    return;
  }

  const positional = [];
  for (const prompt of entry.prompts ?? []) {
    const answer = (await ask(`  ${prompt}: `)).trim();
    // A trailing optional argument left empty simply ends the list.
    if (answer === '' && positional.length >= requiredCount(entry)) break;
    positional.push(answer);
  }

  const flags = { org: session.org, user: session.user, channel: session.channel };
  for (const [flag, prompt] of entry.flagPrompts ?? []) {
    const answer = (await ask(`  ${prompt}: `)).trim();
    if (answer !== '') flags[flag] = answer;
  }

  try {
    note(`${definition.name} ...`);
    const result = await definition.run({ positional, flags });
    if (result !== undefined) {
      printResult(result);
      success('done');
    }
  } catch (error) {
    printError(describeError(error));
  }
}

/**
 * How many of an entry's prompts are mandatory.
 *
 * The optional arguments are always the trailing ones, and the command itself
 * reports a missing mandatory argument, so this only decides when an empty answer
 * means "stop asking" rather than "pass an empty string".
 */
function requiredCount(entry) {
  const optionalTrailing = {
    'create-merchant': 4,
    'create-customer': 4,
    'add-product': 5,
    buy: 3,
  };
  return optionalTrailing[entry.command] ?? (entry.prompts?.length ?? 0);
}

async function searchMenu() {
  for (;;) {
    const outcome = await showMenu(SEARCH_ENTRIES, 'Product search');
    if (outcome === 'back') return undefined;
  }
}

/** Lets the user pick which organization and identity to act as. */
async function chooseIdentity() {
  heading('Identities in the wallet');
  const identities = listIdentities();
  if (identities.length === 0) {
    detail('The wallet is empty. Enrol the standard identities first:');
    detail('  node src/index.js bootstrap');
  } else {
    printTable(identities, [
      { key: 'label', header: 'IDENTITY' },
      { key: 'mspId', header: 'MSP' },
      { key: 'enrolledAt', header: 'ENROLLED' },
    ]);
  }

  detail('');
  for (const org of Object.values(ORGS)) {
    detail(`${org.key} = ${org.label} (${org.mspId})`);
  }

  const org = (await ask(`\n  organization [${session.org}]: `)).trim() || session.org;
  if (!ORGS[org]) {
    printError(`unknown organization '${org}'`);
    return undefined;
  }

  const suggested = `${org}user1`;
  const user = (await ask(`  user [${suggested}]: `)).trim() || suggested;

  session.org = org;
  session.user = user;
  success(`now acting as ${session.user}@${session.org} (${ORGS[org].mspId}, ${ORGS[org].label})`);
  return undefined;
}

async function chooseChannel() {
  detail(`available channels: ${CHANNELS.join(', ')}`);
  const channel = (await ask(`  channel [${session.channel}]: `)).trim() || session.channel;
  if (!CHANNELS.includes(channel)) {
    printError(`unknown channel '${channel}'`);
    return undefined;
  }
  session.channel = channel;
  success(`now working on ${session.channel}`);
  return undefined;
}
