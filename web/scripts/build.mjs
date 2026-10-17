import { buildFrontend } from "./frontend.mjs";
import { cp, mkdir, rm } from "node:fs/promises";
const root = new URL("../", import.meta.url);
await buildFrontend();
await rm(new URL("dist/", root), { recursive: true, force: true });
await mkdir(new URL("dist/", root));
await cp(new URL("index.html", root), new URL("dist/index.html", root));
await cp(new URL("src/", root), new URL("dist/src/", root), { recursive: true });
await cp(new URL("assets/", root), new URL("dist/assets/", root), { recursive: true });
console.log(
  "build: dist/index.html, dist/src, and dist/assets generated (no credentials or lesson answers)",
);
await cp(new URL("runner.html", root), new URL("dist/runner.html", root));
await cp(new URL("generated/", root), new URL("dist/generated/", root), { recursive: true });
await mkdir(new URL("dist/licenses/", root));
for (const font of ["noto-sans-jp", "ibm-plex-mono"]) {
  await cp(
    new URL(`node_modules/@fontsource/${font}/LICENSE`, root),
    new URL(`dist/licenses/${font}.txt`, root),
  );
}
console.log("build: React sample, Runner learner and admin pages generated");
