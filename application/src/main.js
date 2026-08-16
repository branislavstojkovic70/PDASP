// Command dispatch for the console application.
//
// Loaded by index.js only after process.argv has been stashed away, see the
// comment there.

import { allCommands, commandGroups, getCommand, parseArgs } from './cli.js';
import { describeError, printError, printResult } from './output.js';

// Importing a command module registers its commands as a side effect.
import './commands/identity.js';
import './commands/invoke.js';
import './commands/query.js';

export const EXIT_OK = 0;
export const EXIT_FAILURE = 1;
export const EXIT_USAGE = 2;

/**
 * Runs one command.
 *
 * @param {string[]} argv arguments after the script name
 * @returns {Promise<number>} process exit code
 */
export async function run(argv) {
  // No arguments means the interactive menu. It is imported lazily so that a
  // single command invocation does not pay for readline or the menu tables.
  if (argv.length === 0) {
    const { runMenu } = await import('./menu.js');
    return runMenu();
  }

  const { command, positional, flags } = parseArgs(argv);

  if (command === 'help' || flags.help) {
    printHelp(positional[0]);
    return EXIT_OK;
  }

  const definition = getCommand(command);
  if (!definition) {
    printError(`unknown command '${command}'`);
    suggest(command);
    return EXIT_USAGE;
  }

  const result = await definition.run({ positional, flags });
  if (result !== undefined) {
    printResult(result, { compact: Boolean(flags.compact) });
  }
  return EXIT_OK;
}

function printHelp(commandName) {
  if (commandName) {
    const definition = getCommand(commandName);
    if (!definition) {
      printError(`unknown command '${commandName}'`);
      return;
    }
    process.stderr.write(`\n  ${definition.usage}\n      ${definition.summary}\n\n`);
    return;
  }

  process.stderr.write(`
PDASP trading system, console client

  node src/index.js                  start the interactive menu
  node src/index.js <command> ...    run a single command
  node src/index.js help <command>   help for one command

Common flags (any command that talks to the ledger):

  --org <org1|org2|org3>   organization whose identity is used   (default org1)
  --user <id>              wallet identity                       (default org1user1)
  --channel <name>         channel1 or channel2                  (default channel1)
  --peer <0|1|2>           which peer of the organization to use  (default 0)
  --compact                print the JSON result on one line

`);

  for (const [group, commands] of commandGroups()) {
    process.stderr.write(`${group}\n`);
    for (const command of commands) {
      process.stderr.write(`  ${command.usage}\n      ${command.summary}\n`);
    }
    process.stderr.write('\n');
  }
}

/** Points at the closest command name, since a typo is the usual cause. */
function suggest(name) {
  const candidates = allCommands()
    .map((command) => command.name)
    .filter((candidate) => candidate.startsWith(String(name).slice(0, 3)));

  if (candidates.length > 0) {
    printError(`did you mean: ${candidates.join(', ')}?`);
  }
  printError('run `node src/index.js help` for the full list');
}
