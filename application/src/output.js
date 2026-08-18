const useColor = process.stderr.isTTY && !process.env.NO_COLOR;

const RESET = useColor ? '\u001b[0m' : '';
const RED = useColor ? '\u001b[0;31m' : '';
const GREEN = useColor ? '\u001b[0;32m' : '';
const BLUE = useColor ? '\u001b[0;34m' : '';
const GRAY = useColor ? '\u001b[0;90m' : '';
const BOLD = useColor ? '\u001b[1m' : '';

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

  if (/UNAVAILABLE|ECONNREFUSED|Connection refused/i.test(message)) {
    return `${message}. Is the network running? Try ./network/network-up.sh`;
  }
  return message;
}

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

export function formatAmount(value) {
  return Number(value).toFixed(2);
}

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
