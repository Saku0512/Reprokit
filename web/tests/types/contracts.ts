import type { StudyApi } from "../../src/shared/study-api.ts";
import type {
  Assignment,
  LabSession,
  LearnerLesson,
  MarkerObservation,
} from "../../src/shared/types.ts";
import type { LearnerState } from "../../src/learner/lesson-data.ts";

// 実行せずに、APIの戻り値と許可済み入力が型検査で守られることを確かめる。
export function checkContracts(api: StudyApi, state: LearnerState) {
  const prepared: Promise<LabSession> = api.prepare("1.0.0", "vulnerable");
  const lesson: Promise<LearnerLesson> = api.detail("1.0.0");
  const observation: Promise<MarkerObservation> = api.marker("session-id", "fixed");
  const assignment: Promise<Assignment> = api.startTraining();

  // @ts-expect-error 未定義のLab参照は入力できない。
  api.prepare("1.0.0", "arbitrary-target");
  // @ts-expect-error 未定義のVolume方針は入力できない。
  api.delete("session-id", "all");
  // @ts-expect-error マーカー観察はLab Sessionの応答ではない。
  const session: Promise<LabSession> = api.marker("session-id", "fixed");
  // @ts-expect-error 画面状態へ任意の稼働状態は設定できない。
  state.lab = "unknown";

  return { prepared, lesson, observation, assignment, session };
}
