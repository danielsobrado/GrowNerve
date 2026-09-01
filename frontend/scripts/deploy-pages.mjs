import { execFileSync } from "node:child_process";

execFileSync("npx", ["gh-pages", "-d", "dist"], { stdio: "inherit" });
