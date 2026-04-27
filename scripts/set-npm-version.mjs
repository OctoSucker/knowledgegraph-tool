import fs from "node:fs";
import path from "node:path";

const rawVersion = process.argv[2];
if (!rawVersion) {
  console.error("usage: node scripts/set-npm-version.mjs <version>");
  process.exit(1);
}

const version = rawVersion.startsWith("v") ? rawVersion.slice(1) : rawVersion;
if (!version) {
  console.error("version must be non-empty");
  process.exit(1);
}

const packageJSONPath = path.join(process.cwd(), "npm", "package.json");
const packageJSON = JSON.parse(fs.readFileSync(packageJSONPath, "utf8"));
packageJSON.version = version;
fs.writeFileSync(packageJSONPath, `${JSON.stringify(packageJSON, null, 2)}\n`);
