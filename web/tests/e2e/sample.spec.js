import { test, expect } from "@playwright/test";
import { readFile } from "node:fs/promises";

const sample = await readFile(new URL("../../../docs/ui/ui_sample.html", import.meta.url), "utf8");

async function ready(page) {
  await page.evaluate(() => document.fonts.ready);
}

for (const width of [1440, 390]) {
  test(`元HTMLとReactの表示が一致する (${width}px)`, async ({ page, context }, testInfo) => {
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
    for (const target of [page, reference]) await target.locator('[data-a="open"]').first().click();
    for (let phase = 0; phase < 5; phase++) {
      for (const target of [page, reference])
        await target.locator(`[data-a="go"][data-i="${phase}"]`).click();
      await compare(`phase-${phase}`);
    }
    for (const target of [page, reference]) {
      await target.locator('[data-a="deploy"]').click();
    }
    await compare("deploy-dialog");
    for (const target of [page, reference]) {
      await target.keyboard.press("Escape");
      await target.locator("#theme").click();
    }
    await compare("dark");
  });
}

test("教材の操作、デモの起動・停止・削除、完了表示", async ({ page }) => {
  const errors = [];
  page.on("pageerror", (error) => errors.push(error.message));
  const api = [];
  page.on("request", (request) => {
    if (request.url().includes("/api/")) api.push(request.url());
  });
  await page.goto("/");
  await page.getByRole("button", { name: "パストラバーサル 1", exact: true }).click();
  await expect(page.locator(".lcard")).toHaveCount(1);
  await expect(page.locator(".lcard")).toBeDisabled();
  await page.getByRole("button", { name: "すべて 2", exact: true }).click();
  await page.locator('[data-a="open"]').first().click();
  await page.getByRole("button", { name: "成功したログイン" }).click();
  await expect(page.locator(".log")).toContainText("200 / セッション発行");
  await page.getByRole("button", { name: "失敗したログイン" }).click();
  await expect(page.locator(".log")).toContainText("403 Forbidden");
  await page.locator('[data-a="deploy"]').click();
  await expect(page.getByRole("button", { name: "起動する", exact: true })).toBeDisabled();
  await page.getByRole("checkbox").check();
  await page.getByRole("button", { name: "起動する", exact: true }).click();
  await expect(page.locator(".st")).toHaveText("DEPLOYING");
  await page.locator('[data-a="cancel"]').click();
  await expect(page.locator(".st")).toHaveText("READY");
  await page.locator('[data-a="deploy"]').click();
  await page.getByRole("checkbox").check();
  await page.getByRole("button", { name: "起動する", exact: true }).click();
  await expect(page.locator(".st")).toHaveText("RUNNING", { timeout: 15000 });
  await page.locator('[data-a="next"]').click();
  for (let i = 0; i < 3; i++) await page.getByRole("button", { name: "ヒントを見る" }).click();
  await expect(page.getByRole("button", { name: "ヒントを見る" })).toBeDisabled();
  await page.getByRole("button", { name: "観察結果を取得" }).click();
  await expect(page.locator(".data")).toContainText("dummy-report.txt");
  await page.locator('[data-a="next"]').click();
  await page.getByRole("button", { name: "1つ戻る" }).click();
  await expect(page.locator(".nd.at")).toContainText("外部コマンドを実行する");
  await page.locator('[data-a="next"]').click();
  await page.locator('.diff [data-v="a"]').first().click();
  await expect(page.locator(".diff .hl")).toHaveCount(2);
  await page.locator('[data-a="next"]').click();
  for (const action of ["runV", "runF", "normF"])
    await page.locator(`[data-a="${action}"]`).click();
  await expect(page.locator(".reveal")).toContainText("確認できました。");
  await page.locator('[data-a="next"]').click();
  await page.locator('[data-a="stop"]').click();
  await expect(page.locator(".st")).toHaveText("STOPPED");
  await page.locator('[data-a="destroy"]').click();
  await page.locator("#vol").selectOption("rm");
  await page.getByRole("button", { name: "削除する", exact: true }).click();
  await expect(page.locator(".st")).toHaveText("READY");
  await page.locator('[data-a="home"]').first().click();
  await expect(page.locator(".lcard").first()).toContainText("Completed");
  expect(api).toEqual([]);
  expect(errors).toEqual([]);
});
