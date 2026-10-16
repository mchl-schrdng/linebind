import { readdirSync } from "node:fs";
import { join } from "node:path";
import { execFile } from "node:child_process";
import { promisify } from "node:util";

const execute = promisify(execFile);
const files = [];

function collect(directory) {
  for (const entry of readdirSync(directory, { withFileTypes: true })) {
    const path = join(directory, entry.name);
    if (entry.isDirectory()) collect(path);
    else if (entry.isFile()) {
      if (!/\.(?:c|m)?js$/.test(entry.name)) {
        console.error(`This example's src tree accepts only .js, .mjs and .cjs modules: ${path}`);
        process.exit(2);
      }
      files.push(path);
    }
  }
}

collect("src");
let next = 0;
let exitCode = 0;
await Promise.all(Array.from({ length: 4 }, async () => {
  while (exitCode === 0 && next < files.length) {
    const path = files[next++];
    try {
      await execute(process.execPath, ["--check", path]);
    } catch (error) {
      process.stderr.write(error.stderr || `${error.message}\n`);
      exitCode = Math.max(exitCode, error.code === 1 ? 1 : 2);
    }
  }
}));
process.exitCode = exitCode;
