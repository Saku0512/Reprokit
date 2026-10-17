export const LESSON_ID = "command-injection/filebrowser-auth-hook";
const studyPattern = /^[0-9a-f]{32}$/;
const labPattern = /^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/;

function identifier(value, pattern) {
  if (!pattern.test(value)) throw new Error("Session IDが不正です。");
  return value;
}

export class ApiError extends Error {
  constructor(status, payload) {
    const message = typeof payload === "string" ? payload : payload?.failure;
    super(message || `Runnerへの接続に失敗しました（HTTP ${status}）。`);
    this.status = status;
    this.session = payload?.session;
  }
}

// 呼び出し先と入力の形を操作ごとに固定し、自由入力を実行設定へ転用しない。
export class StudyApi {
  constructor(token, request = globalThis.fetch.bind(globalThis)) {
    this.token = token;
    this.request = request;
  }

  async call(path, method = "GET", body) {
    const response = await this.request(path, {
      method,
      headers: {
        "X-Reprokit-Session": this.token,
        ...(body === undefined ? {} : { "Content-Type": "application/json" }),
      },
      body: body === undefined ? undefined : JSON.stringify(body),
      credentials: "same-origin",
      cache: "no-store",
      redirect: "error",
    });
    const raw = await response.text();
    let payload;
    try {
      payload = JSON.parse(raw);
    } catch {
      payload = raw;
    }
    if (!response.ok) throw new ApiError(response.status, payload);
    return payload;
  }

  lessons() {
    return this.call("/api/lessons");
  }
  detail(version) {
    const query = new URLSearchParams({ lessonId: LESSON_ID, lessonVersion: version });
    return this.call(`/api/lessons/detail?${query}`);
  }
  create(version) {
    return this.call("/api/study-sessions", "POST", {
      lessonId: LESSON_ID,
      lessonVersion: version,
    });
  }
  study(id) {
    return this.call(`/api/study-sessions/${identifier(id, studyPattern)}`);
  }
  advance(id) {
    return this.call(`/api/study-sessions/${identifier(id, studyPattern)}/advance`, "POST");
  }
  hint(id, step) {
    return this.call(
      `/api/study-sessions/${identifier(id, studyPattern)}/hints/${encodeURIComponent(step)}`,
      "POST",
    );
  }
  answer(id, question, answer) {
    return this.call(
      `/api/study-sessions/${identifier(id, studyPattern)}/answers/${encodeURIComponent(question)}`,
      "PUT",
      { answer },
    );
  }
  binding(id, ref) {
    if (!["vulnerable", "fixed"].includes(ref)) throw new Error("未定義のLab参照です。");
    return this.call(`/api/study-sessions/${identifier(id, studyPattern)}/labs/${ref}`);
  }
  prepare(version, ref) {
    if (!["vulnerable", "fixed"].includes(ref)) throw new Error("未定義のLab参照です。");
    return this.call("/api/labs", "POST", {
      lessonId: LESSON_ID,
      lessonVersion: version,
      labRef: ref,
    });
  }
  status(id) {
    return this.call(`/api/labs/${identifier(id, labPattern)}`);
  }
  start(id) {
    return this.call(`/api/labs/${identifier(id, labPattern)}/start`, "POST", { confirmed: true });
  }
  stop(id) {
    return this.call(`/api/labs/${identifier(id, labPattern)}/stop`, "POST");
  }
  delete(id, policy) {
    if (!["retain", "delete"].includes(policy)) throw new Error("Volume方針が不正です。");
    return this.call(`/api/labs/${identifier(id, labPattern)}?volumePolicy=${policy}`, "DELETE");
  }
}

export const stateLabels = {
  absent: "未準備",
  preparing: "取得・検査中",
  prepared: "準備済み",
  starting: "起動中",
  running: "稼働中",
  stopping: "停止中",
  stopped: "停止済み",
  deleting: "削除中",
  deleted: "削除済み",
  error: "失敗",
};

export function canStart(lab) {
  return (
    ["prepared", "stopped"].includes(lab?.state) &&
    lab.safetyChecked === true &&
    lab.externalExposure === false
  );
}

export function endpointURL(endpoint) {
  if (
    endpoint.address !== "127.0.0.1" ||
    !Number.isInteger(endpoint.port) ||
    endpoint.port < 1 ||
    endpoint.port > 65535
  )
    return null;
  return `http://127.0.0.1:${endpoint.port}`;
}

export function hasComparisonEndpoints(lab) {
  return ["vulnerable", "fixed"].every((ref) =>
    lab?.endpoints?.some((endpoint) => endpoint.service === ref && endpointURL(endpoint)),
  );
}
