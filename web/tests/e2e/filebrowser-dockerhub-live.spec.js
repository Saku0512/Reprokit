import { test, expect } from "@playwright/test";
import { execFile } from "node:child_process";
import { mkdtemp, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { promisify } from "node:util";
import { launch } from "../../launch.mjs";

const execFileAsync = promisify(execFile);
const images = {
  vulnerable:
    "docker.io/saku0512/reprokit_cve-2026-54088@sha256:cdf1d4e63e4630f0f473e7f9cfd381c20450e65a87b86bceb9c944883ff6cd95",
  fixed:
    "docker.io/saku0512/reprokit_cve-2026-54088@sha256:22164b8c14c41f5aa4556f21442af35bc460a7d5ff85e262579fd37d5e3205a5",
};
const card = (page, name) => page.getByRole("region", { name, exact: true });

async function docker(...args) {
  const { stdout } = await execFileAsync("docker", args, { timeout: 30000 });
  return stdout.trim();
}

async function resources(project, service) {
  const args = [
    "ps",
    "--all",
    "--quiet",
    "--filter",
    `label=com.docker.compose.project=${project}`,
  ];
  if (service) args.push("--filter", `label=com.docker.compose.service=${service}`);
  const output = await docker(...args);
  return output ? output.split("\n") : [];
}

async function advance(page) {
  await page.getByLabel("結果を予想する").fill("認証前の入力を観察する。");
  await page.getByRole("button", { name: /予想を記録して期待結果を開く/ }).click();
  await page.getByLabel("観察メモ", { exact: true }).fill("実際のLabで確認する。");
  await page.getByRole("button", { name: "メモを記録して次へ進む" }).click();
}

async function login(endpoint) {
  const response = await fetch(new URL("/api/login", endpoint), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      username: "id > /tmp/success; echo hook.action=block",
      password: "anything",
    }),
    signal: AbortSignal.timeout(10000),
  });
  expect(response.ok).toBe(false);
}

test("Docker Hub版を実RunnerとWeb UIで再現し、リソースを削除する", async ({ page }) => {
  test.skip(
    process.env.REPROKIT_RUN_DOCKERHUB_E2E !== "1",
    "専用環境で明示的に実行する実Lab E2Eです。",
  );
  test.setTimeout(240000);
  const state = await mkdtemp(join(tmpdir(), "reprokit-dockerhub-e2e-"));
  let app;
  let project;
  let cleaned = false;
  try {
    app = await launch({ port: 0, dist: false, state });
    await page.goto(`${app.origin}/runner`);
    await page.getByRole("button", { name: "学習を始める" }).click();
    const preparedResponse = page.waitForResponse(
      (response) => response.url().endsWith("/api/labs") && response.request().method() === "POST",
    );
    await card(page, "脆弱版").getByRole("button", { name: "Labを準備する" }).click();
    const response = await preparedResponse;
    expect(response.ok()).toBe(true);
    const session = await response.json();
    expect(session.state).toBe("prepared");
    expect(session.labId).toBe("reprokit/filebrowser-cve-2026-54088");
    expect(session.labSessionId).toMatch(/^[0-9a-f-]{36}$/);
    expect(session.sourceRevision).toMatch(/^[0-9a-f]{40}$/);
    expect(session.safetyChecked).toBe(true);
    expect(session.externalExposure).toBe(false);
    const stored = JSON.parse(
      await readFile(join(state, "labs", `${session.labSessionId}.json`), "utf8"),
    );
    expect(stored.sourceUrl).toBe("https://github.com/Saku0512/Reprokit.git");
    expect(stored.sourceSubdirectory).toBe("images/CVE-2026-54088");
    expect(stored.runtimeFile).toBe("docker-compose.yml");
    project = stored.project;
    expect(project).toMatch(/^reprokit_[0-9a-f]{12}$/);
    expect(await resources(project)).toEqual([]);
    const sourceCompose = Buffer.from(stored.sourceCompose, "base64").toString("utf8");
    const lock = Buffer.from(stored.lockfile, "base64").toString("utf8");
    for (const image of Object.values(images)) {
      expect(sourceCompose).toContain(image);
      expect(lock).toContain(image.split("@sha256:")[1]);
    }

    await card(page, "脆弱版").getByRole("button", { name: "安全性を確認して起動する" }).click();
    await page.getByRole("button", { name: "確認してLabを起動する", exact: true }).click();
    await expect(card(page, "脆弱版")).toContainText("稼働中", { timeout: 60000 });
    await expect(card(page, "修正版")).toContainText("稼働中");
    for (const [service, image] of Object.entries(images)) {
      const ids = await resources(project, service);
      expect(ids).toHaveLength(1);
      const actual = JSON.parse(await docker("inspect", ids[0]));
      expect(actual[0].Config.Image).toBe(image);
      expect(actual[0].State.Running).toBe(true);
    }
    const vulnerableEndpoint = await card(page, "脆弱版").getByRole("link").getAttribute("href");
    const fixedEndpoint = await card(page, "修正版").getByRole("link").getAttribute("href");
    expect(vulnerableEndpoint).toMatch(/^http:\/\/127\.0\.0\.1:\d+\/?$/);
    expect(fixedEndpoint).toMatch(/^http:\/\/127\.0\.0\.1:\d+\/?$/);
    expect(fixedEndpoint).not.toBe(vulnerableEndpoint);
    await advance(page);
    await advance(page);
    await advance(page);
    await login(vulnerableEndpoint);
    await advance(page);
    await page.getByRole("button", { name: "脆弱版のマーカーを確認する" }).click();
    await expect(page.getByText(/マーカーを確認しました。内容: uid=/)).toBeVisible();
    await advance(page);
    await login(fixedEndpoint);
    await page.getByRole("button", { name: "修正版のマーカーを確認する" }).click();
    await expect(
      page.getByText("マーカーはありません。修正版ではこの結果が期待値です。"),
    ).toBeVisible();
    await card(page, "脆弱版").getByRole("button", { name: "Labを停止する" }).click();
    await expect(card(page, "脆弱版")).toContainText("停止済み");
    await card(page, "脆弱版").getByRole("button", { name: "Labを削除する", exact: true }).click();
    await page.getByLabel("Volumeの削除方針").selectOption("delete");
    await page.getByRole("button", { name: "方針を確認してLabを削除する" }).click();
    await expect(card(page, "脆弱版")).toContainText("削除済み");
    expect(await resources(project)).toEqual([]);
    expect(
      await docker(
        "network",
        "ls",
        "--quiet",
        "--filter",
        `label=com.docker.compose.project=${project}`,
      ),
    ).toBe("");
    expect(
      await docker(
        "volume",
        "ls",
        "--quiet",
        "--filter",
        `label=com.docker.compose.project=${project}`,
      ),
    ).toBe("");
    cleaned = true;
  } finally {
    if (project && !cleaned) {
      try {
        const vulnerable = card(page, "脆弱版");
        const stop = vulnerable.getByRole("button", { name: "Labを停止する" });
        if (await stop.isEnabled()) {
          await stop.click();
          await expect(vulnerable).toContainText("停止済み", { timeout: 30000 });
        }
        const remove = vulnerable.getByRole("button", { name: "Labを削除する", exact: true });
        if (await remove.isEnabled()) {
          await remove.click();
          await page.getByLabel("Volumeの削除方針").selectOption("delete");
          await page.getByRole("button", { name: "方針を確認してLabを削除する" }).click();
          await expect(vulnerable).toContainText("削除済み", { timeout: 30000 });
        }
        const containers = await resources(project);
        const networks = await docker(
          "network",
          "ls",
          "--quiet",
          "--filter",
          `label=com.docker.compose.project=${project}`,
        );
        const volumes = await docker(
          "volume",
          "ls",
          "--quiet",
          "--filter",
          `label=com.docker.compose.project=${project}`,
        );
        cleaned = containers.length === 0 && networks === "" && volumes === "";
      } catch (error) {
        console.error(`Lab cleanup failed: ${error}`);
      }
    }
    await app?.close();
    if (cleaned || !project) await rm(state, { recursive: true, force: true });
    else
      console.error(
        `Lab cleanup is unverified; preserve state directory: ${state} (project ${project})`,
      );
  }
});
