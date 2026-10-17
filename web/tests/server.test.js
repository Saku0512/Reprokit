import { test } from "node:test";
import assert from "node:assert/strict";
import http from "node:http";
import { createWebServer, runnerConfig } from "../server.mjs";

const token = "test_token_for_local_session";
async function start(server, context) {
  await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
  context.after(
    () =>
      new Promise((resolve) => {
        server.closeAllConnections();
        server.close(resolve);
      }),
  );
  return `http://127.0.0.1:${server.address().port}`;
}

test("接続先とトークンは起動設定で固定し、外部URLを拒否する", () => {
  assert.equal(runnerConfig(undefined, undefined), null);
  assert.deepEqual(runnerConfig("http://127.0.0.1:9123", token), {
    origin: "http://127.0.0.1:9123",
    token,
  });
  for (const url of [
    "https://example.com",
    "http://localhost:9123",
    "http://127.0.0.1:9123/api",
    "http://user@127.0.0.1:9123",
    "http://127.0.0.1:9123/#x",
  ])
    assert.throws(() => runnerConfig(url, token));
  assert.throws(() => runnerConfig("http://127.0.0.1:9123", '<script>"'));
});

test("静的ファイルを限定し、API未設定を成功表示しない", async (t) => {
  const origin = await start(createWebServer(), t);
  const page = await fetch(origin);
  assert.equal(page.status, 200);
  assert.match(await page.text(), /<html lang="ja">/);
  assert.match(page.headers.get("content-security-policy"), /connect-src 'self'/);
  assert.equal((await fetch(`${origin}/api/lessons`)).status, 503);
  assert.equal((await fetch(`${origin}/package.json`)).status, 404);
  assert.equal((await fetch(`${origin}/src/../server.mjs`)).status, 404);
});

test("同一Originとトークンを検証して既存APIだけをRunnerへ転送する", async (t) => {
  const received = [];
  const upstream = await start(
    http.createServer(async (req, res) => {
      let body = "";
      for await (const chunk of req) body += chunk;
      received.push({
        url: req.url,
        host: req.headers.host,
        origin: req.headers.origin,
        token: req.headers["x-reprokit-session"],
        body,
      });
      res.setHeader("Content-Type", "application/json");
      res.end(JSON.stringify({ ok: true }));
    }),
    t,
  );
  const origin = await start(createWebServer({ runner: runnerConfig(upstream, token) }), t);
  const headers = {
    "X-Reprokit-Session": token,
    Origin: origin,
    "Content-Type": "application/json",
  };
  assert.equal(
    (
      await fetch(`${origin}/api/labs`, {
        method: "POST",
        headers,
        body: JSON.stringify({ lessonId: "fixed", lessonVersion: "1.0.0", labRef: "vulnerable" }),
      })
    ).status,
    200,
  );
  assert.equal(received[0].origin, upstream);
  assert.equal(received[0].host, new URL(upstream).host);
  assert.equal(received[0].token, token);
  assert.equal(received.length, 1);
  assert.equal(
    (await fetch(`${origin}/api/lessons`, { headers: { Origin: "https://evil.test" } })).status,
    403,
  );
  assert.equal((await fetch(`${origin}/api/lessons`)).status, 401);
  assert.equal((await fetch(`${origin}/api/shell`, { method: "POST", headers })).status, 404);
  // fetchがHostを書き換える環境があるため、生のHTTP要求で拒否を検証する。
  const rejectedHost = await new Promise((resolve, reject) => {
    http
      .get(origin, { headers: { Host: "evil.test" } }, (response) => {
        response.resume();
        resolve(response.statusCode);
      })
      .on("error", reject);
  });
  assert.equal(rejectedHost, 403);
  assert.equal(received.length, 1);
  const page = await (await fetch(origin)).text();
  assert.ok(page.includes(`name="reprokit-session" content="${token}"`));
});

test("転送先のリダイレクトをたどらず、失敗を表示できる", async (t) => {
  const upstream = await start(
    http.createServer((req, res) => res.writeHead(302, { Location: "https://example.com" }).end()),
    t,
  );
  const origin = await start(createWebServer({ runner: runnerConfig(upstream, token) }), t);
  const response = await fetch(`${origin}/api/lessons`, {
    headers: { "X-Reprokit-Session": token },
  });
  assert.equal(response.status, 502);
});
