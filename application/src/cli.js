import { DEFAULT_CHANNEL, DEFAULT_ORG, DEFAULT_USER, requireChannel, requireOrg } from './config.js';

export function parseArgs(argv) {
  const positional = [];
  const flags = {};

  for (let i = 0; i < argv.length; i += 1) {
    const token = argv[i];

    if (!token.startsWith('--')) {
      positional.push(token);
      continue;
    }

    const body = token.slice(2);
    const equals = body.indexOf('=');
    if (equals !== -1) {
      flags[body.slice(0, equals)] = body.slice(equals + 1);
      continue;
    }

    const next = argv[i + 1];
    if (next === undefined || next.startsWith('--')) {
      flags[body] = true;
    } else {
      flags[body] = next;
      i += 1;
    }
  }

  return { command: positional.shift(), positional, flags };
}


export function connectionOptions(flags) {
  const org = requireOrg(String(flags.org ?? DEFAULT_ORG));
  const user = String(flags.user ?? DEFAULT_USER);
  const channel = requireChannel(String(flags.channel ?? DEFAULT_CHANNEL));
  const peerIndex = Number(flags.peer ?? 0);

  if (!Number.isInteger(peerIndex) || peerIndex < 0 || peerIndex > 2) {
    throw new Error(`--peer must be 0, 1 or 2, got '${flags.peer}'`);
  }

  return { org, user, channel, peerIndex };
}

export function requireArg(positional, index, name, usage) {
  const value = positional[index];
  if (value === undefined || value === '') {
    throw new Error(`missing argument <${name}>. Usage: ${usage}`);
  }
  return value;
}

export function requireNumber(value, name) {
  const parsed = Number(value);
  if (!Number.isFinite(parsed)) {
    throw new Error(`argument <${name}> must be a number, got '${value}'`);
  }
  return parsed;
}

/** Reads an integer argument. */
export function requireInteger(value, name) {
  const parsed = requireNumber(value, name);
  if (!Number.isInteger(parsed)) {
    throw new Error(`argument <${name}> must be a whole number, got '${value}'`);
  }
  return parsed;
}


const registry = new Map();

/**
 * Registers a command.
 *
 * @param {object} definition
 * @param {string} definition.name     command name used on the command line
 * @param {string} definition.group    heading it appears under in the help text
 * @param {string} definition.usage    one line usage string
 * @param {string} definition.summary  one line description
 * @param {function} definition.run    async ({positional, flags}) => result
 */
export function registerCommand(definition) {
  registry.set(definition.name, definition);
}

export function getCommand(name) {
  return registry.get(name);
}

export function allCommands() {
  return [...registry.values()];
}

export function commandGroups() {
  const groups = new Map();
  for (const command of registry.values()) {
    if (!groups.has(command.group)) groups.set(command.group, []);
    groups.get(command.group).push(command);
  }
  return groups;
}
