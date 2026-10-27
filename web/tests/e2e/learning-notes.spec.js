import { test, expect } from "@playwright/test";
import { mockRunner } from "./fixture.js";
import { PH } from "../../src/learner/lesson-data.ts";
import { emptyNote, noteStorageKey } from "../../src/learner/learning-note.ts";

const lesson = { lessonId: "command-injection/filebrowser-auth-hook", lessonVersion: "1.1.0" };
const storageKey = noteStorageKey(lesson);
const labels = ["影響", "原因", "修正", "再発防止"];
const answerField = (page, i) =>
  page.getByRole("textbox", { name: `${labels[i]}についてのまとめ`, exact: true });
const question = (page, i) => page.locator(".note-question").nth(i);
const nextQuestion = (page, i) =>
  question(page, i).getByRole("button", { name: "次の質問へ ↓" }).click();

async function atQuestion(page, i) {
  await expect
    .poll(async () =>
      question(page, i).evaluate((section) => {
        const top = section.getBoundingClientRect().top;
        const header = document.querySelector("header").getBoundingClientRect().bottom;
        const container = section.closest(".notes-page");
        const margin = parseFloat(getComputedStyle(section).scrollMarginTop) || 0;
        const target =
          getComputedStyle(container).overflowY === "auto"
            ? container.getBoundingClientRect().top + margin
            : margin;
        return top >= header + 16 && Math.abs(top - target) <= 2;
      }),
    )
    .toBe(true);
}
const answers = [
  "認証前に、入力した内容がコマンドとして実行される。",
  "資格情報をコマンド引数へ展開し、シェルが命令として解釈した。",
  "引数への文字列展開をやめ、資格情報は環境変数で渡す。",
  "外部入力がシェルの命令に混ざらないか、レビューとテストで確認する。",
];

async function openNotes(page) {
  await page.getByRole("button", { name: "まとめノート →", exact: true }).click();
  await expect(page.getByRole("heading", { name: "まとめノート", exact: true })).toBeVisible();
}

async function finishLesson(page) {
  await page.locator('[data-a="open"]').first().click();
  await page.getByRole("button", { name: "学習を開始する ↓", exact: true }).click();
  for (let phase = 0; phase < PH.length; phase++) {
    await page.locator(`#phase-${phase} [data-a="next"]`).click();
  }
  await page.getByRole("button", { name: "学習のまとめを書く →", exact: true }).click();
}

async function noHorizontalOverflow(page) {
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(
    true,
  );
}

for (const width of [1920, 1440, 768, 390]) {
  test(`ノートを分野から区切り、戻るボタンを教材ページと揃える (${width}px)`, async ({
    page,
  }, testInfo) => {
    await page.setViewportSize({ width, height: 1000 });
    await page.goto("/");
    await page.evaluate(() => document.fonts.ready);
    const navigation = page.getByRole("navigation", { name: "学習の振り返り", exact: true });
    await expect(navigation).toHaveCSS("border-top-style", "dashed");
    const divider = await navigation.boundingBox();
    const category = await page.locator('.cats [data-a="cat"]').last().boundingBox();
    expect(divider.y).toBeGreaterThan(category.y + category.height);
    await page
      .locator(".cats")
      .screenshot({ path: testInfo.outputPath(`sidebar-notes-${width}.png`) });
    await page.locator('[data-a="open"]').first().click();
    const back = page.getByRole("button", { name: "← 教材一覧へ戻る", exact: true });
    const lessonPosition = await back.boundingBox();
    await back.click();
    await openNotes(page);
    const notesPosition = await back.boundingBox();
    expect(Math.abs(notesPosition.x - lessonPosition.x)).toBeLessThanOrEqual(1);
    expect(Math.abs(notesPosition.y - lessonPosition.y)).toBeLessThanOrEqual(1);
    await noHorizontalOverflow(page);
    await page.screenshot({ path: testInfo.outputPath(`notes-navigation-${width}.png`) });
  });
}

for (const width of [1440, 390]) {
  test(`学習後のまとめ、途中再開、カードの振り返りと編集 (${width}px)`, async ({
    page,
  }, testInfo) => {
    const errors = [];
    page.on("pageerror", (error) => errors.push(error.message));
    const fixture = await mockRunner(page);
    await page.setViewportSize({ width, height: 900 });
    await page.emulateMedia({ reducedMotion: width === 1440 ? "no-preference" : "reduce" });
    await page.goto("/");
    await openNotes(page);
    await expect(page.getByRole("heading", { name: "まだまとめノートがありません" })).toBeVisible();
    await page.getByRole("button", { name: "← 教材一覧へ戻る" }).click();
    await finishLesson(page);
    await expect(
      page.getByRole("heading", { name: "どんなことが起きる脆弱性でしたか？" }),
    ).toBeFocused();
    await expect(page.getByRole("textbox")).toHaveCount(4);
    await expect(question(page, 0).getByRole("button", { name: "前の質問へ ↑" })).toBeDisabled();
    await expect(question(page, 3).getByRole("button", { name: "次の質問へ ↓" })).toHaveCount(0);
    await answerField(page, 0).fill(answers[0]);
    await nextQuestion(page, 0);
    await atQuestion(page, 1);
    await expect(question(page, 1).locator(".phase-kicker")).toBeInViewport();
    await page.screenshot({ path: testInfo.outputPath(`note-scroll-${width}.png`) });
    await expect(answerField(page, 0)).toHaveValue(answers[0]);
    const positions = await page
      .locator(".note-question")
      .evaluateAll((sections) => sections.map((section) => section.getBoundingClientRect().top));
    expect(positions.every((top, i) => i === 0 || top > positions[i - 1])).toBe(true);
    await question(page, 1).getByRole("button", { name: "前の質問へ ↑" }).click();
    await atQuestion(page, 0);
    await nextQuestion(page, 0);
    await answerField(page, 1).fill(answers[1]);
    await expect(page.getByRole("status").filter({ hasText: "下書き" })).toContainText(
      "保存しました",
    );
    await page.reload();
    await openNotes(page);
    await expect(page.getByText(/2 \/ 4問回答済み/)).toBeVisible();
    await page.getByRole("button", { name: "下書きを再開する" }).click();
    await expect(answerField(page, 1)).toHaveValue(answers[1]);
    await expect(question(page, 1).locator("h2")).toBeFocused();
    await atQuestion(page, 1);
    await nextQuestion(page, 1);
    await answerField(page, 2).fill(answers[2]);
    await nextQuestion(page, 2);
    await answerField(page, 3).fill(" \n ");
    await expect(page.getByRole("button", { name: "カードに保存する" })).toBeDisabled();
    await answerField(page, 3).fill(answers[3]);
    await noHorizontalOverflow(page);
    await page.screenshot({
      path: testInfo.outputPath(`note-editor-${width}.png`),
      fullPage: true,
    });
    await page.getByRole("button", { name: "カードに保存する" }).click();
    await expect(page.getByRole("article")).toContainText(answers[0]);
    await expect(page.getByRole("heading", { name: "まとめノート", exact: true })).toBeFocused();
    await page.reload();
    await openNotes(page);
    const card = page.getByRole("article");
    await expect(card).toContainText(answers[0]);
    await page.getByText("4つの回答を読む", { exact: true }).click();
    for (const answer of answers)
      await expect(card.locator("dd").filter({ hasText: answer })).toBeVisible();
    await noHorizontalOverflow(page);
    await page.screenshot({ path: testInfo.outputPath(`note-card-${width}.png`), fullPage: true });
    await page.getByRole("button", { name: "表示切替" }).click();
    await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
    await page.screenshot({
      path: testInfo.outputPath(`note-card-dark-${width}.png`),
      fullPage: true,
    });
    await page.getByRole("button", { name: "まとめを編集する" }).click();
    await page.getByRole("button", { name: "✓ 原因", exact: true }).click();
    await atQuestion(page, 1);
    await answerField(page, 1).fill("書き直し中の原因");
    await page.getByRole("button", { name: "← まとめノートへ戻る" }).click();
    await expect(card.locator(".note-summary")).toContainText(answers[1]);
    await expect(card).not.toContainText("書き直し中の原因");
    await page.getByRole("button", { name: "まとめを編集する" }).click();
    await expect(answerField(page, 1)).toHaveValue("書き直し中の原因");
    await answerField(page, 1).fill("見直した原因：入力を命令として扱った。");
    await page.getByRole("button", { name: "✓ 再発防止", exact: true }).click();
    await page.getByRole("button", { name: "カードに保存する" }).click();
    await expect(card.locator(".note-summary")).toContainText(
      "見直した原因：入力を命令として扱った。",
    );
    await page.getByRole("button", { name: "教材を読み返す" }).click();
    await expect(page.getByRole("heading", { name: "概要", exact: true })).toBeVisible();
    expect(fixture.calls).toEqual([]);
    expect(errors).toEqual([]);
  });
}

test("保存できない場合は回答と既存カードを保持し、再試行できる", async ({ page }) => {
  const complete = {
    ...emptyNote(lesson),
    draft: Object.fromEntries(
      ["impact", "cause", "remediation", "prevention"].map((id, i) => [id, answers[i]]),
    ),
    step: 3,
    saved: {
      answers: Object.fromEntries(
        ["impact", "cause", "remediation", "prevention"].map((id, i) => [id, answers[i]]),
      ),
      updatedAt: "2026-10-18T00:00:00Z",
    },
  };
  await page.addInitScript(
    ({ storageKey, complete }) => {
      localStorage.setItem(storageKey, JSON.stringify(complete));
    },
    { storageKey, complete },
  );
  await page.goto("/");
  await openNotes(page);
  await page.getByRole("button", { name: "まとめを編集する" }).click();
  await page.evaluate(() => {
    window.originalNoteSetItem = Storage.prototype.setItem;
    Storage.prototype.setItem = function () {
      throw new DOMException("full", "QuotaExceededError");
    };
  });
  await answerField(page, 3).fill("保存制限中の回答");
  await expect(page.getByRole("alert")).toContainText("保存できませんでした");
  await page.getByRole("button", { name: "カードに保存する" }).click();
  await expect(page.getByRole("alert")).toContainText("カードを保存できませんでした");
  await expect(answerField(page, 3)).toHaveValue("保存制限中の回答");
  expect(
    JSON.parse(await page.evaluate((key) => localStorage.getItem(key), storageKey)).saved.answers
      .prevention,
  ).toBe(answers[3]);
  await page.getByRole("button", { name: "← まとめノートへ戻る" }).click();
  await expect(page.getByRole("article")).not.toContainText("保存制限中の回答");
  await expect(page.getByRole("article").locator("dd").last()).toHaveText(answers[3]);
  await page.getByRole("button", { name: "まとめを編集する" }).click();
  await expect(answerField(page, 3)).toHaveValue("保存制限中の回答");
  await page.evaluate(() => {
    Storage.prototype.setItem = window.originalNoteSetItem;
  });
  await page.getByRole("button", { name: "下書きの保存を再試行する" }).click();
  await expect(page.getByRole("alert")).toHaveCount(0);
  await page.getByRole("button", { name: "カードに保存する" }).click();
  await expect(page.getByRole("article")).toContainText("保存制限中の回答");
});

test("壊れた保存データを読み込んでも画面は操作できる", async ({ page }) => {
  await page.addInitScript((key) => localStorage.setItem(key, "{broken"), storageKey);
  await page.goto("/");
  await openNotes(page);
  await expect(page.getByRole("alert")).toContainText("読み込めませんでした");
  await expect(page.getByRole("button", { name: "教材を読む", exact: true })).toBeVisible();
  expect(await page.evaluate((key) => localStorage.getItem(key), storageKey)).toBe("{broken");
});
