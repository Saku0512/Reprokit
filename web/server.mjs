import http from "node:http";
import { timingSafeEqual } from "node:crypto";
import { readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";

const files = new Map([
  ["/", ["index.html", "text/html; charset=utf-8"]],
  ["/src/app.js", ["src/app.js", "text/javascript; charset=utf-8"]],
  ["/src/api.js", ["src/api.js", "text/javascript; charset=utf-8"]],
  ["/src/storage.js", ["src/storage.js", "text/javascript; charset=utf-8"]],
  ["/src/style.css", ["src/style.css", "text/css; charset=utf-8"]],
  ["/assets/reprokit-logo.png", ["assets/reprokit-logo.png", "image/png"]],
]);

export function runnerConfig(origin, token) {
  if (!origin && !token) return null;
  const url = new URL(origin);
  if (
    url.protocol !== "http:" ||
    url.hostname !== "127.0.0.1" ||
    url.username ||
    url.password ||
    url.pathname !== "/" ||
    url.search ||
    url.hash ||
    !/^[A-Za-z0-9_-]{16,256}$/.test(token ?? "")
  )
    throw new Error("Runner origin and session token must identify a local HTTP Runner");
  return { origin: url.origin, token };
}

function sameToken(actual, expected) {
  const a = Buffer.from(actual ?? ""),
    b = Buffer.from(expected);
  return a.length === b.length && timingSafeEqual(a, b);
}

// 同一端末のWebシェルだけが固定Runnerへ転送する。任意の転送先をHTTP入力から受け取らない。
export function createWebServer({ runner = null, dist = false } = {}) {
  const root = new URL(dist ? "./dist/" : "./", import.meta.url);
  return http.createServer(async (request, response) => {
    response.setHeader("Cache-Control", "no-store");
    response.setHeader("X-Content-Type-Options", "nosniff");
    response.setHeader("Referrer-Policy", "no-referrer");
    response.setHeader(
      "Content-Security-Policy",
      "default-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self'; img-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'",
    );
    const ownOrigin = `http://127.0.0.1:${request.socket.localPort}`;
    if (
      request.headers.host !== new URL(ownOrigin).host ||
      (request.headers.origin && request.headers.origin !== ownOrigin) ||
      request.headers["sec-fetch-site"] === "cross-site"
    ) {
      response.writeHead(403).end("invalid local origin");
      return;
    }
    const url = new URL(request.url, ownOrigin);
    if (url.origin !== ownOrigin) {
      response.writeHead(403).end("invalid request target");
      return;
    }
    if (url.pathname.startsWith("/api/")) {
      if (!runner) {
        response.writeHead(503).end("ローカルRunnerの接続設定がありません。");
        return;
      }
      if (!sameToken(request.headers["x-reprokit-session"], runner.token)) {
        response.writeHead(401).end("invalid local session");
        return;
      }
      if (
        !/^(GET|POST|PUT|DELETE)$/.test(request.method) ||
        !/^\/api\/(lessons(?:\/detail)?|study-sessions(?:\/[a-f0-9]{32}(?:\/advance|\/(?:hints|answers|labs)\/[a-z0-9-]+)?)?|labs(?:\/[a-f0-9-]{36}(?:\/(?:start|stop|markers\/(?:vulnerable|fixed)))?)?)$/.test(
          url.pathname,
        )
      ) {
        response.writeHead(404).end("unknown study operation");
        return;
      }
      try {
        const chunks = [];
        let bytes = 0;
        for await (const chunk of request) {
          bytes += chunk.length;
          if (bytes > 65536) {
            response.writeHead(413).end("request too large");
            return;
          }
          chunks.push(chunk);
        }
        const upstream = await fetch(`${runner.origin}${url.pathname}${url.search}`, {
          method: request.method,
          headers: {
            "X-Reprokit-Session": runner.token,
            Origin: runner.origin,
            ...(request.headers["content-type"]
              ? { "Content-Type": request.headers["content-type"] }
              : {}),
          },
          body: bytes ? Buffer.concat(chunks) : undefined,
          redirect: "error",
          signal: AbortSignal.timeout(120000),
        });
        response.writeHead(upstream.status, {
          "Content-Type": upstream.headers.get("content-type") ?? "text/plain; charset=utf-8",
        });
        response.end(Buffer.from(await upstream.arrayBuffer()));
      } catch {
        response.writeHead(502).end("Runnerに接続できません。再実行前に状態を確認してください。");
      }
      return;
    }
    const asset = files.get(url.pathname);
    if (!asset || request.method !== "GET" || url.search) {
      response.writeHead(404).end("not found");
      return;
    }
    try {
      let data = await readFile(new URL(asset[0], root));
      if (asset[0] === "index.html" && runner)
        data = Buffer.from(
          data
            .toString()
            .replace(
              'name="reprokit-session" content=""',
              `name="reprokit-session" content="${runner.token}"`,
            ),
        );
      response.writeHead(200, { "Content-Type": asset[1] });
      response.end(data);
    } catch {
      response.writeHead(404).end("asset not built");
    }
  });
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  const port = Number(process.env.REPROKIT_WEB_PORT ?? 4173);
  if (!Number.isInteger(port) || port < 1 || port > 65535) throw new Error("Invalid Web port");
  const runner = runnerConfig(
    process.env.REPROKIT_RUNNER_ORIGIN,
    process.env.REPROKIT_RUNNER_TOKEN,
  );
  const server = createWebServer({ runner, dist: process.argv.includes("--dist") });
  server.listen(port, "127.0.0.1", () =>
    console.log(`Reprokit Web: http://127.0.0.1:${port}${runner ? "" : " (Runner未接続)"}`),
  );
}
