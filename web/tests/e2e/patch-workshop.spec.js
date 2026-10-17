import { test, expect } from "@playwright/test";

for (const width of [1440, 390]) {
  test(`コードを編集し、指摘を確認して公式パッチと照らす (${width}px)`, async ({
    page,
  }, testInfo) => {
    await page.setViewportSize({ width, height: 1000 });
    await page.emulateMedia({ reducedMotion: "reduce" });
    const calls = [];
    await page.route("**/api/**", (route) => {
      calls.push(route.request().url());
      return route.abort();
    });
    await page.goto("/");
    await page.locator('[data-a="open"]').first().click();
    await page.getByRole("button", { name: "学習を開始する ↓", exact: true }).click();
    for (let phase = 0; phase < 3; phase++) {
      await page.locator(`#phase-${phase} [data-a="next"]`).click();
    }
    await expect(
      page.getByRole("heading", { name: "自分でコードを修正する", exact: true }),
    ).toBeVisible();
    await expect(
      page.getByRole("heading", { name: "公式パッチと照らす", exact: true }),
    ).toHaveCount(0);
    const editor = page.getByLabel("修正するコード（Go）");
    const result = page.getByRole("region", { name: "修正チェックの結果" });
    const check = page.getByRole("button", { name: "修正内容をチェック", exact: true });
    const original = await editor.inputValue();
    await check.click();
    await expect(result).toContainText("修正を見直してください");
    await expect(result).toContainText("os.Expandで資格情報が引数へ入ります");
    for (let hint = 0; hint < 3; hint++) {
      await page.getByRole("button", { name: "修正のヒントを見る", exact: true }).click();
    }
    await expect(page.locator(".patch-hints li")).toHaveCount(3);
    await expect(
      page.getByRole("button", { name: "修正のヒントを見る", exact: true }),
    ).toBeDisabled();
    // 入力の削除だけでなく、資格情報の受け渡しが保たれる修正を確認する。
    const fixed = original.replace(/envMapping :=[\s\S]*?(?=cmd := exec.Command)/, "");
    await editor.fill(
      fixed
        .split("\n")
        .filter((line) => !line.includes("PASSWORD="))
        .join("\n"),
    );
    await expect(result).not.toContainText("os.Expandで資格情報が引数へ入ります");
    await check.click();
    await expect(result).toContainText("要修正：PASSWORDを環境変数で渡す");
    await editor.fill(fixed);
    await check.click();
    await expect(result).toContainText("抜粋の修正条件を満たしました");
    await page.locator('#phase-3 [data-a="next"]').click();
    await expect(
      page.getByRole("heading", { name: "公式パッチと照らす", exact: true }),
    ).toBeVisible();
    const diff = page.getByLabel("演習コードの修正差分", { exact: true });
    // 表示した差分から元コードと修正後コードを復元し、演習との整合性を確認する。
    const before = await diff
      .locator("[data-change]")
      .evaluateAll((lines) => lines.map((line) => line.textContent.slice(2)).join("\n"));
    const after = await diff
      .locator('[data-change="context"]')
      .evaluateAll((lines) => lines.map((line) => line.textContent.slice(2)).join("\n"));
    expect(before).toBe(original);
    expect(after).toBe(fixed);
    await expect(diff.locator(".add")).toHaveCount(0);
    await expect(diff.locator('[data-change="context"]')).toContainText([
      "strings.Split(a.Command",
      "exec.Command(command[0]",
      'fmt.Sprintf("USERNAME=%s"',
      'fmt.Sprintf("PASSWORD=%s"',
    ]);
    await expect(page.locator("#phase-4")).toContainText("修正前からあり、そのまま残っています");
    await page.screenshot({
      path: testInfo.outputPath("patch-comparison.png"),
      animations: "disabled",
    });
    await page.locator('#phase-4 [data-a="prev"]').click();
    await expect(editor).toHaveValue(fixed);
    expect(
      await page
        .locator(".detail")
        .evaluate((element) => element.scrollWidth <= element.clientWidth),
    ).toBe(true);
    await page.screenshot({
      path: testInfo.outputPath("patch-workshop.png"),
      animations: "disabled",
    });
    await editor.fill(`${fixed}\ncmd.Output()`);
    await expect(result).not.toContainText("抜粋の修正条件を満たしました");
    await check.click();
    await expect(result).toContainText("追加した処理や構文は判定できません");
    await page.getByRole("button", { name: "元のコードに戻す", exact: true }).click();
    await expect(editor).toHaveValue(original);
    await expect(result).toContainText("コードを編集してチェックすると");
    expect(calls).toEqual([]);
  });
}
