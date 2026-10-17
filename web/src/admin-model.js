export const STATUS_META = Object.freeze({
  assigned: { label: "未着手", tone: "neutral" },
  in_progress: { label: "受講中", tone: "info" },
  submitted: { label: "確認待ち", tone: "warning" },
  completed: { label: "完了", tone: "success" },
  needs_retry: { label: "要再受講", tone: "danger" },
});

const ANSWER_META = Object.freeze({
  "trace-boundary": {
    label: "原因と信頼境界",
    facets: ["原因", "影響"],
  },
  "compare-behavior": {
    label: "影響と修正前後の挙動",
    facets: ["影響", "修正"],
  },
  "propose-prevention": {
    label: "修正方針と再発防止",
    facets: ["修正", "再発防止"],
  },
});

export function assignmentSummary(assignments, now = new Date()) {
  const counts = Object.fromEntries(Object.keys(STATUS_META).map((status) => [status, 0]));
  let overdue = 0;
  for (const assignment of assignments) {
    if (Object.hasOwn(counts, assignment.status)) counts[assignment.status]++;
    if (
      assignment.status !== "completed" &&
      Number.isFinite(Date.parse(assignment.dueAt)) &&
      new Date(assignment.dueAt) < now
    )
      overdue++;
  }
  return {
    total: assignments.length,
    counts,
    overdue,
    completionRate: assignments.length
      ? Math.round((counts.completed / assignments.length) * 100)
      : 0,
  };
}

export function parseLearnerIDs(value) {
  const learners = value
    .split(/[\s,]+/)
    .map((item) => item.trim())
    .filter(Boolean);
  if (learners.length === 0) throw new Error("対象者IDを1件以上入力してください。");
  if (new Set(learners).size !== learners.length) throw new Error("対象者IDが重複しています。");
  if (learners.some((learner) => !/^[a-z0-9][a-z0-9._-]{0,62}$/.test(learner)))
    throw new Error(
      "対象者IDは英小文字、数字、ピリオド、ハイフン、アンダースコアで入力してください。",
    );
  return learners;
}

export function completionDescription(course) {
  const required = course?.requiredLessons ?? [];
  if (required.length === 0) return "完了条件を取得できません";
  const lessonLabel = required
    .map((lesson) => `${lesson.lessonId} @ ${lesson.lessonVersion}`)
    .join("、");
  return `必須教材${required.length}件の回答がすべて承認されること（${lessonLabel}）`;
}

export function filterAssignments(assignments, status, query) {
  const normalizedQuery = query.trim().toLocaleLowerCase("ja");
  return assignments.filter((assignment) => {
    if (status && assignment.status !== status) return false;
    if (!normalizedQuery) return true;
    return [assignment.learnerId, assignment.courseId, assignment.courseVersion]
      .join(" ")
      .toLocaleLowerCase("ja")
      .includes(normalizedQuery);
  });
}

export function answerPresentation(questionID) {
  return (
    ANSWER_META[questionID] ?? {
      label: questionID,
      facets: ["確認課題"],
    }
  );
}
