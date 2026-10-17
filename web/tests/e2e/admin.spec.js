import { test, expect } from "@playwright/test";
import { join } from "node:path";
import { testOutputDir } from "./paths.js";

async function mockAdministration(page) {
  const course = {
    id: "secure-development-foundations",
    version: "1.0.0",
    title: "実在事例から学ぶセキュア開発",
    lessons: [{ lessonId: "command-injection/filebrowser-auth-hook", lessonVersion: "1.0.0" }],
    requiredLessons: [
      { lessonId: "command-injection/filebrowser-auth-hook", lessonVersion: "1.0.0" },
    ],
  };
  const base = {
    courseId: course.id,
    courseVersion: course.version,
    allowRetry: true,
    createdAt: "2026-10-01T00:00:00Z",
  };
  const assignments = [
    { ...base, id: "a1", learnerId: "new-user", status: "assigned", dueAt: "2027-01-20T09:00:00Z" },
    {
      ...base,
      id: "a2",
      learnerId: "active-user",
      status: "in_progress",
      dueAt: "2027-01-18T09:00:00Z",
    },
    {
      ...base,
      id: "a3",
      learnerId: "review-user",
      status: "submitted",
      dueAt: "2027-01-15T09:00:00Z",
    },
    {
      ...base,
      id: "a4",
      learnerId: "done-user",
      status: "completed",
      dueAt: "2027-01-10T09:00:00Z",
    },
    {
      ...base,
      id: "a5",
      learnerId: "retry-user",
      status: "needs_retry",
      dueAt: "2027-01-12T09:00:00Z",
    },
  ];
  const submissions = {
    a3: [
      {
        id: "submission-1",
        assignmentId: "a3",
        learnerId: "review-user",
        lessonId: "command-injection/filebrowser-auth-hook",
        lessonVersion: "1.0.0",
        submittedAt: "2026-10-17T08:00:00Z",
        evaluation: "pending",
        answers: {
          "trace-boundary": "認証前の入力がシェルの解釈対象になり、任意処理へ到達した。",
          "compare-behavior": "脆弱版だけでマーカーが作成され、認証失敗前の実行が確認できた。",
          "propose-prevention": "入力をコマンドへ展開せず、回帰テストとレビュー観点を追加する。",
        },
        operationLogs: ["この操作ログは表示してはいけない"],
      },
    ],
  };
  const calls = [];
  await page.route("**/api/**", async (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    const body = request.postData() ? request.postDataJSON() : null;
    calls.push({ path, method: request.method(), body });
    const json = (value, status = 200) =>
      route.fulfill({ status, contentType: "application/json", body: JSON.stringify(value) });
    if (path === "/api/admin/config") return json({ oidcEnabled: false });
    if (path === "/api/admin/session") return route.fulfill({ status: 204, body: "" });
    if (path === "/api/training/courses") return json([course]);
    if (path === "/api/training/assignments" && request.method() === "GET")
      return json(assignments);
    if (path === "/api/training/assignments" && request.method() === "POST") {
      const created = body.learnerIds.map((learnerId, index) => ({
        ...base,
        id: `new-${index}`,
        learnerId,
        dueAt: body.dueAt,
        allowRetry: body.allowRetry,
        status: "assigned",
      }));
      assignments.push(...created);
      return json(created, 201);
    }
    const submissionMatch = path.match(/^\/api\/training\/assignments\/([^/]+)\/submissions$/);
    if (submissionMatch) return json(submissions[submissionMatch[1]] ?? []);
    if (path === "/api/training/submissions/submission-1/review") {
      submissions.a3[0].evaluation = body.evaluation;
      assignments.find((assignment) => assignment.id === "a3").status =
        body.evaluation === "accepted" ? "completed" : "needs_retry";
      return json({ submission: submissions.a3[0] });
    }
    return json({ error: "unknown mock operation" }, 404);
  });
  return { assignments, calls };
}

test("複数対象者へ割り当て、状態を絞り込み、4観点の回答を確認する", async ({ page }) => {
  const fixture = await mockAdministration(page);
  await page.goto("/admin");
  await expect(page.getByRole("heading", { name: "研修の割り当てと進捗" })).toBeVisible();
  await expect(page.locator("#metric-total")).toHaveText("5");
  await expect(page.locator("#metric-assigned")).toHaveText("1");
  await expect(page.locator("#metric-in-progress")).toHaveText("1");
  await expect(page.locator("#metric-submitted")).toHaveText("1");
  await expect(page.locator("#metric-completed")).toHaveText("1");
  await expect(page.locator("#metric-needs-retry")).toHaveText("1");
  await expect(page.locator("#completion-rule")).toContainText("必須教材1件の回答がすべて承認");

  await page.getByLabel("対象者ID").fill("alice, bob\nteam-member-03");
  await page.getByLabel("受講期限").fill("2027-02-01T18:00");
  await page.getByRole("button", { name: "割り当てる" }).click();
  await expect(page.locator("#admin-notice")).toContainText("3名へ");
  const createCall = fixture.calls.find(
    (call) => call.path === "/api/training/assignments" && call.method === "POST",
  );
  expect(createCall.body.learnerIds).toEqual(["alice", "bob", "team-member-03"]);
  expect(createCall.body.allowRetry).toBe(true);
  expect(createCall.body).not.toHaveProperty("organizationId");

  await page.getByLabel("状態").selectOption("needs_retry");
  await expect(page.locator("#assignments tr")).toHaveCount(1);
  await expect(page.locator("#assignments")).toContainText("retry-user");
  await page.getByLabel("状態").selectOption("");
  await page.getByLabel("受講者・コース").fill("review-user");
  const reviewRow = page.locator("#assignments tr").filter({ hasText: "review-user" });
  await reviewRow.getByRole("button", { name: "提出を見る" }).click();
  for (const facet of ["原因", "影響", "修正", "再発防止"])
    await expect(page.getByText(facet, { exact: true }).first()).toBeVisible();
  await expect(page.getByText("認証前の入力がシェルの解釈対象")).toBeVisible();
  await expect(page.getByText("この操作ログは表示してはいけない")).toHaveCount(0);

  await page.getByRole("button", { name: "回答を承認" }).click();
  await expect(page.locator("#metric-completed")).toHaveText("2");
  await expect(page.locator("#metric-submitted")).toHaveText("0");
  await expect(page.getByText("承認済み", { exact: true })).toBeVisible();
  await page.screenshot({ path: join(testOutputDir, "admin-desktop.png"), fullPage: true });
});

test("管理画面を小画面でも横にはみ出さず操作できる", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await mockAdministration(page);
  await page.goto("/admin");
  await expect(page.getByRole("heading", { name: "研修の割り当てと進捗" })).toBeVisible();
  expect(
    await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth),
  ).toBeTruthy();
  const firstAssignment = page.locator("#assignments tr").first();
  await expect(firstAssignment.locator("td").first()).toHaveAttribute("data-label", "対象者");
  await expect(firstAssignment.getByRole("button", { name: "提出を見る" })).toBeVisible();
  await page.screenshot({ path: join(testOutputDir, "admin-mobile.png"), fullPage: true });
});
