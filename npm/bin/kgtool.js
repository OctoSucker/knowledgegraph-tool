#!/usr/bin/env node

const { spawn } = require("node:child_process");
const fs = require("node:fs");
const path = require("node:path");

const isWin = process.platform === "win32";
const binaryName = isWin ? "kgtool.exe" : "kgtool";
const binaryPath = path.join(__dirname, "..", "vendor", binaryName);

if (!fs.existsSync(binaryPath)) {
  console.error(
    "knowledgegraph-tool: binary is missing. Reinstall the package or run npm rebuild knowledgegraph-tool."
  );
  process.exit(1);
}

const child = spawn(binaryPath, process.argv.slice(2), {
  stdio: "inherit"
});

child.on("exit", (code, signal) => {
  if (signal) {
    process.kill(process.pid, signal);
    return;
  }
  process.exit(code == null ? 1 : code);
});

child.on("error", (err) => {
  console.error(`knowledgegraph-tool: failed to start binary: ${err.message}`);
  process.exit(1);
});
