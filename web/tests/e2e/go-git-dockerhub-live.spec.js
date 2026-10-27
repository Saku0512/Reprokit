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
    "docker.io/saku0512/reprokit_cve-2026-71557@sha256:2c32850969c685ccac825ea5479fa86f8d319223cb11aae4cb55bc0361447267",
  fixed:
    "docker.io/saku0512/reprokit_cve-2026-71557@sha256:81b410dd1c0418ebd680ed90341695e37581326db4812c8f362beb0c72832e97",
};

async function docker(...args) {
  const { stdout } = await execFileAsync("docker", args, { timeout: 30000 });
  return stdout.trim();
}

async function resources(project) {
  const output = await docker(
    "ps",
    "--all",
    "--quiet",
    "--filter",
    `label=com.docker.compose.project=${project}`,
  );
  return output ? output.split("\n") : [];
}

async function runner(app, path, method = "GET", body) {
  const response = await fetch(`${app.runner.origin}${path}`, {
    method,
    headers: {
      "X-Reprokit-Session": app.runner.token,
      ...(body === undefined ? {} : { "Content-Type": "application/json" }),
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const payload = await response.json();
  // Node.jsのFetch APIではokはPlaywright Responseと異なり真偽値である。
  expect(response.ok, JSON.stringify(payload)).toBe(true);
  return payload;
}

test("go-git Docker HubイメージをRunnerで比較し、固定検証結果と片付けを確認する", async () => {
  test.skip(
    process.env.REPROKIT_RUN_DOCKERHUB_E2E !== "1",
    "専用環境で明示的に実行する実Lab E2Eです。",
  );
  test.setTimeout(240000);
  const state = await mkdtemp(join(tmpdir(), "reprokit-go-git-e2e-"));
  let app;
  let project;
  try {
    app = await launch({ port: 0, dist: false, state });
    const session = await runner(app, "/api/labs", "POST", {
      lessonId: "path-traversal/go-git-loose-reference",
      lessonVersion: "1.0.0",
      labRef: "vulnerable",
    });
    expect(session.labId).toBe("reprokit/go-git-cve-2026-71557");
    const stored = JSON.parse(
      await readFile(join(state, "labs", `${session.labSessionId}.json`), "utf8"),
    );
    expect(stored.sourceSubdirectory).toBe("images/CVE-2026-71557");
    project = stored.project;
    const sourceCompose = Buffer.from(stored.sourceCompose, "base64").toString("utf8");
    for (const image of Object.values(images)) expect(sourceCompose).toContain(image);
    expect(await resources(project)).toEqual([]);

    await runner(app, `/api/labs/${session.labSessionId}/start`, "POST", { confirmed: true });
    const ids = await resources(project);
    expect(ids).toHaveLength(2);
    const actualImages = [];
    for (const id of ids)
      actualImages.push(JSON.parse(await docker("inspect", id))[0].Config.Image);
    expect(actualImages.sort()).toEqual(Object.values(images).sort());
    const vulnerable = await runner(
      app,
      `/api/labs/${session.labSessionId}/actions/vulnerable/reference_check`,
      "POST",
    );
    const fixed = await runner(
      app,
      `/api/labs/${session.labSessionId}/actions/fixed/reference_check`,
      "POST",
    );
    expect(vulnerable.output).toContain("VULNERABLE");
    expect(fixed.output).toContain("FIXED");

    await runner(app, `/api/labs/${session.labSessionId}/stop`, "POST");
    await runner(app, `/api/labs/${session.labSessionId}?volumePolicy=delete`, "DELETE");
    expect(await resources(project)).toEqual([]);
  } finally {
    if (project)
      await execFileAsync("docker", [
        "compose",
        "--project-name",
        project,
        "down",
        "--volumes",
        "--remove-orphans",
      ]).catch(() => {});
    await app?.close();
    await rm(state, { recursive: true, force: true });
  }
});
