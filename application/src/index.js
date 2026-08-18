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
