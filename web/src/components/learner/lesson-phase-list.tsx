import type { LearnerState, SampleLesson } from "../../learner/lesson-data.ts";
import { PH } from "../../learner/lesson-data.ts";
import { lessonPhaseLeads, lessonPhases } from "./lesson-phases.tsx";

interface LessonPhaseListProps {
  lesson: SampleLesson;
  state: LearnerState;
}

export function LessonPhaseList({ lesson, state }: LessonPhaseListProps) {
  return (
    <div className="lesson-sections">
      {lessonPhases.slice(0, state.revealed + 1).map((Phase, index) => (
        <section
          key={PH[index][0]}
          id={`phase-${index}`}
          data-phase={index}
          className="lesson-phase"
          aria-labelledby={`phase-title-${index}`}
        >
          <p className="phase-kicker">
            STEP {String(index + 1).padStart(2, "0")} / {PH.length}
          </p>
          <h2 className="ph" id={`phase-title-${index}`} tabIndex={-1}>
            {PH[index][0]}
          </h2>
          <p className="lead2">{lessonPhaseLeads[index]}</p>
          <div className="phase-body">
            <Phase lesson={lesson} state={state} />
          </div>
          <div className="nav">
            <button className="btn" data-a="prev" data-i={index} disabled={index === 0}>
              前の内容へ
            </button>
            <button
              className="btn primary"
              data-a="next"
              data-i={index}
              disabled={index === PH.length - 1 && state.done[index]}
            >
              {index === PH.length - 1
                ? state.done[index]
                  ? "学習を完了しました"
                  : "完了する"
                : `次へ：${PH[index + 1][0]} ↓`}
            </button>
          </div>
        </section>
      ))}
    </div>
  );
}
