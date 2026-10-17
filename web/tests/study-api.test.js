import { test } from "node:test";
import assert from "node:assert/strict";
import {
  StudyApi,
  LESSON_ID,
  ApiError,
  canStart,
  endpointURL,
  hasComparisonEndpoints,
} from "../src/shared/study-api.js";

const studyId = "a".repeat(32);
const labId = "11111111-1111-4111-8111-111111111111";

test("Labの準備は固定参照だけを送り、Startを呼ばない", async () => {
  const requests = [];
  const api = new StudyApi("secret", async (path, init) => {
    requests.push({ path, ...init });
    return new Response("{}");
  });
  await api.prepare("1.0.0", "vulnerable");
  assert.equal(requests.length, 1);
  assert.equal(requests[0].path, "/api/labs");
  assert.deepEqual(JSON.parse(requests[0].body), {
    lessonId: LESSON_ID,
    lessonVersion: "1.0.0",
    labRef: "vulnerable",
  });
  assert.equal(requests[0].headers["X-Reprokit-Session"], "secret");
  assert.equal(requests[0].redirect, "error");
});

test("定義済みのライフサイクル操作・Volume方針だけを使う", async () => {
  const requests = [];
  const api = new StudyApi("secret", async (path, init) => {
    requests.push([path, init]);
    return new Response("{}");
  });
  await api.start(labId);
  await api.stop(labId);
  await api.delete(labId, "retain");
  await api.delete(labId, "delete");
  assert.deepEqual(JSON.parse(requests[0][1].body), { confirmed: true });
  assert.equal(requests[1][1].body, undefined);
  assert.equal(requests[2][0], `/api/labs/${labId}?volumePolicy=retain`);
  assert.equal(requests[3][0], `/api/labs/${labId}?volumePolicy=delete`);
  assert.throws(() => api.prepare("1.0.0", "http://evil.test"));
  assert.throws(() => api.start("../../shell"));
  assert.throws(() => api.delete(labId, "--volumes"));
  assert.equal(requests.length, 4);
});

test("コンテナログは固定比較対象だけを読める", async () => {
  const requests = [];
  const api = new StudyApi("secret", async (path, init) => {
    requests.push([path, init]);
    return new Response('{"service":"vulnerable","content":"login failed\\n"}');
  });
  const logs = await api.logs(labId, "vulnerable");
  assert.equal(requests[0][0], `/api/labs/${labId}/logs/vulnerable`);
  assert.equal(requests[0][1].method, "GET");
  assert.equal(logs.content, "login failed\n");
  assert.throws(() => api.logs(labId, "reprokit_proxy_vulnerable_1"));
  assert.equal(requests.length, 1);
});

test("検証アクションは固定の/tmp一覧だけを実行する", async () => {
  const requests = [];
  const api = new StudyApi("secret", async (path, init) => {
    requests.push([path, init]);
    return new Response('{"service":"vulnerable","action":"list_tmp","output":"success\\n"}');
  });
  const result = await api.listTmp(labId, "vulnerable");
  assert.equal(requests[0][0], `/api/labs/${labId}/actions/vulnerable/list_tmp`);
  assert.equal(requests[0][1].method, "POST");
  assert.equal(requests[0][1].body, undefined);
  assert.equal(result.action, "list_tmp");
  assert.throws(() => api.listTmp(labId, "reprokit_proxy_vulnerable_1"));
});

test("回答は受講APIのanswerだけに入る", async () => {
  let actual;
  const api = new StudyApi("secret", async (path, init) => {
    actual = [path, init];
    return new Response("{}");
  });
  const answer = "入力をシェルで解釈させない。<script>alert(1)</script>";
  await api.answer(studyId, "trace-boundary", answer);
  assert.equal(actual[0], `/api/study-sessions/${studyId}/answers/trace-boundary`);
  assert.equal(actual[1].method, "PUT");
  assert.deepEqual(JSON.parse(actual[1].body), { answer });
});

test("研修提出は保存済みStudy Session IDだけを送る", async () => {
  const requests = [];
  const api = new StudyApi("secret", async (path, init) => {
    requests.push([path, init]);
    return new Response("{}");
  });
  await api.trainingAssignment();
  await api.startTraining();
  await api.submitTraining(studyId);
  assert.equal(requests[0][0], "/api/training/learner/assignment");
  assert.deepEqual(JSON.parse(requests[1][1].body), {});
  assert.deepEqual(JSON.parse(requests[2][1].body), { studySessionId: studyId });
});

test("Runnerの失敗レスポンスからSessionと理由を失わない", async () => {
  const session = {
    labSessionId: labId,
    state: "error",
    failure: { operation: "start", message: "digest mismatch" },
  };
  const api = new StudyApi(
    "secret",
    async () =>
      new Response(JSON.stringify({ session, failure: "digest mismatch" }), { status: 500 }),
  );
  await assert.rejects(
    api.start(labId),
    (error) =>
      error instanceof ApiError &&
      error.message === "digest mismatch" &&
      error.session.state === "error",
  );
});

test("安全性とloopbackが確認できたLabだけを起動・接続できる", () => {
  const safe = { state: "prepared", safetyChecked: true, externalExposure: false };
  assert.equal(canStart(safe), true);
  for (const lab of [
    { ...safe, state: "running" },
    { ...safe, safetyChecked: false },
    { ...safe, externalExposure: true },
    { state: "prepared" },
  ])
    assert.equal(canStart(lab), false);
  assert.equal(endpointURL({ address: "127.0.0.1", port: 18080 }), "http://127.0.0.1:18080");
  for (const address of ["0.0.0.0", "example.com", "127.0.0.1@evil.test", "[::1]"])
    assert.equal(endpointURL({ address, port: 80 }), null);
  assert.equal(endpointURL({ address: "127.0.0.1", port: 65536 }), null);
});

test("同じSession内の両版はservice名と安全な接続先の両方で確認する", () => {
  const endpoints = [
    { service: "vulnerable", address: "127.0.0.1", port: 8080 },
    { service: "fixed", address: "127.0.0.1", port: 8081 },
  ];
  assert.equal(hasComparisonEndpoints({ endpoints }), true);
  assert.equal(hasComparisonEndpoints({ endpoints: endpoints.slice(0, 1) }), false);
  assert.equal(
    hasComparisonEndpoints({
      endpoints: [endpoints[0], { ...endpoints[1], address: "example.com" }],
    }),
    false,
  );
  assert.equal(
    hasComparisonEndpoints({ endpoints: [endpoints[0], { ...endpoints[1], service: "unknown" }] }),
    false,
  );
});
