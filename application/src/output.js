// Output formatting and error translation.
//
// The assignment requires the console application to return JSON, so every
// successful result is printed as JSON on stdout. Human readable notes go to
// stderr, which keeps `node src/index.js ... | jq` working in the shell tests.

// Colour is disabled when stderr is not a terminal, so redirected output in the
// shell tests stays free of escape sequences.
const useColor = process.stderr.isTTY && !process.env.NO_COLOR;

const RESET = useColor ? '\u001b[0m' : '';
const RED = useColor ? '\u001b[0;31m' : '';
const GREEN = useColor ? '\u001b[0;32m' : '';
const BLUE = useColor ? '\u001b[0;34m' : '';
const GRAY = useColor ? '\u001b[0;90m' : '';
const BOLD = useColor ? '\u001b[1m' : '';

/** Prints the result as JSON on stdout. */
export function printResult(value, { compact = false } = {}) {
  process.stdout.write(JSON.stringify(value, null, compact ? 0 : 2) + '\n');
}

export function note(message) {
  process.stderr.write(`${BLUE}==>${RESET} ${message}\n`);
}

export function success(message) {
  process.stderr.write(`${GREEN} ok ${RESET} ${message}\n`);
}

export function detail(message) {
  process.stderr.write(`${GRAY}    ${message}${RESET}\n`);
}

export function heading(message) {
  process.stderr.write(`\n${BOLD}${BLUE}${message}${RESET}\n`);
}

export function printError(message) {
  process.stderr.write(`${RED} XX ${RESET}${message}\n`);
}

/**
 * Turns an SDK or chaincode failure into a single readable line.
 *
 * A chaincode error arrives wrapped in gRPC metadata and endorsement details, so
 * the raw message is a wall of text that repeats the same sentence once per
 * endorsing peer. What matters to the user is the sentence the chaincode itself
 * produced, which is what this digs out.
 */
export function describeError(error) {
  const details = error?.details;
  if (Array.isArray(details) && details.length > 0) {
    const messages = details
      .map((entry) => cleanChaincodeMessage(entry.message))
      .filter(Boolean);
    const unique = [...new Set(messages)];
    if (unique.length > 0) {
      return unique.join('; ');
    }
  }

  const message = cleanChaincodeMessage(error?.message) ?? String(error);

  // A peer that is not running produces a bare gRPC transport error, which says
  // nothing about what the user was trying to do.
  if (/UNAVAILABLE|ECONNREFUSED|Connection refused/i.test(message)) {
    return `${message}. Is the network running? Try ./network/network-up.sh`;
  }
  return message;
}

/**
 * Strips the endorsement wrapper from a chaincode error.
 *
 * The peer reports failures as
 *   "chaincode response 500, <message>"
 * or
 *   "transaction returned with failure: <message>"
 * possibly prefixed with the endorsing peer address.
 */
function cleanChaincodeMessage(raw) {
  if (!raw) return null;
  let message = String(raw);

  const patterns = [
    /chaincode response \d+,\s*/i,
    /transaction returned with failure:\s*/i,
    /^error in simulation:\s*/i,
  ];
  for (const pattern of patterns) {
    message = message.replace(pattern, '');
  }
  return message.trim() || null;
}

/** Formats a number as an amount with two decimals, for the interactive menu. */
export function formatAmount(value) {
  return Number(value).toFixed(2);
}

/** Renders an array of objects as a plain text table on stderr. */
export function printTable(rows, columns) {
  if (!Array.isArray(rows) || rows.length === 0) {
    detail('(no records)');
    return;
  }

  const widths = columns.map(({ key, header }) =>
    Math.max(header.length, ...rows.map((row) => String(row[key] ?? '').length)));

  const line = columns
    .map(({ header }, index) => header.padEnd(widths[index]))
    .join('  ');
  process.stderr.write(`${BOLD}${line}${RESET}\n`);

  for (const row of rows) {
    process.stderr.write(columns
      .map(({ key }, index) => String(row[key] ?? '').padEnd(widths[index]))
      .join('  ') + '\n');
  }
}
