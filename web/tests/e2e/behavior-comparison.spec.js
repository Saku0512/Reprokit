import { test, expect } from "@playwright/test";
import { mockRunner } from "./fixture.js";

const variant = (page, service) =>
  page.getByRole("region", {
    name: `${service === "vulnerable" ? "脆弱版" : "修正版"}の挙動`,
    exact: true,
  });
const observe = (page, service) => variant(page, service).locator('[data-a="compare"]');

async function openComparison(page) {
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.goto("/");
  await page.locator('[data-a="open"]').first().click();
  await page.getByRole("button", { name: "学習を開始する ↓", exact: true }).click();
  for (let phase = 0; phase < 5; phase++)
    await page.locator(`#phase-${phase} [data-a="next"]`).click();
}

async function deploy(page) {
  await page.locator('[data-a="deploy"]').click();
  await page.getByRole("checkbox").check();
  await page.getByRole("button", { name: "起動する", exact: true }).click();
  await expect(page.locator(".st")).toHaveText("RUNNING");
}

for (const width of [1440, 390]) {
  test(`同じ入力の処理と実際のマーカー結果を比較できる (${width}px)`, async ({
    page,
  }, testInfo) => {
    await page.setViewportSize({ width, height: 1200 });
    const fixture = await mockRunner(page, { paired: true });
    await openComparison(page);
    for (const service of ["vulnerable", "fixed"]) {
      await expect(variant(page, service).getByRole("status")).toContainText("未確認");
      await expect(observe(page, service)).toBeDisabled();
      await expect(variant(page, service)).toContainText("403 Forbidden");
    }
    await expect(page.locator(".comparison-input")).toContainText(
      "id > /tmp/success; echo hook.action=block",
    );
    await page.context().grantPermissions(["clipboard-read", "clipboard-write"]);
    await page
      .locator(".comparison-input")
      .getByRole("button", { name: "Usernameをコピー" })
      .click();
    await expect
      .poll(() => page.evaluate(() => navigator.clipboard.readText()))
      .toBe("id > /tmp/success; echo hook.action=block");
    await page.locator("#phase-1").getByRole("button", { name: "Usernameをコピー" }).click();
    await expect
      .poll(() => page.evaluate(() => navigator.clipboard.readText()))
      .toBe("id > /tmp/success; echo hook.action=block");
    await expect(variant(page, "vulnerable")).toContainText("シェルが入力を命令として解釈");
    await expect(variant(page, "fixed")).toContainText("入力をシェル構文に再解釈しない");
    expect(fixture.calls.filter((call) => call.path.includes("/markers/"))).toHaveLength(0);
    await deploy(page);
    for (const [service, port] of [
      ["vulnerable", 18080],
      ["fixed", 18081],
    ]) {
      await expect(variant(page, service).getByRole("link")).toHaveAttribute(
        "href",
        `http://127.0.0.1:${port}`,
      );
      await observe(page, service).focus();
      await page.keyboard.press("Enter");
    }
    await expect(variant(page, "vulnerable").getByRole("status")).toContainText("マーカーあり");
    await expect(variant(page, "vulnerable").getByRole("status")).toContainText(
      "uid=1000(filebrowser)",
    );
    await expect(variant(page, "fixed").getByRole("status")).toContainText("マーカーなし");
    await expect(variant(page, "fixed").getByRole("status")).toContainText("未送信でも同じ結果");
    await expect(page.locator(".behavior-comparison .reveal")).toContainText(
      "マーカーの有無に差があります",
    );
    expect(
      fixture.calls
        .filter((call) => call.path.includes("/markers/"))
        .map((call) => ({
          service: call.path.split("/").at(-1),
          method: call.method,
          body: call.body,
        })),
    ).toEqual([
      { service: "vulnerable", method: "GET", body: null },
      { service: "fixed", method: "GET", body: null },
    ]);
    expect(fixture.calls.filter((call) => call.path.includes("/actions/"))).toHaveLength(0);
    expect(
      await page.locator(".detail").evaluate((node) => node.scrollWidth <= node.clientWidth),
    ).toBe(true);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(
      true,
    );
    const left = await variant(page, "vulnerable").boundingBox();
    const right = await variant(page, "fixed").boundingBox();
    if (width > 700) expect(Math.abs(left.y - right.y)).toBeLessThan(1);
    else expect(right.y).toBeGreaterThan(left.y + left.height);
    await page.evaluate(() => document.fonts.ready);
    await expect(page.locator(".toast")).not.toHaveClass(/\bon\b/);
    for (const theme of ["light", "dark"]) {
      if (theme === "dark") await page.getByRole("button", { name: "表示切替" }).click();
      if (width > 700) {
        await page.locator(".behavior-comparison").screenshot({
          path: testInfo.outputPath(`comparison-${theme}.png`),
          animations: "disabled",
        });
      } else {
        // スクロール領域より長い親の切り抜きではなく、画面内に収まる各版を撮影する。
        for (const service of ["vulnerable", "fixed"]) {
          await variant(page, service).scrollIntoViewIfNeeded();
          await variant(page, service).screenshot({
            path: testInfo.outputPath(`comparison-${service}-${theme}.png`),
            animations: "disabled",
          });
        }
      }
    }
    await page.locator('[data-a="stop"]').click();
    await expect(page.locator(".st")).toHaveText("STOPPED");
    for (const service of ["vulnerable", "fixed"]) {
      await expect(variant(page, service).getByRole("status")).toContainText("未確認");
      await expect(observe(page, service)).toBeDisabled();
    }
    await deploy(page);
    await observe(page, "fixed").click();
    await expect(variant(page, "fixed").getByRole("status")).toContainText("マーカーなし");
    await page.locator('[data-a="destroy"]').click();
    await page.getByRole("button", { name: "削除する", exact: true }).click();
    await expect(variant(page, "fixed").getByRole("status")).toContainText("未確認");
    await expect(observe(page, "fixed")).toBeDisabled();
    await expect(variant(page, "fixed").getByRole("link")).toHaveCount(0);
  });
}

test("想定と逆の観察結果を成功として表示しない", async ({ page }) => {
  await mockRunner(page, { paired: true });
  await page.route("**/markers/*", (route) => {
    const service = route.request().url().split("/").at(-1);
    return route.fulfill({ json: { service, exists: service === "fixed", content: "old marker" } });
  });
  await openComparison(page);
  await deploy(page);
  for (const service of ["vulnerable", "fixed"]) {
    await observe(page, service).click();
    await expect(variant(page, service).getByRole("status")).toContainText("想定と異なる結果");
  }
  await expect(variant(page, "vulnerable").getByRole("status")).toContainText("マーカーなし");
  await expect(variant(page, "fixed").getByRole("status")).toContainText("マーカーあり");
  await expect(page.locator(".behavior-comparison .reveal")).toContainText(
    "想定した差をまだ確認できていません",
  );
  await expect(page.locator(".marker-result.success")).toHaveCount(0);
});

test("再取得中と通信失敗で古い成功表示を消し、失敗した版だけ再試行できる", async ({ page }) => {
  const fixture = await mockRunner(page, { paired: true });
  await openComparison(page);
  await deploy(page);
  for (const service of ["vulnerable", "fixed"]) await observe(page, service).click();
  await expect(page.locator(".behavior-comparison .reveal")).toBeVisible();
  let release;
  const gate = new Promise((resolve) => {
    release = resolve;
  });
  await page.route("**/markers/fixed", async (route) => {
    await gate;
    await route.fulfill({ status: 503, json: { failure: "marker temporarily unavailable" } });
  });
  await observe(page, "fixed").click();
  await expect(observe(page, "fixed")).toBeDisabled();
  await expect(variant(page, "fixed").getByRole("status")).toContainText("取得中");
  await expect(variant(page, "fixed").getByRole("status")).not.toContainText("マーカーなし");
  await expect(page.locator(".behavior-comparison .reveal")).toHaveCount(0);
  release();
  await expect(variant(page, "fixed").getByRole("status")).toContainText(
    "marker temporarily unavailable",
  );
  await expect(variant(page, "fixed").getByRole("status")).not.toContainText("マーカーなし");
  await expect(variant(page, "vulnerable").getByRole("status")).toContainText("マーカーあり");
  await page.unroute("**/markers/fixed");
  await observe(page, "fixed").click();
  await expect(variant(page, "fixed").getByRole("status")).toContainText("マーカーなし");
  expect(fixture.calls.filter((call) => call.path.endsWith("/markers/vulnerable"))).toHaveLength(1);
});

test("Lab停止前の遅延レスポンスで観察済み状態へ戻らない", async ({ page }) => {
  await mockRunner(page, { paired: true });
  await openComparison(page);
  await deploy(page);
  let release;
  const gate = new Promise((resolve) => {
    release = resolve;
  });
  await page.route("**/markers/vulnerable", async (route) => {
    await gate;
    await route.fulfill({ json: { service: "vulnerable", exists: true, content: "late marker" } });
  });
  await observe(page, "vulnerable").click();
  await expect(variant(page, "vulnerable").getByRole("status")).toContainText("取得中");
  await page.locator('[data-a="stop"]').click();
  await expect(page.locator(".st")).toHaveText("STOPPED");
  const response = page.waitForResponse("**/markers/vulnerable");
  release();
  await response;
  await page.evaluate(
    () => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))),
  );
  await expect(variant(page, "vulnerable").getByRole("status")).toContainText("未確認");
  await expect(variant(page, "vulnerable")).not.toContainText("late marker");
  await expect(observe(page, "vulnerable")).toBeDisabled();
});

test("別serviceのレスポンスを対象の結果として表示しない", async ({ page }) => {
  await mockRunner(page, { paired: true });
  await page.route("**/markers/fixed", (route) =>
    route.fulfill({ json: { service: "vulnerable", exists: false } }),
  );
  await openComparison(page);
  await deploy(page);
  await observe(page, "fixed").click();
  await expect(variant(page, "fixed").getByRole("status")).toContainText(
    "対象のマーカーを確認できませんでした",
  );
  await expect(variant(page, "fixed").getByRole("status")).not.toContainText("マーカーなし");
});
