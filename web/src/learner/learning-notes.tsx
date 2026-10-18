import { useEffect, useRef } from "react";
import type { LearningNote, NoteAnswerID } from "./learning-note.ts";
import type { SampleLesson } from "./lesson-data.ts";
import { NOTE_QUESTIONS, NOTE_ANSWER_LIMIT, answeredCount } from "./learning-note.ts";

export function NoteEditor({
  lesson,
  note,
  status,
  failure,
  onAnswer,
  onStep,
  onSave,
  onRetry,
  onBack,
}: {
  lesson: SampleLesson;
  note: LearningNote;
  status: string;
  failure: string;
  onAnswer: (id: NoteAnswerID, answer: string) => void;
  onStep: (step: number) => void;
  onSave: () => void;
  onRetry: () => void;
  onBack: () => void;
}) {
  const headings = useRef<(HTMLHeadingElement | null)[]>([]);
  const initialStep = useRef(note.step);
  useEffect(() => {
    const heading = headings.current[initialStep.current];
    heading?.focus({ preventScroll: true });
    if (initialStep.current > 0)
      heading?.closest(".note-question")?.scrollIntoView({ block: "start" });
  }, []);
  function moveToQuestion(step: number) {
    onStep(step);
    const heading = headings.current[step];
    heading?.focus({ preventScroll: true });
    heading?.closest(".note-question")?.scrollIntoView({
      block: "start",
      behavior: window.matchMedia("(prefers-reduced-motion: reduce)").matches
        ? "instant"
        : "smooth",
    });
  }
  const count = answeredCount(note.draft);
  return (
    <main className="notes-page">
      <div className="notes-inner">
        <button className="back" onClick={onBack}>
          ← まとめノートへ戻る
        </button>
        <h1>学んだことをまとめる</h1>
        <p className="notes-lead">{lesson.sub}</p>
        <p className="notes-storage">
          回答はこのブラウザに保存します。別の端末との共有や研修担当者への提出は行いません。ブラウザのデータを消すとノートも消えます。
        </p>
        <ol className="note-steps" aria-label="まとめの質問">
          {NOTE_QUESTIONS.map((item, i) => (
            <li key={item.id}>
              <button
                className="btn"
                aria-current={i === note.step ? "step" : undefined}
                onClick={() => moveToQuestion(i)}
              >
                {note.draft[item.id].trim() ? "✓ " : `${i + 1}. `}
                {item.label}
              </button>
            </li>
          ))}
        </ol>
        <form
          className="note-form"
          onSubmit={(event) => {
            event.preventDefault();
            onSave();
          }}
        >
          {NOTE_QUESTIONS.map((question, i) => (
            <section
              className="note-question card"
              key={question.id}
              aria-labelledby={`note-title-${question.id}`}
            >
              <p className="phase-kicker">
                質問 {i + 1} / {NOTE_QUESTIONS.length}
              </p>
              <h2
                id={`note-title-${question.id}`}
                ref={(element) => {
                  headings.current[i] = element;
                }}
                tabIndex={-1}
              >
                {question.question}
              </h2>
              <label className="note-answer-label" htmlFor={`note-${question.id}`}>
                {question.label}についてのまとめ
              </label>
              <p id={`note-hint-${question.id}`} className="notes-lead">
                {question.hint}
              </p>
              <textarea
                id={`note-${question.id}`}
                value={note.draft[question.id]}
                maxLength={NOTE_ANSWER_LIMIT}
                rows={6}
                aria-describedby={`note-hint-${question.id} note-length-${question.id}`}
                onFocus={() => {
                  if (note.step !== i) onStep(i);
                }}
                onChange={(event) => onAnswer(question.id, event.target.value)}
                placeholder="自分の言葉で、短く書いてみましょう。"
              />
              <p id={`note-length-${question.id}`} className="note-length">
                {note.draft[question.id].length} / {NOTE_ANSWER_LIMIT}文字
              </p>
              <div className="note-actions">
                <button
                  type="button"
                  className="btn"
                  disabled={i === 0}
                  onClick={() => moveToQuestion(i - 1)}
                >
                  前の質問へ ↑
                </button>
                {i < NOTE_QUESTIONS.length - 1 && (
                  <button
                    type="button"
                    className="btn primary"
                    onClick={() => moveToQuestion(i + 1)}
                  >
                    次の質問へ ↓
                  </button>
                )}
              </div>
            </section>
          ))}
          <div className="note-save card">
            <p className="phase-kicker">
              {count} / {NOTE_QUESTIONS.length}問回答済み
            </p>
            <p className="notes-storage" role="status">
              {status}
            </p>
            {failure && (
              <div>
                <p className="note-failure" role="alert">
                  {failure}
                </p>
                <button type="button" className="btn" onClick={onRetry}>
                  下書きの保存を再試行する
                </button>
              </div>
            )}
            <button
              type="submit"
              className="btn primary"
              disabled={count !== NOTE_QUESTIONS.length}
            >
              カードに保存する
            </button>
            {count < NOTE_QUESTIONS.length && (
              <p className="notes-storage">
                未回答の質問があります。上にスクロールして回答できます。
              </p>
            )}
          </div>
        </form>
      </div>
    </main>
  );
}

export function NoteLibrary({
  lesson,
  note,
  failure,
  onEdit,
  onRead,
}: {
  lesson: SampleLesson;
  note: LearningNote;
  failure: string;
  onEdit: () => void;
  onRead: () => void;
}) {
  const count = answeredCount(note.draft);
  const heading = useRef<HTMLHeadingElement>(null);
  useEffect(() => {
    heading.current?.focus();
  }, []);
  return (
    <main className="notes-page">
      <div className="notes-layout">
        <button className="back lesson-back" data-a="home">
          ← 教材一覧へ戻る
        </button>
        <div className="notes-inner">
          <h1 ref={heading} tabIndex={-1}>
            まとめノート
          </h1>
          <p className="notes-lead">学んだ脆弱性を、自分の言葉で振り返る。</p>
          <p className="notes-storage">
            このブラウザに保存したノートです。ブラウザのデータを消すとノートも消えます。
          </p>
          {failure && (
            <p className="note-failure" role="alert">
              {failure}
            </p>
          )}
          {note.saved ? (
            <article className="note-card card" aria-labelledby="note-card-title">
              <div className="note-card-meta">
                <span className="pill">{lesson.cat}</span>
                <time dateTime={note.saved.updatedAt}>
                  {new Date(note.saved.updatedAt).toLocaleDateString("ja-JP")} 更新
                </time>
              </div>
              <h2 id="note-card-title">{lesson.sub}</h2>
              <p className="notes-storage">
                {lesson.product} · {lesson.cve} · 教材 v{note.lessonVersion}
              </p>
              <p className="note-card-preview">{note.saved.answers.impact}</p>
              <details className="note-details">
                <summary>4つの回答を読む</summary>
                <dl className="note-summary">
                  {NOTE_QUESTIONS.map(({ id, label }) => (
                    <div key={id}>
                      <dt>{label}</dt>
                      <dd>{note.saved!.answers[id]}</dd>
                    </div>
                  ))}
                </dl>
              </details>
              <div className="note-actions">
                <button className="btn" onClick={onRead}>
                  教材を読み返す
                </button>
                <button className="btn primary" onClick={onEdit}>
                  まとめを編集する
                </button>
              </div>
            </article>
          ) : (
            <section className="note-card card">
              <h2>{count ? "書きかけのまとめがあります" : "まだまとめノートがありません"}</h2>
              <p>
                {count
                  ? `${lesson.sub} · ${count} / ${NOTE_QUESTIONS.length}問回答済み`
                  : "教材を学んだら、4つの質問に答えてカードを作りましょう。"}
              </p>
              <div className="note-actions">
                <button className="btn" onClick={onRead}>
                  教材を読む
                </button>
                {count > 0 && (
                  <button className="btn primary" onClick={onEdit}>
                    下書きを再開する
                  </button>
                )}
              </div>
            </section>
          )}
        </div>
      </div>
    </main>
  );
}
