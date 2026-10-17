import { readdir } from "node:fs/promises";
import { spawnSync } from "node:child_process";
const root = new URL("../", import.meta.url);
const files = await readdir(root, { recursive: true });
for (const file of files.filter(
  (file) =>
    /\.(m?js)$/.test(file) && !/^(node_modules|dist|test-results|playwright-report)\//.test(file),
)) {
  const result = spawnSync(process.execPath, ["--check", new URL(file, root).pathname], {
    stdio: "inherit",
  });
  if (result.status !== 0) process.exit(1);
}
console.log("lint: JavaScript syntax checks passed");
