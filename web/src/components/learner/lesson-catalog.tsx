import { Fragment } from "react";
import type { StateProps } from "../../learner/lesson-data.ts";
import { L, PH } from "../../learner/lesson-data.ts";

export function LessonList({ state: S }: StateProps) {
  const cats = [
    ["all", "すべて", L.length],
    ["コマンドインジェクション", "コマンドインジェクション", 1],
    ["パストラバーサル", "パストラバーサル", 1],
  ];
  const items = L.filter((l) => S.cat === "all" || l.cat === S.cat);
  return (
    <>
      <div className="list">
        <aside className="cats">
          <h2>{"分野"}</h2>
          {cats.map((c) => (
            <Fragment key={c[0]}>
              <button className="cat" aria-pressed={S.cat === c[0]} data-a="cat" data-v={c[0]}>
                <span>{c[1]}</span>
                <span>{c[2]}</span>
              </button>
            </Fragment>
          ))}
          <nav className="sidebar-notes" aria-label="学習の振り返り">
            <button className="cat" data-a="notes">
              {"まとめノート →"}
            </button>
          </nav>
          {"\n             "}
          <p className="note2">
            {
              "教材は、自分のPC上のLabにだけ試します。実在のサイトや他人のサーバーには使わないでください。"
            }
          </p>
        </aside>
        {"\n             "}
        <section className="cards">
          <h1>{"教材を選ぶ"}</h1>
          <p className="lead">{"脆弱性や製品の概要を見て、教材を選びます。"}</p>
          {"\n             "}
          <div className="grid">
            {items.map((l) => {
              const n = l.ready ? Object.keys(S.done).length : 0,
                deg = (n / PH.length) * 360;
              return (
                <Fragment key={l.id}>
                  <button className="lcard" data-a="open" data-v={l.id} disabled={!l.ready}>
                    {"\n             "}
                    <div>
                      <small className="mono">{l.cve}</small>
                      <b>{l.title}</b>
                      <div
                        style={{ margin: "8px 0 0", display: "flex", gap: "6px", flexWrap: "wrap" }}
                      >
                        <span className={"pill " + (l.src === "fork追加Lab" ? "amber" : "")}>
                          {l.src}
                        </span>
                        {l.ready ? (
                          n === PH.length ? (
                            <>
                              <span className="pill green">{"Completed"}</span>
                            </>
                          ) : (
                            ""
                          )
                        ) : (
                          <>
                            <span className="pill">{"準備中"}</span>
                          </>
                        )}
                      </div>
                    </div>
                    {"\n             "}
                    <div
                      className="ring"
                      style={{
                        background: "conic-gradient(var(--teal) " + deg + "deg,var(--line) 0)",
                      }}
                    >
                      <div>
                        <span>
                          {n}
                          <em>{`/ ${PH.length}`}</em>
                        </span>
                      </div>
                    </div>
                  </button>
                </Fragment>
              );
            })}
          </div>
        </section>
      </div>
    </>
  );
}
