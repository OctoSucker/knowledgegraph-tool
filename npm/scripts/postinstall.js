const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const https = require("node:https");
const { spawnSync } = require("node:child_process");

const pkg = require("../package.json");

const repo = process.env.KGTOOL_REPOSITORY || "OctoSucker/knowledgegraph-tool";
const version = process.env.KGTOOL_VERSION || pkg.version;
const baseURL =
  process.env.KGTOOL_RELEASE_BASE_URL ||
  `https://github.com/${repo}/releases/download/v${version}`;

const platformMap = {
  darwin: "darwin",
  linux: "linux",
  win32: "windows"
};

const archMap = {
  x64: "amd64",
  arm64: "arm64"
};

const platform = platformMap[process.platform];
const arch = archMap[process.arch];

if (!platform || !arch) {
  console.error(
    `knowledgegraph-tool: unsupported platform ${process.platform}/${process.arch}`
  );
  process.exit(1);
}

const ext = platform === "windows" ? "zip" : "tar.gz";
const archive = `knowledgegraph-tool_${version}_${platform}_${arch}.${ext}`;
const downloadURL = `${baseURL}/${archive}`;

const vendorDir = path.join(__dirname, "..", "vendor");
const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "kgtool-"));
const archivePath = path.join(tmpDir, archive);
const binaryName = platform === "windows" ? "kgtool.exe" : "kgtool";
const binaryPath = path.join(vendorDir, binaryName);

fs.mkdirSync(vendorDir, { recursive: true });

download(downloadURL, archivePath)
  .then(() => extract(archivePath, tmpDir, ext))
  .then(() => installBinary(tmpDir, binaryPath))
  .then(() => {
    fs.rmSync(tmpDir, { recursive: true, force: true });
  })
  .catch((err) => {
    console.error(`knowledgegraph-tool: install failed: ${err.message}`);
    process.exit(1);
  });

function download(url, dest) {
  return new Promise((resolve, reject) => {
    const request = https.get(url, (res) => {
      if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
        res.resume();
        download(res.headers.location, dest).then(resolve).catch(reject);
        return;
      }
      if (res.statusCode !== 200) {
        reject(new Error(`download ${url} returned HTTP ${res.statusCode}`));
        res.resume();
        return;
      }
      const file = fs.createWriteStream(dest);
      res.pipe(file);
      file.on("finish", () => file.close(resolve));
      file.on("error", reject);
    });
    request.on("error", reject);
  });
}

function extract(archivePath, tmpDir, ext) {
  if (ext === "zip") {
    run("unzip", ["-o", archivePath, "-d", tmpDir]);
    return;
  }
  run("tar", ["-xzf", archivePath, "-C", tmpDir]);
}

function installBinary(tmpDir, dest) {
  const source = findBinary(tmpDir, path.basename(dest));
  if (!source) {
    throw new Error(`binary ${path.basename(dest)} not found in archive`);
  }
  fs.copyFileSync(source, dest);
  fs.chmodSync(dest, 0o755);
}

function findBinary(dir, name) {
  const entries = fs.readdirSync(dir, { withFileTypes: true });
  for (const entry of entries) {
    const fullPath = path.join(dir, entry.name);
    if (entry.isFile() && entry.name === name) {
      return fullPath;
    }
    if (entry.isDirectory()) {
      const match = findBinary(fullPath, name);
      if (match) {
        return match;
      }
    }
  }
  return null;
}

function run(command, args) {
  const result = spawnSync(command, args, { stdio: "inherit" });
  if (result.status !== 0) {
    throw new Error(`${command} ${args.join(" ")} failed`);
  }
}
