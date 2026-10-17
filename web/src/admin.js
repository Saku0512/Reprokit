import {
  STATUS_META,
  answerPresentation,
  assignmentSummary,
  completionDescription,
  filterAssignments,
  parseLearnerIDs,
} from "./admin-model.js";

const loginPanel = document.querySelector("#login-panel");
const dashboard = document.querySelector("#dashboard");
const notice = document.querySelector("#admin-notice");
const assignmentForm = document.querySelector("#assignment-form");
const courseSelect = document.querySelector("#course");
const dueInput = document.querySelector("#due");
const statusFilter = document.querySelector("#status-filter");
const assignmentSearch = document.querySelector("#assignment-search");
let courses = [];
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

function selectedCourse() {
  return courses[Number(courseSelect.value)];
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

function renderCourses() {
  const list = document.querySelector("#courses");
  list.replaceChildren();
  courseSelect.replaceChildren();
  if (courses.length === 0) {
    list.append(textElement("p", "empty-state", "割り当て可能なコースがありません。"));
    courseSelect.disabled = true;
    assignmentForm.querySelector("button[type='submit']").disabled = true;
    updateCompletionRule();
    return;
  }
  courseSelect.disabled = false;
  assignmentForm.querySelector("button[type='submit']").disabled = false;
  for (const [index, course] of courses.entries()) {
    const article = document.createElement("article");
    article.className = "course-item";
    article.append(textElement("h3", "", course.title));
    article.append(textElement("p", "", `${course.id} @ ${course.version}`));
    article.append(
      textElement(
        "p",
        "course-condition",
        `教材 ${course.lessons?.length ?? 0}件 / 必須 ${course.requiredLessons?.length ?? 0}件`,
      ),
    );
    list.append(article);
    const option = document.createElement("option");
    option.value = String(index);
    option.textContent = `${course.title} / ${course.version}`;
    courseSelect.append(option);
  }
  updateCompletionRule();
}

function updateCompletionRule() {
  document.querySelector("#completion-rule").textContent = completionDescription(selectedCourse());
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
  const visible = filterAssignments(assignments, statusFilter.value, assignmentSearch.value).sort(
    (left, right) => Date.parse(left.dueAt) - Date.parse(right.dueAt),
  );
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
    const course = courses.find(
      (item) => item.id === assignment.courseId && item.version === assignment.courseVersion,
    );
    const courseCell = cell(row, "", "", "コース");
    courseCell.append(
      textElement("strong", "", course?.title ?? assignment.courseId),
      textElement("small", "table-detail", `${assignment.courseId} @ ${assignment.courseVersion}`),
    );
    const statusCell = cell(row, "", "", "状態");
    statusCell.append(statusBadge(assignment.status));
    const overdue = assignment.status !== "completed" && new Date(assignment.dueAt) < new Date();
    cell(row, formatDate(assignment.dueAt), overdue ? "deadline-overdue" : "", "受講期限");
    cell(
      row,
      course?.requiredLessons?.length
        ? `必須教材 ${course.requiredLessons.length}件を承認`
        : "条件不明",
      "",
      "完了条件",
    );
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
  const values = await call(`/api/training/assignments/${assignmentID}/submissions`);
  const root = document.querySelector("#submissions");
  const context = document.querySelector("#submission-context");
  root.replaceChildren();
  context.textContent = assignment ? `${assignment.learnerId} / ${assignment.courseId}` : "";
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
  [courses, assignments] = await Promise.all([
    call("/api/training/courses"),
    call("/api/training/assignments"),
  ]);
  renderCourses();
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
    const course = selectedCourse();
    if (!course) throw new Error("コースを選択してください。");
    const learnerIDs = parseLearnerIDs(document.querySelector("#learners").value);
    const dueAt = new Date(dueInput.value);
    if (!Number.isFinite(dueAt.getTime()) || dueAt <= new Date())
      throw new Error("受講期限は現在より後に設定してください。");
    const created = await call("/api/training/assignments", {
      method: "POST",
      body: JSON.stringify({
        courseId: course.id,
        courseVersion: course.version,
        learnerIds: learnerIDs,
        cohortId: "",
        dueAt: dueAt.toISOString(),
        allowRetry: document.querySelector("#retry").checked,
      }),
    });
    document.querySelector("#learners").value = "";
    notice.textContent = `${created.length}名へ「${course.title}」を割り当てました。`;
    await refreshAssignments();
  });
});

courseSelect.addEventListener("change", updateCompletionRule);
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
