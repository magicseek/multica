#!/usr/bin/env node
// Rebrand the bundled Electron.app's Info.plist so `pnpm dev:desktop`
// shows "Multica Canary" in the menu bar, Cmd+Tab switcher, and
// Activity Monitor. On macOS these titles come from CFBundleName at
// launch time — `app.setName()` cannot override them at runtime, so
// patching the plist in node_modules is the only working fix.
//
// Idempotent: runs on every dev launch and no-ops once the plist already
// matches. The patch is isolated to this worktree's node_modules — we
// unlink the file before rewriting so we never mutate a pnpm-store inode
// shared with another project.

import { createRequire } from "node:module";
import { execFileSync } from "node:child_process";
import {
  copyFileSync,
  existsSync,
  readFileSync,
  unlinkSync,
  writeFileSync,
} from "node:fs";
import { resolve } from "node:path";
import { fileURLToPath } from "node:url";

if (process.platform !== "darwin") process.exit(0);

const DESIRED_NAME = "Multica Canary";
const DESIRED_ICON_FILE = "multica-dev.icns";
const DESIRED_ICON_SOURCE = fileURLToPath(
  new URL("../build/icon.icns", import.meta.url),
);

const require = createRequire(import.meta.url);
// `require('electron')` returns the path to the executable
// (.../Electron.app/Contents/MacOS/Electron). Walk up to Contents/Info.plist.
const electronBin = require("electron");
const plistPath = resolve(electronBin, "../../Info.plist");
const resourcesPath = resolve(electronBin, "../../Resources");
const iconPath = resolve(resourcesPath, DESIRED_ICON_FILE);
const appBundlePath = resolve(electronBin, "../../..");

function plistGet(key) {
  try {
    return execFileSync(
      "/usr/libexec/PlistBuddy",
      ["-c", `Print :${key}`, plistPath],
      { encoding: "utf8", stdio: ["ignore", "pipe", "ignore"] },
    ).trim();
  } catch {
    return "";
  }
}

function plistSet(key, value) {
  try {
    execFileSync("/usr/libexec/PlistBuddy", [
      "-c",
      `Set :${key} ${value}`,
      plistPath,
    ]);
  } catch {
    execFileSync("/usr/libexec/PlistBuddy", [
      "-c",
      `Add :${key} string ${value}`,
      plistPath,
    ]);
  }
}

if (
  plistGet("CFBundleName") === DESIRED_NAME &&
  plistGet("CFBundleDisplayName") === DESIRED_NAME &&
  plistGet("CFBundleIconFile") === DESIRED_ICON_FILE &&
  fileEquals(iconPath, DESIRED_ICON_SOURCE)
) {
  process.exit(0);
}

// Break any pnpm hardlink to the global store: read, unlink, rewrite.
// PlistBuddy would otherwise write through the hardlink and mutate the
// shared store file (and every other project's Electron.app with it).
const original = readFileSync(plistPath);
unlinkSync(plistPath);
writeFileSync(plistPath, original);

plistSet("CFBundleName", DESIRED_NAME);
plistSet("CFBundleDisplayName", DESIRED_NAME);
plistSet("CFBundleIconFile", DESIRED_ICON_FILE);

if (existsSync(iconPath)) unlinkSync(iconPath);
copyFileSync(DESIRED_ICON_SOURCE, iconPath);

// Nudge macOS icon services to re-read the dev Electron.app bundle.
execFileSync("/usr/bin/touch", ["-c", appBundlePath]);

console.log(
  `[brand-dev-electron] ${plistPath} → CFBundleName="${DESIRED_NAME}", CFBundleIconFile="${DESIRED_ICON_FILE}"`,
);

function fileEquals(left, right) {
  try {
    return readFileSync(left).equals(readFileSync(right));
  } catch {
    return false;
  }
}
