import { test, expect } from "@playwright/test";
import { readFile } from "node:fs/promises";
import { mockRunner } from "./fixture.js";
import { PH } from "../../src/learner/lesson-data.ts";

const sample = await readFile(new URL("../../../docs/ui/ui_sample.html", import.meta.url), "utf8");

async function ready(page) {
  await page.evaluate(() => document.fonts.ready);
}

async function startLesson(page) {
  await page.getByRole("button", { name: "学習を開始する ↓", exact: true }).click();
  await expect(page.locator("#phase-title-0")).toBeFocused();
}

function labEndpoints(page) {
  return page.locator(".tm .r2 > span").filter({ has: page.locator(".mono") });
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
        // 章数と削除済みのヘッダー文言を現行仕様へ合わせ、残りのレイアウトを比較する。
        body: sample
          .replace(
            '<p class="note2">',
            '<button class="cat" data-a="notes">まとめノート →</button><p class="note2">',
          )
          .replaceAll("/ 5</em>", `/ ${PH.length}</em>`)
          .replace(/<span\b[^>]*>\s*実際のCVEを、自分で試して理解する\s*<\/span\s*>/, "")
          .replace(/<link[\s\S]*?\/>/, '<link rel="stylesheet" href="/generated/app.css" />'),
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
  const endpoints = labEndpoints(page);
  await expect(endpoints).toHaveCount(2);
  await expect(endpoints.nth(0)).toContainText("脆弱版：http://127.0.0.1:18080");
  await expect(endpoints.nth(1)).toContainText("修正版：http://127.0.0.1:18081");
  await page.getByRole("button", { name: "検証コンソール", exact: true }).click();
  const consoleDialog = page.getByRole("dialog", { name: "検証コンソール" });
  await expect(consoleDialog).toBeVisible();
  await consoleDialog.getByRole("button", { name: "ログを更新", exact: true }).click();
  await expect(consoleDialog.locator(".console-service .log")).toContainText("vulnerable log");
  await expect(consoleDialog.getByText("cat /tmp/success")).toBeVisible();
  await consoleDialog.getByRole("button", { name: "確認", exact: true }).click();
  await expect(consoleDialog.locator(".console-service .log").last()).toContainText(
    "uid=1000(filebrowser)",
  );
  await expect(consoleDialog.getByRole("tab")).toHaveCount(0);
  await expect(consoleDialog.getByRole("region", { name: "脆弱コンテナ" })).toBeVisible();
  await expect(consoleDialog).not.toContainText("修正版コンテナ");
  await consoleDialog.getByRole("button", { name: "検証コンソールを閉じる" }).click();
  await expect(consoleDialog).toBeHidden();
  await page.locator('#phase-0 [data-a="next"]').click();
  await atNextBoundary(page, 0);
  await expect(page.getByRole("heading", { name: "影響を受けるデータ", exact: true })).toHaveCount(
    0,
  );
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
  await expect(observation.getByRole("tab")).toHaveCount(0);
  await expect(observation.getByRole("region", { name: "脆弱コンテナ" })).toBeVisible();
  await expect(observation).not.toContainText("修正版コンテナ");
  await expect(observation.locator(".console-service .log").first()).toContainText(
    "vulnerable log",
  );
  await expect(observation.getByText("cat /tmp/success")).toBeVisible();
  await expect(observation.locator(".console-service .log").last()).toContainText(
    "uid=1000(filebrowser)",
  );
  const consoleCalls = fixture.calls.filter(
    (call) => call.path.includes("/logs/") || call.path.includes("/markers/"),
  );
  expect(consoleCalls.map((call) => call.path.split("/").slice(4).join("/")).sort()).toEqual([
    "logs/vulnerable",
    "logs/vulnerable",
    "markers/vulnerable",
    "markers/vulnerable",
  ]);
  await page.locator('#phase-1 [data-a="next"]').click();
  await atNextBoundary(page, 1);
  const traceDetail = page.getByRole("region", { name: "この地点の様子", exact: true });
  await expect(page.locator(".nd.at")).toContainText("ログイン要求を受け取る");
  await expect(traceDetail.locator("code")).toContainText("json.NewDecoder(r.Body).Decode(&cred)");
  await expect(page.getByRole("button", { name: "1つ戻る" })).toBeDisabled();
  await expect(page.getByRole("button", { name: "1つ進める" })).toBeEnabled();
  for (const code of [
    "sh -c $USERNAME",
    "a.Cred = cred",
    "os.Expand(arg, envMapping)",
    "cmd.Output()",
    'a.Fields.Values["hook.action"]',
  ]) {
    await page.getByRole("button", { name: "1つ進める" }).click();
    await expect(traceDetail.locator("code")).toContainText(code);
  }
  await expect(page.locator(".nd.at")).toContainText("認証結果を確定する");
  await expect(page.getByRole("button", { name: "1つ進める" })).toBeDisabled();
  await expect(traceDetail).toContainText("後からログインを拒否しても取り消されません");
  await expect(traceDetail.locator("code")).toContainText('a.Fields.Values["hook.action"]');
  await page.getByRole("button", { name: "1つ戻る" }).click();
  await expect(page.locator(".nd.at")).toContainText("外部コマンドを実行する");
  await expect(traceDetail).toContainText("File Browserプロセスの権限");
  await expect(traceDetail.locator("code")).toContainText("cmd.Output()");
  for (const code of [
    "os.Expand(arg, envMapping)",
    "a.Cred = cred",
    "sh -c $USERNAME",
    "json.NewDecoder(r.Body).Decode(&cred)",
  ]) {
    await page.getByRole("button", { name: "1つ戻る" }).click();
    await expect(traceDetail.locator("code")).toContainText(code);
  }
  await expect(page.getByRole("button", { name: "1つ戻る" })).toBeDisabled();
  await expect(
    traceDetail.getByRole("link", { name: "File Browser 2.63.5の実装を見る" }),
  ).toHaveAttribute("href", /v2\.63\.5\/auth\/hook\.go#L36$/);
  await page.getByRole("button", { name: "1つ進める" }).click();
  await expect(traceDetail.locator("code")).toHaveText("sh -c $USERNAME");
  await page.locator('#phase-2 [data-a="next"]').click();
  await atNextBoundary(page, 2);
  await expect(page.getByLabel("修正するコード（Go）")).toBeVisible();
  await page.locator('#phase-3 [data-a="next"]').click();
  await atNextBoundary(page, 3);
  await expect(page.locator('.diff [data-change="removed"]')).toHaveCount(16);
  await expect(page.locator('.diff [data-change="context"]')).toHaveCount(4);
  await page.locator('#phase-4 [data-a="next"]').click();
  await atNextBoundary(page, 4);
  for (const service of ["vulnerable", "fixed"])
    await page.locator(`[data-a="compare"][data-v="${service}"]`).click();
  await expect(page.locator("#phase-5 .reveal")).toContainText("マーカーの有無に差があります");
  await page.locator('#phase-5 [data-a="next"]').click();
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

test("半画面でも脆弱版を修正版より先に表示する", async ({ page }) => {
  await page.setViewportSize({ width: 708, height: 900 });
  await mockRunner(page, { paired: true });
  await page.goto("/");
  await page.locator('[data-a="open"]').first().click();
  await startLesson(page);
  await page.locator('[data-a="deploy"]').click();
  await page.getByRole("checkbox").check();
  await page.getByRole("button", { name: "起動する", exact: true }).click();

  const endpoints = labEndpoints(page);
  await expect(endpoints).toHaveCount(2);
  await expect(endpoints.nth(0)).toContainText("脆弱版：http://127.0.0.1:18080");
  await expect(endpoints.nth(1)).toContainText("修正版：http://127.0.0.1:18081");
  const positions = await endpoints.evaluateAll((elements) =>
    elements.map((element) => element.getBoundingClientRect().top),
  );
  expect(positions[0]).toBeLessThan(positions[1]);
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
    await expect(outline).toContainText(`0 / ${PH.length} ステップ完了`);
    await page.getByRole("button", { name: "学習を開始する ↓", exact: true }).focus();
    await page.keyboard.press("Enter");
    await expect(page.locator("#phase-title-0")).toBeFocused();
    await atPhase(page, 0);
    await expect(steps.nth(0)).toHaveAttribute("aria-current", "step");
    await expect(outline).toContainText(`0 / ${PH.length} ステップ完了`);
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
    await expect(outline).toContainText(`2 / ${PH.length} ステップ完了`);
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
    await expect(outline).toContainText(`2 / ${PH.length} ステップ完了`);
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
