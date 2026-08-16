#!/usr/bin/env node
//
// Console application for the PDASP trading system.
//
//   node src/index.js                 list the available commands
//   node src/index.js help <command>  help for one command
//   node src/index.js <command> ...   run one command and exit
//
// Every command prints its result as JSON on stdout and its progress notes on
// stderr, so the shell test scripts can pipe the output straight into jq.
//
// This file is only a bootstrap. fabric-common, pulled in by fabric-ca-client,
// initialises its configuration by parsing process.argv with yargs at import
// time, and yargs treats a bare `help` argument as its own command: it prints its
// usage and exits before our dispatcher ever runs. Stashing the arguments and
// blanking process.argv before anything else is imported keeps our command line
// ours. ES module imports are hoisted, so this has to happen in a module that
// imports nothing statically.

const args = process.argv.slice(2);
process.argv = process.argv.slice(0, 2);

const { run, EXIT_FAILURE, EXIT_OK } = await import('./main.js');
const { describeError, printError } = await import('./output.js');

try {
  process.exit((await run(args)) ?? EXIT_OK);
} catch (error) {
  printError(describeError(error));
  if (process.env.PDASP_DEBUG) {
    console.error(error);
  }
  process.exit(EXIT_FAILURE);
}
