import { test, expect } from "@playwright/test";
import { readFile } from "node:fs/promises";
import { mockRunner } from "./fixture.js";

const sample = await readFile(new URL("../../../docs/ui/ui_sample.html", import.meta.url), "utf8");

async function ready(page) {
  await page.evaluate(() => document.fonts.ready);
}

async function startLesson(page) {
  await page.getByRole("button", { name: "学習を開始する ↓", exact: true }).click();
  await expect(page.locator("#phase-title-0")).toBeFocused();
}

async function atPhase(page, phase) {
  await expect
    .poll(() =>
      page.locator(`#phase-title-${phase}`).evaluate((element) => {
        const top = element.getBoundingClientRect().top;
        const headerBottom = document.querySelector("header").getBoundingClientRect().bottom;
        const detail = element.closest(".detail");
        const margin = parseFloat(getComputedStyle(element).scrollMarginTop) || 0;
        const target = Math.min(
          detail.scrollHeight - detail.clientHeight,
          Math.max(0, detail.scrollTop + top - detail.getBoundingClientRect().top - margin),
        );
        return top >= headerBottom && Math.abs(detail.scrollTop - target) <= 2;
      }),
    )
    .toBe(true);
}

async function atNextBoundary(page, previous) {
  await expect
    .poll(() =>
      page.locator(`#phase-${previous}`).evaluate((element) => {
        const detail = element.closest(".detail");
        const gap = element.getBoundingClientRect().bottom - detail.getBoundingClientRect().top;
        // 次章が短い場合はスクロール可能範囲の末尾まで移動する。
        const atEnd = detail.scrollTop + detail.clientHeight >= detail.scrollHeight - 2;
        return Math.abs(gap) <= 2 || (atEnd && gap >= 0);
      }),
    )
    .toBe(true);
}

for (const width of [1440, 390]) {
  test(`教材一覧は元HTMLとReactの表示が一致する (${width}px)`, async ({
    page,
    context,
  }, testInfo) => {
    await page.setViewportSize({ width, height: 1000 });
    const reference = await context.newPage();
    await reference.setViewportSize({ width, height: 1000 });
    // 元HTMLの外部フォント参照だけを同梱フォントへ置換し、同一の描画条件で比較する。
    await reference.route("**/reference", (route) =>
      route.fulfill({
        contentType: "text/html",
        body: sample.replace(
          /<link[\s\S]*?\/>/,
          '<link rel="stylesheet" href="/generated/app.css" />',
        ),
      }),
    );
    await page.goto("/");
    await reference.goto("/reference");
    await expect(page.locator('img[alt="Reprokit"]')).toBeVisible();
    const compare = async (name) => {
      await ready(page);
      await ready(reference);
      const actual = await page.screenshot({ animations: "disabled" });
      const expected = await reference.screenshot({ animations: "disabled" });
      await testInfo.attach(`${name}-react`, { body: actual, contentType: "image/png" });
      await testInfo.attach(`${name}-html`, { body: expected, contentType: "image/png" });
      expect(actual.equals(expected), `${name} screenshot equality`).toBe(true);
    };
    await compare("list");
    for (const target of [page, reference]) await target.locator("#theme").click();
    await compare("dark");
  });
}

test("教材の操作、Runnerでの起動・停止・削除、完了表示", async ({ page }) => {
  const errors = [];
  page.on("pageerror", (error) => errors.push(error.message));
  const fixture = await mockRunner(page, { paired: true });
  await page.goto("/");
  await page.getByRole("button", { name: "パストラバーサル 1", exact: true }).click();
  await expect(page.locator(".lcard")).toHaveCount(1);
  await expect(page.locator(".lcard")).toBeDisabled();
  await page.getByRole("button", { name: "すべて 2", exact: true }).click();
  await page.locator('[data-a="open"]').first().click();
  await startLesson(page);
  await expect(page.locator("#phase-0 .note")).toContainText("有効な認証情報を用意していません");
  await expect(page.locator("#phase-0 .log")).toContainText("403 Forbidden");
  await page.locator('[data-a="deploy"]').click();
  await expect(page.getByRole("button", { name: "起動する", exact: true })).toBeDisabled();
  await page.getByRole("checkbox").check();
  await page.getByRole("button", { name: "起動する", exact: true }).click();
  await expect(page.locator(".st")).toHaveText("RUNNING");
  await expect(page.locator('a[href="http://127.0.0.1:18080"]')).toBeVisible();
  await expect(page.locator('a[href="http://127.0.0.1:18081"]')).toBeVisible();
  await page.getByRole("button", { name: "検証コンソール", exact: true }).click();
  const consoleDialog = page.getByRole("dialog", { name: "検証コンソール" });
  await expect(consoleDialog).toBeVisible();
  await consoleDialog.getByRole("button", { name: "ログを更新", exact: true }).click();
  await expect(consoleDialog.locator(".console-service .log")).toContainText("vulnerable log");
  await consoleDialog.getByRole("button", { name: "実行", exact: true }).click();
  await expect(consoleDialog.locator(".console-service .log").last()).toContainText("success");
  await consoleDialog.getByRole("tab", { name: "修正版コンテナ" }).click();
  await consoleDialog.getByRole("button", { name: "ログを更新", exact: true }).click();
  await expect(consoleDialog.locator(".console-service .log")).toContainText("fixed log");
  await consoleDialog.getByRole("button", { name: "検証コンソールを閉じる" }).click();
  await expect(consoleDialog).toBeHidden();
  await page.locator('#phase-0 [data-a="next"]').click();
  await atNextBoundary(page, 0);
  for (let i = 0; i < 3; i++) await page.getByRole("button", { name: "ヒントを見る" }).click();
  await expect(page.getByRole("button", { name: "ヒントを見る" })).toBeDisabled();
  await expect(page.locator("#phase-1 .step + .log")).toContainText(
    "Username: id > /tmp/success; echo hook.action=block",
  );
  await expect(page.locator("#phase-1 .step + .log")).toContainText("Password: anything");
  await page.getByRole("button", { name: "観察結果を取得" }).click();
  const observation = page.locator("#phase-1 .card").filter({
    has: page.getByRole("heading", { name: "観察結果（隔離コンテナ内）" }),
  });
  await expect(observation.getByRole("tab", { name: "修正版コンテナ" })).toHaveAttribute(
    "aria-selected",
    "true",
  );
  await expect(observation.locator(".console-service .log").first()).toContainText("fixed log");
  await observation.getByRole("tab", { name: "脆弱コンテナ" }).click();
  await page.getByRole("button", { name: "観察結果を取得" }).click();
  await expect(observation.locator(".console-service .log").first()).toContainText(
    "vulnerable log",
  );
  await expect(observation.locator(".console-service .log").last()).toContainText("success");
  await expect(page.locator("#phase-1 .data")).toContainText("dummy-report.txt");
  await page.locator('#phase-1 [data-a="next"]').click();
  await atNextBoundary(page, 1);
  await page.getByRole("button", { name: "1つ戻る" }).click();
  await expect(page.locator(".nd.at")).toContainText("外部コマンドを実行する");
  await page.locator('#phase-2 [data-a="next"]').click();
  await atNextBoundary(page, 2);
  await page.locator('.diff [data-v="a"]').first().click();
  await expect(page.locator(".diff .hl")).toHaveCount(2);
  await page.locator('#phase-3 [data-a="next"]').click();
  await atNextBoundary(page, 3);
  for (const action of ["runV", "runF"]) await page.locator(`[data-a="${action}"]`).click();
  await expect(page.locator("#phase-4 .reveal")).toContainText("確認できました。");
  await page.locator('#phase-4 [data-a="next"]').click();
  await page.locator('[data-a="stop"]').click();
  await expect(page.locator(".st")).toHaveText("STOPPED");
  await page.locator('[data-a="destroy"]').click();
  await page.locator("#vol").selectOption("rm");
  await page.getByRole("button", { name: "削除する", exact: true }).click();
  await expect(page.locator(".st")).toHaveText("READY");
  await page.locator('[data-a="home"]').first().click();
  await expect(page.locator(".lcard").first()).toContainText("Completed");
  expect(
    fixture.calls.filter((call) => call.path === "/api/labs" && call.method === "POST"),
  ).toEqual([
    expect.objectContaining({
      body: {
        lessonId: "command-injection/filebrowser-auth-hook",
        lessonVersion: "1.0.0",
        labRef: "vulnerable",
      },
    }),
  ]);
  expect(
    fixture.calls.filter(
      (call) => call.path.endsWith("/start") && call.method === "POST" && call.body.confirmed,
    ),
  ).toHaveLength(1);
  expect(fixture.calls.filter((call) => call.path.endsWith("/stop"))).toHaveLength(1);
  expect(fixture.calls.filter((call) => call.method === "DELETE")[0].query).toBe(
    "?volumePolicy=delete",
  );
  expect(errors).toEqual([]);
});

test("Deployの進捗バーは目盛りごとに増える", async ({ page }) => {
  await mockRunner(page, { delayPrepare: 2000 });
  await page.goto("/");
  await page.locator('[data-a="open"]').first().click();
  await startLesson(page);
  await page.locator('[data-a="deploy"]').click();
  await page.getByRole("checkbox").check();
  await page.getByRole("button", { name: "起動する", exact: true }).click();

  const progress = page.locator(".seg i.on");
  await expect.poll(async () => (await progress.count()) > 0).toBe(true);
  expect(await progress.count()).toBeLessThan(10);
  await expect(page.locator(".st")).toHaveText("RUNNING");
});

for (const width of [1440, 390]) {
  test(`縦ナビと次へで章を追加し、スクロールで読み返せる (${width}px)`, async ({
    page,
  }, testInfo) => {
    await page.setViewportSize({ width, height: 900 });
    await page.goto("/");
    await ready(page);
    await page.locator('[data-a="open"]').first().click();
    const compact = width <= 1000;
    const toggle = page.getByRole("button", { name: "学習メニューを開く" });
    const outline = page.locator(compact ? "#mobile-lesson-outline" : "#lesson-outline");
    if (!compact) {
      const nav = await outline.boundingBox();
      const title = await page
        .getByRole("heading", { name: "File Browser", exact: true })
        .boundingBox();
      const back = await page
        .getByRole("button", { name: "← 教材一覧へ戻る", exact: true })
        .boundingBox();
      expect(Math.abs(nav.y - title.y)).toBeLessThanOrEqual(1);
      expect(back.y + back.height).toBeLessThan(nav.y);
      expect(Math.abs(back.x - nav.x)).toBeLessThanOrEqual(1);
    }
    if (compact) await toggle.click();
    const steps = outline.locator("button[data-a='go']");
    const positions = await steps.evaluateAll((elements) =>
      elements.map((element) => {
        const bounds = element.getBoundingClientRect();
        return { x: bounds.x, y: bounds.y };
      }),
    );
    expect(
      positions.every(
        (position, i) => position.x === positions[0].x && (!i || position.y > positions[i - 1].y),
      ),
    ).toBeTruthy();
    if (compact) {
      await page.getByRole("button", { name: "学習メニューを閉じる" }).click();
      await expect(page.getByRole("dialog", { name: "学習メニュー" })).toBeHidden();
    }
    await page.screenshot({
      path: testInfo.outputPath("lesson-start.png"),
      fullPage: width < 1000,
      animations: "disabled",
    });
    await expect(page.locator(".lesson-phase")).toHaveCount(0);
    await expect(steps.nth(0)).toBeDisabled();
    await expect(outline).toContainText("0 / 5 ステップ完了");
    await page.getByRole("button", { name: "学習を開始する ↓", exact: true }).focus();
    await page.keyboard.press("Enter");
    await expect(page.locator("#phase-title-0")).toBeFocused();
    await atPhase(page, 0);
    await expect(steps.nth(0)).toHaveAttribute("aria-current", "step");
    await expect(outline).toContainText("0 / 5 ステップ完了");
    await expect(steps.nth(1)).toBeDisabled();
    await expect(page.locator(".lesson-phase")).toHaveCount(1);
    await page.locator('#phase-0 [data-a="next"]').focus();
    await page.keyboard.press("Enter");
    await expect(page.locator(".lesson-phase")).toHaveCount(2);
    await expect(page.locator("#phase-title-1")).toBeFocused();
    await expect(steps.nth(1)).toHaveAttribute("aria-current", "step");
    await expect(page.locator("#phase-0 .log")).toContainText("403 Forbidden");
    await atNextBoundary(page, 0);
    await page.locator('#phase-1 [data-a="next"]').click();
    await expect(page.locator(".lesson-phase")).toHaveCount(3);
    await atNextBoundary(page, 1);
    await expect(steps.nth(2)).toHaveAttribute("aria-current", "step");
    if (compact) await toggle.click();
    await steps.nth(0).click();
    if (compact) await expect(page.getByRole("dialog", { name: "学習メニュー" })).toBeHidden();
    await expect(page.locator("#phase-title-0")).toBeFocused();
    await atPhase(page, 0);
    await expect(steps.nth(0)).toHaveAttribute("aria-current", "step");
    // 手動スクロールは現在位置だけを更新し、未読の章を完了扱いにしない。
    await page
      .locator("#phase-title-2")
      .evaluate((element) => element.scrollIntoView({ behavior: "instant", block: "start" }));
    await expect(steps.nth(2)).toHaveAttribute("aria-current", "step");
    await expect(outline).toContainText("2 / 5 ステップ完了");
    await expect(steps.nth(3)).toBeDisabled();
    await page.locator('#phase-2 [data-a="prev"]').click();
    await expect(page.locator("#phase-title-1")).toBeFocused();
    await expect(steps.nth(1)).toHaveAttribute("aria-current", "step");
    await page.locator('#phase-0 [data-a="next"]').click();
    await expect(page.locator(".lesson-phase")).toHaveCount(3);
    await atNextBoundary(page, 0);
    await expect(steps.nth(1)).toHaveAttribute("aria-current", "step");
    expect(
      await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
    ).toBeTruthy();
    expect(
      await page
        .locator(".detail")
        .evaluate((element) => element.scrollWidth <= element.clientWidth),
    ).toBeTruthy();
    const screenshot = testInfo.outputPath("lesson-reading.png");
    await page.screenshot({ path: screenshot, animations: "disabled" });
    await testInfo.attach(`lesson-reading-${width}`, {
      path: screenshot,
      contentType: "image/png",
    });
    await page.getByRole("button", { name: "Step 1を読み返す ↓", exact: true }).click();
    await expect(page.locator("#phase-title-0")).toBeFocused();
    await expect(page.locator(".lesson-phase")).toHaveCount(3);
    await expect(outline).toContainText("2 / 5 ステップ完了");
    await expect(page.locator("#phase-0 .log")).toContainText("403 Forbidden");
  });
}

test("動きを減らす設定でも次章へ進み、表示済みの章を保持する", async ({ page }) => {
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.goto("/");
  await page.locator('[data-a="open"]').first().click();
  await startLesson(page);
  await page.locator('#phase-0 [data-a="next"]').click();
  await expect(page.locator("#phase-title-1")).toBeFocused();
  await expect(page.locator(".lesson-phase")).toHaveCount(2);
  expect(
    await page
      .locator("#phase-1 .phase-body")
      .evaluate((element) => getComputedStyle(element).animationName),
  ).toBe("none");
});

for (const width of [390, 768]) {
  test(`狭い画面ではメニューを開閉でき、ヘッダーを固定して章へ移動する (${width}px)`, async ({
    page,
  }, testInfo) => {
    await page.setViewportSize({ width, height: 844 });
    await page.emulateMedia({ reducedMotion: "reduce" });
    await page.goto("/");
    await page.locator('[data-a="open"]').first().click();
    await startLesson(page);
    const toggle = page.getByRole("button", { name: "学習メニューを開く" });
    const drawer = page.getByRole("dialog", { name: "学習メニュー" });
    await expect(toggle).toBeVisible();
    await expect(page.locator("#lesson-outline")).toBeHidden();
    await expect(drawer).toBeHidden();
    await page.locator('#phase-0 [data-a="next"]').click();
    await expect
      .poll(() =>
        page.locator("#phase-title-1").evaluate((element) => element.getBoundingClientRect().top),
      )
      .toBeGreaterThanOrEqual(64);
    expect(
      await page.locator("header").evaluate((element) => element.getBoundingClientRect().top),
    ).toBe(0);
    const position = () =>
      page.locator(".detail").evaluate((element) => ({ detail: element.scrollTop, page: scrollY }));
    const scroll = await position();
    expect(scroll.detail).toBeGreaterThan(0);
    await toggle.focus();
    await page.keyboard.press("Enter");
    await expect(toggle).toHaveAttribute("aria-expanded", "true");
    await expect(page.getByRole("button", { name: "学習メニューを閉じる" })).toBeFocused();
    await page.screenshot({ path: testInfo.outputPath("mobile-menu.png"), animations: "disabled" });
    await page.keyboard.press("Escape");
    await expect(drawer).toBeHidden();
    await expect(toggle).toHaveAttribute("aria-expanded", "false");
    await expect(toggle).toBeFocused();
    expect(await position()).toEqual(scroll);
    await toggle.click();
    await drawer.locator('[data-a="go"][data-i="0"]').click();
    await expect(drawer).toBeHidden();
    await expect(page.locator("#phase-title-0")).toBeFocused();
    expect(
      await page
        .locator("#phase-title-0")
        .evaluate((element) => element.getBoundingClientRect().top),
    ).toBeGreaterThanOrEqual(64);
    await toggle.click();
    await page.getByRole("button", { name: "学習メニューを閉じる" }).click();
    await expect(drawer).toBeHidden();
    await toggle.click();
    await page.setViewportSize({ width: 1440, height: 900 });
    await expect(drawer).toBeHidden();
    await expect(toggle).toBeHidden();
    await expect(page.locator("#lesson-outline")).toBeVisible();
    await page.setViewportSize({ width, height: 844 });
    await expect(toggle).toBeVisible();
    await expect(drawer).toBeHidden();
  });
}
