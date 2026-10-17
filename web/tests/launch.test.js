import { test } from "node:test";
import assert from "node:assert/strict";
import { once } from "node:events";
import { mkdtemp, readdir, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { launch, stateDirectory } from "../launch.mjs";

test(
  "Webの起動処理が実Go Runnerへ認証接続し、回答を再起動後も復元する",
  { timeout: 60000 },
  async (t) => {
    const state = await mkdtemp(join(tmpdir(), "reprokit-launch-test-"));
    let app;
    t.after(async () => {
      await app?.close();
      await rm(state, { recursive: true, force: true });
    });
    app = await launch({ port: 0, dist: false, state });
    const tokenFrom = async (origin) => {
      const html = await (await fetch(origin)).text();
      const token = html.match(/name="reprokit-session" content="([^"]+)"/)?.[1];
      assert.ok(token);
      return token;
    };
    let token = await tokenFrom(app.origin);
    const request = async (path, method = "GET", body) => {
      const response = await fetch(`${app.origin}${path}`, {
        method,
        headers: {
          "X-Reprokit-Session": token,
          ...(body ? { "Content-Type": "application/json" } : {}),
        },
        body: body ? JSON.stringify(body) : undefined,
      });
      assert.ok(response.ok, `HTTP ${response.status} for ${path}`);
      return response.json();
    };
    assert.equal((await fetch(`${app.origin}/api/lessons`)).status, 401);
    const catalog = await request("/api/lessons");
    assert.equal(catalog[0].id, "command-injection/filebrowser-auth-hook");
    const session = await request("/api/study-sessions", "POST", {
      lessonId: catalog[0].id,
      lessonVersion: catalog[0].version,
    });
    await request(`/api/study-sessions/${session.id}/answers/trace-boundary`, "PUT", {
      answer: "認証前の外部入力をコマンド文字列へ展開しない。",
    });
    assert.deepEqual(await readdir(join(state, "labs")), []);
    const oldToken = token,
      oldProcess = app.child;
    await app.close();
    assert.notEqual(oldProcess.exitCode ?? oldProcess.signalCode, null);
    app = await launch({ port: 0, dist: false, state });
    token = await tokenFrom(app.origin);
    assert.ok(token !== oldToken, "Runner token must rotate on restart");
    const resumed = await request(`/api/study-sessions/${session.id}`);
    assert.equal(
      resumed.answers["trace-boundary"],
      "認証前の外部入力をコマンド文字列へ展開しない。",
    );
    assert.deepEqual(await readdir(join(state, "labs")), []);
    // 親だけが持つパイプが切れた場合、RunnerとWebの両方が終了する。
    const exited = once(app.child, "exit");
    app.child.stdio[3].destroy();
    await exited;
    assert.equal(app.server.listening, false);
  },
);

test("状態保存先は明示指定またはXDG配下の専用ディレクトリを使う", () => {
  assert.equal(
    stateDirectory({ REPROKIT_STATE_DIR: "/tmp/reprokit-custom" }),
    "/tmp/reprokit-custom",
  );
  assert.equal(stateDirectory({ XDG_STATE_HOME: "/tmp/state" }), "/tmp/state/reprokit");
});
