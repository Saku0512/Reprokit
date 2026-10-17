import { test } from "node:test";
import assert from "node:assert/strict";
import {
  answerPresentation,
  assignmentSummary,
  completionDescription,
  filterAssignments,
  parseLearnerIDs,
} from "../src/admin-model.js";

const assignments = [
  {
    learnerId: "alice",
    courseId: "secure-web",
    courseVersion: "1",
    status: "assigned",
    dueAt: "2026-10-10T00:00:00Z",
  },
  {
    learnerId: "bob",
    courseId: "secure-web",
    courseVersion: "1",
    status: "in_progress",
    dueAt: "2026-10-20T00:00:00Z",
  },
  {
    learnerId: "carol",
    courseId: "review",
    courseVersion: "2",
    status: "completed",
    dueAt: "2026-10-01T00:00:00Z",
  },
  {
    learnerId: "dave",
    courseId: "review",
    courseVersion: "2",
    status: "needs_retry",
    dueAt: "2026-10-15T00:00:00Z",
  },
];

test("進捗を状態別に集計し、完了以外の期限超過だけを数える", () => {
  assert.deepEqual(assignmentSummary(assignments, new Date("2026-10-17T00:00:00Z")), {
    total: 4,
    counts: { assigned: 1, in_progress: 1, submitted: 0, completed: 1, needs_retry: 1 },
    overdue: 2,
    completionRate: 25,
  });
});

test("複数の対象者IDを改行またはカンマから重複なく読み取る", () => {
  assert.deepEqual(parseLearnerIDs("alice, bob\nteam-03"), ["alice", "bob", "team-03"]);
  assert.throws(() => parseLearnerIDs("alice alice"), /重複/);
  assert.throws(() => parseLearnerIDs("Alice"), /英小文字/);
  assert.throws(() => parseLearnerIDs("  "), /1件以上/);
});

test("コースの必須教材から完了条件を説明する", () => {
  assert.equal(
    completionDescription({ requiredLessons: [{ lessonId: "auth/hook", lessonVersion: "1.0.0" }] }),
    "必須教材1件の回答がすべて承認されること（auth/hook @ 1.0.0）",
  );
});

test("状態と受講者・コースを組み合わせて絞り込む", () => {
  assert.deepEqual(filterAssignments(assignments, "in_progress", "BOB"), [assignments[1]]);
  assert.deepEqual(filterAssignments(assignments, "", "review"), [assignments[2], assignments[3]]);
});

test("回答IDを原因・影響・修正・再発防止の観点へ対応付ける", () => {
  assert.deepEqual(answerPresentation("trace-boundary").facets, ["原因", "影響"]);
  assert.deepEqual(answerPresentation("compare-behavior").facets, ["影響", "修正"]);
  assert.deepEqual(answerPresentation("propose-prevention").facets, ["修正", "再発防止"]);
  assert.deepEqual(answerPresentation("custom-question").facets, ["確認課題"]);
});
