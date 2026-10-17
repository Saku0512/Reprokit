import { build } from "esbuild";

export async function buildFrontend() {
  await build({
    entryPoints: [new URL("../src/sample-app.jsx", import.meta.url).pathname],
    outdir: new URL("../generated/", import.meta.url).pathname,
    entryNames: "app",
    bundle: true,
    minify: true,
    format: "esm",
    jsx: "automatic",
    loader: { ".woff": "file", ".woff2": "file" },
    define: { "process.env.NODE_ENV": '"production"' },
  });
}
