import { cp, mkdir, rm } from "node:fs/promises";
const root = new URL("../", import.meta.url);
await rm(new URL("dist/", root), { recursive: true, force: true });
await mkdir(new URL("dist/", root));
await cp(new URL("index.html", root), new URL("dist/index.html", root));
await cp(new URL("admin.html", root), new URL("dist/admin.html", root));
await cp(new URL("src/", root), new URL("dist/src/", root), { recursive: true });
await cp(new URL("assets/", root), new URL("dist/assets/", root), { recursive: true });
console.log("build: learner and admin pages generated (no credentials or lesson answers)");
