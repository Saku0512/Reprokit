import { test, expect } from "@playwright/test";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { launch } from "../../launch.mjs";

const vulnerable = (page) => page.getByRole("region", { name: "脆弱版", exact: true });
const fixed = (page) => page.getByRole("region", { name: "修正版", exact: true });
const markerInput = "id > /tmp/success; echo hook.action=block";

async function recordAndAdvance(page, note) {
  await page.getByLabel("結果を予想する").fill("固定Labの認証前処理を観察する。");
  await page.getByRole("button", { name: /予想を記録して期待結果を開く/ }).click();
  await page.getByLabel("観察メモ", { exact: true }).fill(note);
  await page.getByRole("button", { name: "メモを記録して次へ進む" }).click();
}

async function sendMarker(endpoint) {
  const response = await fetch(new URL("/api/login", endpoint), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username: markerInput, password: "anything" }),
  });
  expect(response.ok).toBeFalsy();
}

async function cleanupFromUI(page) {
  const card = vulnerable(page);
  try {
    const stop = card.getByRole("button", { name: "Labを停止する" });
    if (await stop.isEnabled()) {
      await stop.click();
      await expect(card).toContainText("停止済み", { timeout: 30000 });
    }
    const remove = card.getByRole("button", { name: "Labを削除する", exact: true });
    if (await remove.isEnabled()) {
      await remove.click();
      await page.getByRole("button", { name: "方針を確認してLabを削除する" }).click();
      await expect(card).toContainText("削除済み", { timeout: 30000 });
    }
  } catch {
    // テスト失敗時でも、後続の明示的なcleanupを妨げない。
  }
}

test("実RunnerとWeb UIでFile Browserの脆弱版・修正版を比較する", async ({ page }) => {
  test.skip(
    process.env.REPROKIT_RUN_FILEBROWSER_E2E !== "1",
    "実際の脆弱Labは専用環境でのみ、REPROKIT_RUN_FILEBROWSER_E2E=1 を指定して実行します。",
  );
  test.setTimeout(240000);
  const state = await mkdtemp(join(tmpdir(), "reprokit-filebrowser-e2e-"));
  let app;
  try {
    app = await launch({ port: 0, dist: false, state });
    await page.goto(`${app.origin}/runner`);
    await page.getByRole("button", { name: "学習を始める" }).click();
    await vulnerable(page).getByRole("button", { name: "Labを準備する" }).click();
    await expect(vulnerable(page)).toContainText("準備済み", { timeout: 120000 });
    await vulnerable(page).getByRole("button", { name: "安全性を確認して起動する" }).click();
    await page.getByRole("button", { name: "確認してLabを起動する", exact: true }).click();
    await expect(vulnerable(page)).toContainText("稼働中", { timeout: 60000 });
    await expect(fixed(page)).toContainText("稼働中");

    const vulnerableEndpoint = await vulnerable(page).getByRole("link").getAttribute("href");
    const fixedEndpoint = await fixed(page).getByRole("link").getAttribute("href");
    expect(vulnerableEndpoint).toMatch(/^http:\/\/127\.0\.0\.1:/);
    expect(fixedEndpoint).toMatch(/^http:\/\/127\.0\.0\.1:/);

    await recordAndAdvance(page, "脆弱版LabはRunnerの接続先から起動できた。");
    await recordAndAdvance(page, "ログイン入力の到達経路を確認する。");
    await recordAndAdvance(page, "シェル解釈の前に認証が失敗することを確認する。");
    await sendMarker(vulnerableEndpoint);
    await recordAndAdvance(page, "ログインは失敗したが、固定マーカーの観察へ進む。");
    await expect(page.locator("#page-title")).toHaveText("脆弱版の観察結果を確かめる");
    await page.getByRole("button", { name: "脆弱版のマーカーを確認する" }).click();
    await expect(page.getByText(/マーカーを確認しました。内容: uid=/)).toBeVisible();

    await recordAndAdvance(page, "脆弱版で認証前に固定マーカーが作られた。");
    await expect(page.locator("#page-title")).toHaveText("修正版で同じ入力を比較する");
    await sendMarker(fixedEndpoint);
    await page.getByRole("button", { name: "修正版のマーカーを確認する" }).click();
    await expect(
      page.getByText("マーカーはありません。修正版ではこの結果が期待値です。"),
    ).toBeVisible();

    await fixed(page).getByRole("button", { name: "Labを停止する" }).click();
    await expect(vulnerable(page)).toContainText("停止済み");
    await vulnerable(page).getByRole("button", { name: "Labを削除する", exact: true }).click();
    await page.getByRole("button", { name: "方針を確認してLabを削除する" }).click();
    await expect(vulnerable(page)).toContainText("削除済み");
    await expect(fixed(page)).toContainText("削除済み");
  } finally {
    await cleanupFromUI(page);
    await app?.close();
    await rm(state, { recursive: true, force: true });
  }
});
