import { spawnSync } from "node:child_process";
import {
  copyFileSync,
  cpSync,
  existsSync,
  mkdtempSync,
  readdirSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const SCRIPT_DIR = dirname(fileURLToPath(import.meta.url));
const FRONTEND_DIR = resolve(SCRIPT_DIR, "..");
const REPOSITORY_DIR = resolve(FRONTEND_DIR, "..");
const DIST_DIR = join(FRONTEND_DIR, "dist");
const NPM = process.platform === "win32" ? "npm.cmd" : "npm";

const CONFIG = Object.freeze({
  sourceBranch: "main",
  pagesBranch: "gh-pages",
  remote: "origin",
  basePath: "/GrowNerve/",
  pagesUrl: "https://danielsobrado.github.io/GrowNerve/",
});

function run(command, args, cwd, extraEnv = {}) {
  const result = spawnSync(command, args, {
    cwd,
    env: { ...process.env, ...extraEnv },
    stdio: "inherit",
  });

  if (result.error) throw result.error;
  if (result.status !== 0) {
    throw new Error(`${command} ${args.join(" ")} failed with exit code ${result.status ?? "unknown"}.`);
  }
}

function capture(command, args, cwd) {
  const result = spawnSync(command, args, { cwd, encoding: "utf8" });
  if (result.error) throw result.error;
  if (result.status !== 0) {
    throw new Error(result.stderr?.trim() || `${command} ${args.join(" ")} failed.`);
  }
  return result.stdout.trim();
}

function assertCleanRepository() {
  const changes = capture("git", ["status", "--porcelain"], REPOSITORY_DIR);
  if (changes) {
    throw new Error("The repository has uncommitted changes. Commit or stash them before publishing GitHub Pages.");
  }
}

function clearWorktree(directory) {
  for (const entry of readdirSync(directory)) {
    if (entry === ".git") continue;
    rmSync(join(directory, entry), { recursive: true, force: true });
  }
}

function copyBuild(directory) {
  for (const entry of readdirSync(DIST_DIR)) {
    cpSync(join(DIST_DIR, entry), join(directory, entry), { recursive: true });
  }

  const indexFile = join(directory, "index.html");
  const fallbackFile = join(directory, "404.html");
  if (!existsSync(fallbackFile)) copyFileSync(indexFile, fallbackFile);
  writeFileSync(join(directory, ".nojekyll"), "", "utf8");
}

function main() {
  const branch = capture("git", ["branch", "--show-current"], REPOSITORY_DIR);
  if (branch !== CONFIG.sourceBranch) {
    throw new Error(`GitHub Pages must be published from ${CONFIG.sourceBranch}; current branch is ${branch || "detached HEAD"}.`);
  }

  assertCleanRepository();

  console.log("Installing frontend dependencies for the browser build...");
  run(NPM, ["install", "--package-lock=false", "--no-audit", "--no-fund"], FRONTEND_DIR);

  console.log("Building GrowNerve for GitHub Pages...");
  run(NPM, ["run", "build:browser"], FRONTEND_DIR, { VITE_BASE_PATH: CONFIG.basePath });

  assertCleanRepository();

  const indexFile = join(DIST_DIR, "index.html");
  if (!existsSync(indexFile)) throw new Error(`Browser build did not produce ${indexFile}.`);

  const sourceCommit = capture("git", ["rev-parse", "HEAD"], REPOSITORY_DIR);
  const sourceCommitShort = capture("git", ["rev-parse", "--short", "HEAD"], REPOSITORY_DIR);

  console.log(`Fetching ${CONFIG.pagesBranch}...`);
  run(
    "git",
    ["fetch", CONFIG.remote, `${CONFIG.pagesBranch}:refs/remotes/${CONFIG.remote}/${CONFIG.pagesBranch}`],
    REPOSITORY_DIR,
  );

  const temporaryRoot = mkdtempSync(join(tmpdir(), "grownerve-pages-"));
  const worktree = join(temporaryRoot, "site");
  let worktreeAdded = false;

  try {
    run(
      "git",
      ["worktree", "add", "--detach", worktree, `refs/remotes/${CONFIG.remote}/${CONFIG.pagesBranch}`],
      REPOSITORY_DIR,
    );
    worktreeAdded = true;

    clearWorktree(worktree);
    copyBuild(worktree);

    run("git", ["add", "-A"], worktree);
    const changes = capture("git", ["status", "--porcelain"], worktree);
    if (!changes) {
      console.log("GitHub Pages is already up to date.");
      return;
    }

    run("git", ["commit", "-m", `deploy: GrowNerve ${sourceCommitShort}`], worktree);
    run("git", ["push", CONFIG.remote, `HEAD:${CONFIG.pagesBranch}`], worktree);

    console.log(`Published ${sourceCommit} to ${CONFIG.pagesUrl}`);
  } finally {
    if (worktreeAdded) {
      const cleanup = spawnSync("git", ["worktree", "remove", "--force", worktree], {
        cwd: REPOSITORY_DIR,
        stdio: "inherit",
      });
      if (cleanup.error) console.error("Failed to remove temporary Git worktree.", cleanup.error);
    }
    rmSync(temporaryRoot, { recursive: true, force: true });
  }
}

try {
  main();
} catch (error) {
  console.error(error instanceof Error ? error.message : error);
  process.exitCode = 1;
}
