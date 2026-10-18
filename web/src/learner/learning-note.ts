export const NOTE_QUESTIONS = [
  {
    id: "impact",
    label: "影響",
    question: "どんなことが起きる脆弱性でしたか？",
    hint: "誰が、何をできてしまうのかを1〜2文で書いてみましょう。",
  },
  {
    id: "cause",
    label: "原因",
    question: "なぜ、その脆弱性が起きたのでしょうか？",
    hint: "入力がどの処理に渡り、どこで扱いを誤ったのかを思い出しましょう。",
  },
  {
    id: "remediation",
    label: "修正",
    question: "どのように修正すると防げますか？",
    hint: "修正前と修正後で、入力の扱いがどう変わったかを書きましょう。",
  },
  {
    id: "prevention",
    label: "再発防止",
    question: "自分の開発で、何を確認・工夫しますか？",
    hint: "実装、コードレビュー、テストで実践したいことを一つ挙げましょう。",
  },
] as const;

export const NOTE_ANSWER_LIMIT = 600;
export type NoteAnswerID = (typeof NOTE_QUESTIONS)[number]["id"];
export type NoteAnswers = Record<NoteAnswerID, string>;
export interface NoteLesson {
  lessonId: string;
  lessonVersion: string;
}
export interface LearningNote extends NoteLesson {
  draft: NoteAnswers;
  step: number;
  saved: { answers: NoteAnswers; updatedAt: string } | null;
}
type NoteStorage = Pick<Storage, "getItem" | "setItem">;

export function emptyNote(lesson: NoteLesson): LearningNote {
  return {
    ...lesson,
    draft: { impact: "", cause: "", remediation: "", prevention: "" },
    step: 0,
    saved: null,
  };
}

export function noteStorageKey(lesson: NoteLesson) {
  return `reprokit:learning-note:v1:${lesson.lessonId}:${lesson.lessonVersion}`;
}

function validAnswers(value: unknown): value is NoteAnswers {
  if (!value || typeof value !== "object") return false;
  return NOTE_QUESTIONS.every(({ id }) => {
    const answer = (value as Record<string, unknown>)[id];
    return typeof answer === "string" && answer.length <= NOTE_ANSWER_LIMIT;
  });
}

export function answeredCount(answers: NoteAnswers) {
  return NOTE_QUESTIONS.filter(({ id }) => answers[id].trim()).length;
}

export function readNote(lesson: NoteLesson, storage?: NoteStorage) {
  const fallback = emptyNote(lesson);
  try {
    const raw = (storage ?? window.localStorage).getItem(noteStorageKey(lesson));
    if (raw === null) return { note: fallback, failure: "" };
    const value: unknown = JSON.parse(raw);
    if (!value || typeof value !== "object") throw new Error("invalid note");
    const note = value as LearningNote;
    if (
      note.lessonId !== lesson.lessonId ||
      note.lessonVersion !== lesson.lessonVersion ||
      !validAnswers(note.draft) ||
      !Number.isInteger(note.step) ||
      note.step < 0 ||
      note.step >= NOTE_QUESTIONS.length ||
      (note.saved !== null &&
        (!note.saved ||
          !validAnswers(note.saved.answers) ||
          answeredCount(note.saved.answers) !== NOTE_QUESTIONS.length ||
          typeof note.saved.updatedAt !== "string" ||
          !Number.isFinite(Date.parse(note.saved.updatedAt))))
    )
      throw new Error("invalid note");
    return {
      note: {
        ...lesson,
        draft: { ...note.draft },
        step: note.step,
        saved: note.saved
          ? { answers: { ...note.saved.answers }, updatedAt: note.saved.updatedAt }
          : null,
      },
      failure: "",
    };
  } catch {
    return {
      note: fallback,
      failure: "保存したノートを読み込めませんでした。ブラウザの保存設定を確認してください。",
    };
  }
}

export function writeNote(note: LearningNote, storage?: NoteStorage): boolean {
  try {
    (storage ?? window.localStorage).setItem(noteStorageKey(note), JSON.stringify(note));
    return true;
  } catch {
    return false;
  }
}

export function finishNote(note: LearningNote, now = new Date()): LearningNote | null {
  if (answeredCount(note.draft) !== NOTE_QUESTIONS.length) return null;
  const answers = Object.fromEntries(
    NOTE_QUESTIONS.map(({ id }) => [id, note.draft[id].trim()]),
  ) as NoteAnswers;
  return { ...note, saved: { answers, updatedAt: now.toISOString() } };
}
