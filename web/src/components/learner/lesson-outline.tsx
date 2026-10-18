import type { StateProps } from "../../learner/lesson-data.ts";
import { PH } from "../../learner/lesson-data.ts";

type LessonOutlineProps = StateProps & { id?: string };

export function LessonOutline({ state: S, id = "lesson-outline" }: LessonOutlineProps) {
  return (
    <nav id={id} className="lesson-outline" aria-label="学習の流れ">
      <p className="outline-title">学習の流れ</p>
      <p className="outline-progress">
        {Object.keys(S.done).length} / {PH.length} ステップ完了
      </p>
      <ol>
        {PH.map((phase, i) => (
          <li key={phase[0]}>
            <button
              className={`outline-step ${S.done[i] ? "done" : ""}`}
              data-a="go"
              data-i={i}
              disabled={i > S.revealed}
              aria-current={i === S.p ? "step" : undefined}
              aria-controls={i <= S.revealed ? `phase-${i}` : undefined}
            >
              <span className="step-number" aria-hidden="true">
                {S.done[i] ? "✓" : i + 1}
              </span>
              <span>
                <b>{phase[0]}</b>
                <small>{phase[1]}</small>
              </span>
            </button>
          </li>
        ))}
      </ol>
    </nav>
  );
}
