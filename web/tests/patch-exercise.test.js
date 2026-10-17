import { test } from "node:test";
import assert from "node:assert/strict";
import { checkPatch, starterPatch } from "../src/learner/patch-exercise.ts";

const fixed = `command := strings.Split(a.Command, " ")
cmd := exec.Command(command[0], command[1:]...)
cmd.Env = append(os.Environ(), fmt.Sprintf("USERNAME=%s", a.Cred.Username))
cmd.Env = append(cmd.Env, fmt.Sprintf("PASSWORD=%s", a.Cred.Password))`;

test("未修正は環境変数を渡していても展開処理を指摘する", () => {
  const result = checkPatch(starterPatch);
  assert.equal(result.passed, false);
  assert.deepEqual(
    result.checks.map((check) => check.passed),
    [true, false, true, true],
  );
});

test("マッピングと展開ループを削除し、資格情報の環境変数を残せば修正条件を満たす", () => {
  assert.equal(checkPatch(fixed).passed, true);
  assert.equal(checkPatch(`// os.Expandは削除した\n${fixed}`).passed, true);
  assert.equal(checkPatch(fixed.replaceAll(" := ", ":=").replaceAll("\n", "\n\t")).passed, true);
});

test("ループだけを削除して未使用のenvMappingが残れば指摘する", () => {
  const result = checkPatch(
    starterPatch.replace(/for i, arg := range command \{[\s\S]*?\n\}\n/, ""),
  );
  assert.equal(result.passed, false);
  assert.match(result.checks.find((check) => !check.passed).detail, /未使用/);
});

test("資格情報を渡す処理まで削除すると修正成功にしない", () => {
  for (const key of ["USERNAME", "PASSWORD"]) {
    const result = checkPatch(
      fixed
        .split("\n")
        .filter((line) => !line.includes(key))
        .join("\n"),
    );
    assert.equal(result.passed, false);
    assert.ok(result.checks.some((check) => !check.passed));
  }
});

test("コメント中の正解、構文の破損、追加された呼び出しを受け入れない", () => {
  for (const code of [
    "",
    `/* ${fixed} */`,
    fixed.replace("USERNAME=", "USER="),
    fixed.replace('" "', '"'),
    fixed.replace("cmd.Env =", "cmd.Env =="),
    `${fixed}\ncmd.Output()`,
    `${fixed}\nfetch("https://example.invalid")`,
    `${fixed}\n/* 閉じていないコメント`,
  ]) {
    const result = checkPatch(code);
    assert.equal(result.passed, false);
    assert.ok(result.error);
  }
});

test("大きすぎる入力は判定しない", () => {
  assert.equal(checkPatch("x".repeat(12001)).passed, false);
  assert.match(checkPatch("x".repeat(12001)).error, /12,000/);
});
