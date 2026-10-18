import { test } from "node:test";
import assert from "node:assert/strict";
import {
  emptyNote,
  noteStorageKey,
  readNote,
  writeNote,
  finishNote,
} from "../src/learner/learning-note.ts";

const lesson = { lessonId: "command-injection/filebrowser-auth-hook", lessonVersion: "1.0.0" };
const answers = { impact: "影響", cause: "原因", remediation: "修正", prevention: "再発防止" };
function memoryStorage() {
  const values = new Map();
  return {
    getItem: (key) => values.get(key) ?? null,
    setItem: (key, value) => values.set(key, value),
  };
}

test("下書きと質問の位置を復元し、教材と版が異なる記録を混ぜない", () => {
  const storage = memoryStorage();
  const note = { ...emptyNote(lesson), draft: { ...answers, prevention: "" }, step: 2 };
  assert.equal(writeNote(note, storage), true);
  assert.deepEqual(readNote(lesson, storage), { note, failure: "" });
  for (const identity of [
    { ...lesson, lessonVersion: "2.0.0" },
    { ...lesson, lessonId: "other" },
  ]) {
    assert.deepEqual(readNote(identity, storage).note, emptyNote(identity));
  }
});

test("空白だけの回答では完成せず、カード保存後の下書き編集でカードを変更しない", () => {
  const note = emptyNote(lesson);
  note.draft = { ...answers, prevention: " \n " };
  assert.equal(finishNote(note), null);
  note.draft.prevention = " テストで再発を防ぐ ";
  const complete = finishNote(note, new Date("2026-10-18T00:00:00Z"));
  assert.equal(complete.saved.answers.prevention, "テストで再発を防ぐ");
  complete.draft.cause = "書き直し中";
  assert.equal(complete.saved.answers.cause, "原因");
  const storage = memoryStorage();
  writeNote(complete, storage);
  assert.deepEqual(readNote(lesson, storage).note, complete);
});

test("壊れた保存データや不正な質問位置、回答、カードを画面へ渡さない", () => {
  const storage = memoryStorage();
  const valid = { ...emptyNote(lesson), draft: answers };
  for (const value of [
    "{broken",
    "null",
    "[]",
    "42",
    JSON.stringify({ ...valid, step: -1 }),
    JSON.stringify({ ...valid, step: 4 }),
    JSON.stringify({ ...valid, step: 1.5 }),
    JSON.stringify({ ...valid, draft: { ...answers, cause: 12 } }),
    JSON.stringify({ ...valid, draft: { ...answers, cause: "x".repeat(601) } }),
    JSON.stringify({ ...valid, lessonVersion: "2.0.0" }),
    JSON.stringify({ ...valid, saved: {} }),
    JSON.stringify({ ...valid, saved: { answers, updatedAt: "invalid" } }),
    JSON.stringify({
      ...valid,
      saved: { answers: { ...answers, cause: " " }, updatedAt: "2026-10-18" },
    }),
  ]) {
    storage.setItem(noteStorageKey(lesson), value);
    const result = readNote(lesson, storage);
    assert.deepEqual(result.note, emptyNote(lesson));
    assert.ok(result.failure);
    assert.equal(storage.getItem(noteStorageKey(lesson)), value);
  }
});

test("保存先が使えなくても例外を漏らさず、成功を返さない", () => {
  const storage = {
    getItem() {
      throw new Error("blocked");
    },
    setItem() {
      throw new Error("quota");
    },
  };
  assert.ok(readNote(lesson, storage).failure);
  assert.equal(writeNote(emptyNote(lesson), storage), false);
});
