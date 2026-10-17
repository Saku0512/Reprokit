const loginPanel = document.querySelector("#login-panel");
const dashboard = document.querySelector("#dashboard");
const notice = document.querySelector("#admin-notice");
let courses = [];

async function run(task) {
  try {
    await task();
  } catch (error) {
    notice.textContent = `操作に失敗しました: ${error.message}`;
  }
}

async function call(path, options = {}) {
  const response = await fetch(path, {
    credentials: "same-origin",
    cache: "no-store",
    redirect: "error",
    ...options,
    headers: options.body ? { "Content-Type": "application/json" } : undefined,
  });
  if (!response.ok) throw new Error((await response.text()) || `HTTP ${response.status}`);
  if (response.status === 204) return null;
  return response.json();
}

function cell(row, value) {
  const item = document.createElement("td");
  item.textContent = value;
  row.append(item);
  return item;
}

async function load() {
  [courses] = await Promise.all([call("/api/training/courses"), refreshAssignments()]);
  const list = document.querySelector("#courses");
  const select = document.querySelector("#course");
  list.replaceChildren();
  select.replaceChildren();
  for (const [index, course] of courses.entries()) {
    const paragraph = document.createElement("p");
    paragraph.textContent = `${course.title}（${course.id} @ ${course.version}）`;
    list.append(paragraph);
    const option = document.createElement("option");
    option.value = String(index);
    option.textContent = `${course.title} / ${course.version}`;
    select.append(option);
  }
}

async function refreshAssignments() {
  const assignments = await call("/api/training/assignments");
  const body = document.querySelector("#assignments");
  body.replaceChildren();
  for (const assignment of assignments) {
    const row = document.createElement("tr");
    cell(row, assignment.learnerId);
    cell(row, `${assignment.courseId} @ ${assignment.courseVersion}`);
    cell(row, assignment.status);
    cell(row, new Date(assignment.dueAt).toLocaleString("ja-JP"));
    const action = cell(row, "");
    const button = document.createElement("button");
    button.textContent = "提出を見る";
    button.addEventListener("click", () => void run(() => showSubmissions(assignment.id)));
    action.append(button);
    body.append(row);
  }
}

async function showSubmissions(assignmentId) {
  const values = await call(`/api/training/assignments/${assignmentId}/submissions`);
  const root = document.querySelector("#submissions");
  root.replaceChildren();
  document.querySelector("#submission-help").textContent = values.length
    ? `${values.length}件の提出があります。`
    : "提出はまだありません。";
  for (const submission of values) {
    const section = document.createElement("section");
    section.className = "submission";
    const heading = document.createElement("h3");
    heading.textContent = `${submission.learnerId} / ${submission.lessonId}`;
    section.append(heading);
    const details = document.createElement("dl");
    for (const [question, answer] of Object.entries(submission.answers)) {
      const term = document.createElement("dt");
      term.textContent = question;
      const description = document.createElement("dd");
      description.textContent = answer;
      details.append(term, description);
    }
    section.append(details);
    if (submission.evaluation === "pending") {
      const actions = document.createElement("div");
      actions.className = "actions";
      for (const [value, label] of [
        ["accepted", "承認"],
        ["needs_retry", "再提出"],
      ]) {
        const button = document.createElement("button");
        button.textContent = label;
        button.addEventListener(
          "click",
          () =>
            void run(async () => {
              await call(`/api/training/submissions/${submission.id}/review`, {
                method: "POST",
                body: JSON.stringify({ evaluation: value }),
              });
              notice.textContent = `${label}として記録しました。`;
              await Promise.all([showSubmissions(assignmentId), refreshAssignments()]);
            }),
        );
        actions.append(button);
      }
      section.append(actions);
    }
    root.append(section);
  }
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
    await load();
  } catch {
    notice.textContent = "ログインできませんでした。認証情報を確認してください。";
  }
});

document.querySelector("#assignment-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  await run(async () => {
    const course = courses[Number(document.querySelector("#course").value)];
    if (!course) throw new Error("コースを選択してください。");
    await call("/api/training/assignments", {
      method: "POST",
      body: JSON.stringify({
        courseId: course.id,
        courseVersion: course.version,
        learnerIds: [document.querySelector("#learner").value],
        cohortId: "",
        dueAt: new Date(document.querySelector("#due").value).toISOString(),
        allowRetry: document.querySelector("#retry").checked,
      }),
    });
    notice.textContent = "割り当てを作成しました。";
    await refreshAssignments();
  });
});

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

if ((await fetch("/api/admin/session", { credentials: "same-origin" })).ok) {
  loginPanel.hidden = true;
  dashboard.hidden = false;
  await run(load);
}
