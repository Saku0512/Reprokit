import assert from "node:assert/strict";
import { execFileSync, spawnSync } from "node:child_process";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";
import { isValidSubject } from "./commit-message.mjs";

test("許可されたprefixとcomponentを持つ件名を受け入れる", () => {
  for (const prefix of ["feat", "fix", "docs", "test", "ci", "refactor", "chore"]) {
    assert.equal(isValidSubject(`${prefix}(web-api): 受講画面を更新`), true);
  }
  assert.equal(isValidSubject("test(web): 受講画面の回帰テストを拡充"), true);
});

test("形式が不正な件名を拒否する", () => {
  for (const subject of [
    "merge(web): 変更",
    "fix: 変更",
    "fix(): 変更",
    "fix(Web): 変更",
    "fix(-web): 変更",
    "fix(web-): 変更",
    "fix(web):変更",
    "fix(web): ",
    "fix(web):  変更",
    "fix(web): 変更\n別行",
    "fix(web): 変更\n",
  ]) {
    assert.equal(isValidSubject(subject), false, subject);
  }
});

test("PRの新規コミットとタイトルを検証し、既存履歴を検査しない", () => {
  const directory = mkdtempSync(join(tmpdir(), "reprokit-commit-check-"));
  const git = (...args) => execFileSync("git", args, { cwd: directory, encoding: "utf8" }).trim();
  const script = fileURLToPath(new URL("./check-commit-messages.mjs", import.meta.url));
  const check = (base, head, title) =>
    spawnSync(process.execPath, [script], {
      cwd: directory,
      env: { ...process.env, BASE_SHA: base, HEAD_SHA: head, PR_TITLE: title },
      encoding: "utf8",
    });

  try {
    git("init", "-q");
    git("config", "user.name", "Test");
    git("config", "user.email", "test@example.invalid");
    git("commit", "--allow-empty", "-qm", "old history");
    const base = git("rev-parse", "HEAD");
    git("commit", "--allow-empty", "-qm", "feat(web): 新機能");
    const validHead = git("rev-parse", "HEAD");
    assert.equal(check(base, validHead, "feat(web): 新機能").status, 0);
    assert.equal(check(base, validHead, "invalid title").status, 1);
    git("commit", "--allow-empty", "-qm", "invalid commit");
    const result = check(base, git("rev-parse", "HEAD"), "feat(web): 新機能");
    assert.equal(result.status, 1);
    assert.match(result.stderr, /invalid commit/);
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
});
