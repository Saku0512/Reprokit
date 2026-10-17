import {
  STATUS_META,
  answerPresentation,
  assignmentSummary,
  completionDescription,
  filterAssignments,
  materialForAssignment,
  parseLearnerIDs,
} from "./assignment-model.js";

const loginPanel = document.querySelector("#login-panel");
const dashboard = document.querySelector("#dashboard");
const notice = document.querySelector("#admin-notice");
const assignmentToast = document.querySelector("#assignment-toast");
let assignmentToastTimer;
const assignmentForm = document.querySelector("#assignment-form");
const materialSelect = document.querySelector("#material");
const dueInput = document.querySelector("#due");
const statusFilter = document.querySelector("#status-filter");
const assignmentSearch = document.querySelector("#assignment-search");
let materials = [];
let assignments = [];
let selectedAssignmentID = "";

async function call(path, options = {}) {
  const response = await fetch(path, {
    credentials: "same-origin",
    cache: "no-store",
    redirect: "error",
    ...options,
    headers: {
      ...options.headers,
      ...(options.body ? { "Content-Type": "application/json" } : {}),
    },
  });
  if (!response.ok) throw new Error((await response.text()) || `HTTP ${response.status}`);
  if (response.status === 204) return null;
  return response.json();
}

async function run(task) {
  try {
    await task();
  } catch (error) {
    notice.textContent = `操作に失敗しました: ${error.message}`;
  }
}

function textElement(name, className, value) {
  const element = document.createElement(name);
  if (className) element.className = className;
  element.textContent = value;
  return element;
}

function cell(row, value, className = "", label = "") {
  const item = textElement("td", className, value);
  if (label) item.dataset.label = label;
  row.append(item);
  return item;
}

function selectedMaterial() {
  return materials[Number(materialSelect.value)];
}

function materialName(material) {
  if (!material) return "教材情報なし";
  const evidence = material.cves?.join("、") || material.lessonId.split("/").at(-1);
  return `${material.product} / ${evidence}`;
}

function formatDate(value) {
  const date = new Date(value);
  return Number.isFinite(date.getTime())
    ? date.toLocaleString("ja-JP", { dateStyle: "medium", timeStyle: "short" })
    : "日時不明";
}

function localDateTimeValue(date) {
  return new Date(date.getTime() - date.getTimezoneOffset() * 60_000).toISOString().slice(0, 16);
}

function renderMaterials() {
  const list = document.querySelector("#materials");
  list.replaceChildren();
  materialSelect.replaceChildren();
  if (materials.length === 0) {
    list.append(textElement("p", "empty-state", "割り当て可能な教材がありません。"));
    materialSelect.disabled = true;
    assignmentForm.querySelector("button[type='submit']").disabled = true;
    updateCompletionRule();
    return;
  }
  materialSelect.disabled = false;
  assignmentForm.querySelector("button[type='submit']").disabled = false;
  for (const [index, material] of materials.entries()) {
    const article = document.createElement("article");
    article.className = "material-item";
    article.append(textElement("h3", "", materialName(material)));
    article.append(textElement("p", "", material.title));
    article.append(
      textElement(
        "p",
        "",
        `${material.lessonId} @ ${material.lessonVersion} / ${material.durationMinutes}分`,
      ),
    );
    list.append(article);
    const option = document.createElement("option");
    option.value = String(index);
    option.textContent = `${materialName(material)} / ${material.lessonVersion}`;
    materialSelect.append(option);
  }
  updateCompletionRule();
}

function updateCompletionRule() {
  document.querySelector("#completion-rule").textContent =
    completionDescription(selectedMaterial());
}

function renderSummary() {
  const summary = assignmentSummary(assignments);
  const metrics = {
    "metric-total": summary.total,
    "metric-assigned": summary.counts.assigned,
    "metric-in-progress": summary.counts.in_progress,
    "metric-submitted": summary.counts.submitted,
    "metric-completed": summary.counts.completed,
    "metric-needs-retry": summary.counts.needs_retry,
  };
  for (const [id, value] of Object.entries(metrics))
    document.querySelector(`#${id}`).textContent = value;
  document.querySelector("#completion-rate").textContent = `完了率 ${summary.completionRate}%`;
  document.querySelector("#completion-meter-value").style.width = `${summary.completionRate}%`;
  document.querySelector("#overdue-count").textContent = `期限超過 ${summary.overdue}件`;
  document.querySelector("#summary-description").textContent = summary.total
    ? `${summary.total}件の割り当てを状態別に集計しています。`
    : "割り当てを作成すると、ここに進捗が表示されます。";
}

function statusBadge(status) {
  const meta = STATUS_META[status] ?? { label: status, tone: "neutral" };
  return textElement("span", `status-badge status-${meta.tone}`, meta.label);
}

function renderAssignments() {
  const body = document.querySelector("#assignments");
  body.replaceChildren();
  const visible = filterAssignments(
    assignments,
    statusFilter.value,
    assignmentSearch.value,
    materials,
  ).sort((left, right) => Date.parse(left.dueAt) - Date.parse(right.dueAt));
  document.querySelector("#assignment-count").textContent =
    `${visible.length}件 / 全${assignments.length}件`;
  if (visible.length === 0) {
    const row = document.createElement("tr");
    row.className = "empty-row";
    const item = cell(row, "条件に一致する割り当てはありません。");
    item.colSpan = 6;
    body.append(row);
    return;
  }
  for (const assignment of visible) {
    const row = document.createElement("tr");
    if (assignment.id === selectedAssignmentID) row.setAttribute("aria-current", "true");
    cell(row, assignment.learnerId, "", "対象者");
    const material = materialForAssignment(assignment, materials);
    const materialCell = cell(row, "", "", "教材");
    materialCell.append(
      textElement("strong", "", materialName(material)),
      textElement(
        "small",
        "table-detail",
        material ? `${material.lessonId} @ ${material.lessonVersion}` : "教材情報を取得できません",
      ),
    );
    const statusCell = cell(row, "", "", "状態");
    statusCell.append(statusBadge(assignment.status));
    const overdue = assignment.status !== "completed" && new Date(assignment.dueAt) < new Date();
    cell(row, formatDate(assignment.dueAt), overdue ? "deadline-overdue" : "", "受講期限");
    cell(row, material ? "提出回答を承認" : "条件不明", "", "完了条件");
    const action = cell(row, "", "table-action", "操作");
    const button = document.createElement("button");
    button.type = "button";
    button.textContent = "提出を見る";
    button.setAttribute("aria-controls", "submission-panel");
    button.addEventListener("click", () => void run(() => showSubmissions(assignment.id)));
    action.append(button);
    body.append(row);
  }
}

async function refreshAssignments() {
  assignments = await call("/api/training/assignments");
  renderSummary();
  renderAssignments();
}

function renderAnswer(questionID, answer) {
  const meta = answerPresentation(questionID);
  const wrapper = document.createElement("div");
  wrapper.className = "submission-answer";
  const term = document.createElement("dt");
  term.append(textElement("span", "", meta.label));
  const facets = document.createElement("span");
  facets.className = "answer-facets";
  for (const facet of meta.facets) facets.append(textElement("span", "facet-tag", facet));
  term.append(facets);
  const description = textElement("dd", "", answer);
  wrapper.append(term, description);
  return wrapper;
}

async function showSubmissions(assignmentID) {
  selectedAssignmentID = assignmentID;
  renderAssignments();
  const assignment = assignments.find((item) => item.id === assignmentID);
  const material = assignment ? materialForAssignment(assignment, materials) : undefined;
  const values = await call(`/api/training/assignments/${assignmentID}/submissions`);
  const root = document.querySelector("#submissions");
  const context = document.querySelector("#submission-context");
  root.replaceChildren();
  context.textContent = assignment ? `${assignment.learnerId} / ${materialName(material)}` : "";
  document.querySelector("#submission-help").textContent = values.length
    ? `${values.length}件の提出を新しい順に表示しています。`
    : "この割り当ての提出はまだありません。";
  if (values.length === 0) {
    root.append(textElement("p", "empty-state", "受講者が回答を提出すると、ここで確認できます。"));
  }
  for (const submission of values.sort(
    (left, right) => Date.parse(right.submittedAt) - Date.parse(left.submittedAt),
  )) {
    const section = document.createElement("article");
    section.className = "submission";
    const header = document.createElement("div");
    header.className = "submission-header";
    const title = document.createElement("div");
    title.append(
      textElement("h3", "", `${submission.lessonId} @ ${submission.lessonVersion}`),
      textElement("p", "muted", `提出 ${formatDate(submission.submittedAt)}`),
    );
    const evaluation =
      submission.evaluation === "accepted"
        ? { label: "承認済み", tone: "success" }
        : submission.evaluation === "needs_retry"
          ? { label: "再受講", tone: "danger" }
          : { label: "確認待ち", tone: "warning" };
    header.append(
      title,
      textElement("span", `status-badge status-${evaluation.tone}`, evaluation.label),
    );
    section.append(header);
    const details = document.createElement("dl");
    for (const [question, answer] of Object.entries(submission.answers))
      details.append(renderAnswer(question, answer));
    section.append(details);
    if (submission.evaluation === "pending") {
      const actions = document.createElement("div");
      actions.className = "actions review-actions";
      const choices = [["accepted", "回答を承認"]];
      if (assignment?.allowRetry) choices.push(["needs_retry", "再受講を依頼"]);
      for (const [value, label] of choices) {
        const button = document.createElement("button");
        button.type = "button";
        button.textContent = label;
        if (value === "accepted") button.className = "primary";
        button.addEventListener(
          "click",
          () =>
            void run(async () => {
              await call(`/api/training/submissions/${submission.id}/review`, {
                method: "POST",
                body: JSON.stringify({ evaluation: value }),
              });
              notice.textContent = `${label}として記録しました。`;
              await refreshAssignments();
              await showSubmissions(assignmentID);
            }),
        );
        actions.append(button);
      }
      section.append(actions);
    }
    root.append(section);
  }
  document.querySelector("#submission-panel").scrollIntoView({
    behavior: "smooth",
    block: "start",
  });
}

async function load() {
  const [materialRecords, assignmentRecords] = await Promise.all([
    call("/api/training/materials"),
    call("/api/training/assignments"),
  ]);
  materials = materialRecords;
  assignments = assignmentRecords;
  renderMaterials();
  renderSummary();
  renderAssignments();
}

document.querySelector("#login-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  try {
    await call("/api/admin/login", {
      method: "POST",
      body: JSON.stringify({
        username: document.querySelector("#username").value,
        password: document.querySelector("#password").value,
      }),
    });
    loginPanel.hidden = true;
    dashboard.hidden = false;
    notice.textContent = "";
    await load();
  } catch {
    notice.textContent = "ログインできませんでした。認証情報を確認してください。";
  }
});

assignmentForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  await run(async () => {
    const material = selectedMaterial();
    if (!material) throw new Error("教材を選択してください。");
    const learnerIDs = parseLearnerIDs(document.querySelector("#learners").value);
    const dueAt = new Date(dueInput.value);
    if (!Number.isFinite(dueAt.getTime()) || dueAt <= new Date())
      throw new Error("受講期限は現在より後に設定してください。");
    const created = await call("/api/training/assignments", {
      method: "POST",
      body: JSON.stringify({
        lessonId: material.lessonId,
        lessonVersion: material.lessonVersion,
        learnerIds: learnerIDs,
        cohortId: "",
        dueAt: dueAt.toISOString(),
        allowRetry: document.querySelector("#retry").checked,
      }),
    });
    document.querySelector("#learners").value = "";
    clearTimeout(assignmentToastTimer);
    assignmentToast.textContent = `${created.length}名に「${materialName(material)}」を割り当てました。`;
    assignmentToast.classList.add("is-visible");
    assignmentToastTimer = setTimeout(() => {
      assignmentToast.classList.remove("is-visible");
      assignmentToast.textContent = "";
    }, 5000);
    await refreshAssignments();
  });
});

materialSelect.addEventListener("change", updateCompletionRule);
statusFilter.addEventListener("change", renderAssignments);
assignmentSearch.addEventListener("input", renderAssignments);

document.querySelector("#logout").addEventListener("click", () => {
  void run(async () => {
    await call("/api/admin/logout", { method: "POST" });
    location.reload();
  });
});

document.querySelector("#download").addEventListener("click", () => {
  void run(async () => {
    const response = await fetch("/api/training/exports/results.csv", {
      credentials: "same-origin",
    });
    if (!response.ok) throw new Error("CSVを取得できませんでした。");
    const link = document.createElement("a");
    link.href = URL.createObjectURL(await response.blob());
    link.download = "reprokit-training-results.csv";
    link.click();
    URL.revokeObjectURL(link.href);
  });
});

const defaultDue = new Date();
defaultDue.setDate(defaultDue.getDate() + 14);
defaultDue.setHours(18, 0, 0, 0);
dueInput.min = localDateTimeValue(new Date());
dueInput.value = localDateTimeValue(defaultDue);

const adminConfig = await call("/api/admin/config").catch(() => ({ oidcEnabled: false }));
document.querySelector("#sso-login").hidden = !adminConfig.oidcEnabled;
if ((await fetch("/api/admin/session", { credentials: "same-origin" })).ok) {
  loginPanel.hidden = true;
  dashboard.hidden = false;
  await run(load);
}
