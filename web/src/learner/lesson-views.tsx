import type { StateProps } from "./lesson-data.ts";
import { useEffect, useRef } from "react";
import { L, PH } from "./lesson-data.ts";
import { LessonEnvironment } from "./lesson-environment.tsx";
import { LessonOutline } from "./lesson-outline.tsx";
import { lessonPhaseLeads, lessonPhases } from "./lesson-phases.tsx";

export function LessonDetail({
  state: S,
  onPhaseChange,
}: StateProps & { onPhaseChange: (phase: number) => void }) {
  const lesson = L.find((item) => item.id === "fb")!;
  const detail = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const container = detail.current;
    if (!container) return;
    let frame = 0;
    const updatePosition = () => {
      const bounds = container.getBoundingClientRect();
      const top = Math.max(0, bounds.top) + Math.min(window.innerHeight * 0.25, 160);
      let phase = -1;
      for (const section of container.querySelectorAll<HTMLElement>("[data-phase]")) {
        if (section.getBoundingClientRect().top <= top) phase = Number(section.dataset.phase);
      }
      onPhaseChange(phase);
    };
    const onScroll = () => {
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(updatePosition);
    };
    container.addEventListener("scroll", onScroll, { passive: true });
    window.addEventListener("scroll", onScroll, { passive: true });
    window.addEventListener("resize", onScroll);
    return () => {
      cancelAnimationFrame(frame);
      container.removeEventListener("scroll", onScroll);
      window.removeEventListener("scroll", onScroll);
      window.removeEventListener("resize", onScroll);
    };
  }, [onPhaseChange]);

  return (
    <div className="detail" ref={detail}>
      <div className="lesson-layout">
        <button className="back lesson-back" data-a="home">
          ← 教材一覧へ戻る
        </button>
        <LessonOutline state={S} />
        <div className="in">
          <h1 className="t">{lesson.title}</h1>
          <div className="facts" style={{ marginTop: "8px" }}>
            <div>
              <span>{"CVE"}</span>
              {lesson.cve}
            </div>
            <div>
              <span>{"製品"}</span>
              {lesson.product}
            </div>
            <div>
              <span>{"Labの出どころ"}</span>
              {lesson.src}
              {"（"}
              {lesson.sha}
              {"）"}
            </div>
          </div>
          <LessonEnvironment state={S} />
          <div className="pro">
            <h2>{"概要"}</h2>
          </div>
          <div className="prb">
            <p>
              {"File Browser 2.63.5 は、ブラウザからファイルを管理できるWebアプリケーションです。"}
            </p>
            <p>
              {lesson.cve}
              {
                " は、フック認証の設定でユーザー名やパスワードを外部コマンドへ渡す際、入力値がコマンドとして解釈されるコマンドインジェクションの脆弱性です。認証処理を通じて、意図しないコマンドが実行されるおそれがあります。"
              }
            </p>
            <p>
              {"この教材では脆弱版での挙動を観察し、原因と修正版での変化を確認します。"}
              <b>{"Labの画面は別タブで開きます。"}</b>
              {"実在のサイトには試さないでください。"}
            </p>
          </div>
          <div className="nav overview-actions">
            <button className="btn primary" data-a="begin">
              {S.revealed < 0 ? "学習を開始する ↓" : "Step 1を読み返す ↓"}
            </button>
          </div>
          <div className="lesson-sections">
            {lessonPhases.slice(0, S.revealed + 1).map((Phase, index) => (
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
                  <Phase lesson={lesson} state={S} />
                </div>
                <div className="nav">
                  <button className="btn" data-a="prev" data-i={index} disabled={index === 0}>
                    前の内容へ
                  </button>
                  <button
                    className="btn primary"
                    data-a="next"
                    data-i={index}
                    disabled={index === PH.length - 1 && S.done[index]}
                  >
                    {index === PH.length - 1
                      ? S.done[index]
                        ? "学習を完了しました"
                        : "完了する"
                      : `次へ：${PH[index + 1][0]} ↓`}
                  </button>
                </div>
              </section>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
