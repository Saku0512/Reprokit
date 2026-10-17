import { build } from "esbuild";
import { readFile } from "node:fs/promises";
import { dirname } from "node:path";
import postcss from "postcss";
import tailwindcss from "@tailwindcss/postcss";

export async function buildFrontend({ write = true, logLevel } = {}) {
  await build({
    entryPoints: {
      app: new URL("../src/learner/main.jsx", import.meta.url).pathname,
      runner: new URL("../src/shared/base.css", import.meta.url).pathname,
      admin: new URL("../src/admin/admin.css", import.meta.url).pathname,
    },
    outdir: new URL("../generated/", import.meta.url).pathname,
    write,
    logLevel,
    bundle: true,
    minify: true,
    format: "esm",
    jsx: "automatic",
    loader: { ".woff": "file", ".woff2": "file" },
    define: { "process.env.NODE_ENV": '"production"' },
    plugins: [
      {
        name: "tailwindcss",
        setup(builder) {
          // 開発配信・静的ビルド・lintで同じCSS変換を使う。
          builder.onLoad({ filter: /\/src\/.*\.css$/ }, async ({ path }) => {
            const source = await readFile(path, "utf8");
            const result = await postcss([tailwindcss({ optimize: false })]).process(source, {
              from: path,
            });
            return { contents: result.css, loader: "css", resolveDir: dirname(path) };
          });
        },
      },
    ],
  });
}
