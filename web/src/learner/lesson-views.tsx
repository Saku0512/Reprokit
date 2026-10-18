import type { PhaseProps, StateProps } from "./lesson-data.ts";
import { Fragment, useEffect, useRef } from "react";
import { L, PH, NODES, TRACE_DETAILS } from "./lesson-data.ts";
import { endpointURL } from "../shared/study-api.ts";
import { PatchWorkshop } from "./patch-workshop.tsx";
import { patchComparisonLines } from "./patch-exercise.ts";
import { BehaviorComparison } from "./behavior-comparison.tsx";
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

function LabBox({ state: S }: StateProps) {
  const s = S.lab;
  const endpoints = (S.runtime?.endpoints ?? [])
    .map((endpoint) => ({ ...endpoint, url: endpointURL(endpoint) }))
    .filter((endpoint) => endpoint.url);
  const endpointTitle = (service: string) =>
    service === "vulnerable" ? "脆弱版" : service === "fixed" ? "修正版" : "Lab";
  let r1, r2;
  if (s === "ready") {
    r1 = (
      <>
        <button className="btn primary" data-a="deploy">
          {"▶ DEPLOY"}
        </button>
      </>
    );
    r2 = (
      <>
        <span className="st">{"READY"}</span>
        <span className="sep"></span>
        <span>{"IP：—"}</span>
      </>
    );
  } else if (s === "deploying") {
    const st = ["固定コミットを取得", "Composeを検査", "イメージdigestを確認", "起動を確認"][
      Math.min(3, Math.floor(S.seg / 5))
    ];
    r1 = (
      <>
        <button className="btn" disabled>
          {"処理中…"}
        </button>
      </>
    );
    r2 = (
      <>
        <span className="st dep">{"DEPLOYING"}</span>
        <span className="sep"></span>
        <span>
          {st}
          {"…"}
        </span>
        <span className="sep"></span>
        <span className="seg">
          {Array.from({ length: 20 }, (_, i) => (
            <Fragment key={i}>
              <i className={i < S.seg ? "on" : ""}></i>
            </Fragment>
          ))}
        </span>
      </>
    );
  } else if (s === "running") {
    r1 = (
      <>
        <button className="btn danger" data-a="stop">
          {"■ STOP"}
        </button>
        <button className="btn danger" data-a="destroy">
          {"🗑 DESTROY"}
        </button>
      </>
    );
    r2 = (
      <>
        <span className="st run">{"RUNNING"}</span>
        <span className="sep"></span>
        {endpoints.map((endpoint, i) => (
          <Fragment key={`${endpoint.service}:${endpoint.port}`}>
            {i > 0 && <span className="sep" />}
            <span>
              {endpointTitle(endpoint.service)}
              {"："}
              <span className="mono">{endpoint.url}</span>{" "}
              <button className="btn sm" data-a="copy" data-v={endpoint.url}>
                {"コピー"}
              </button>{" "}
              <a
                className="btn sm"
                href={endpoint.url ?? undefined}
                target="_blank"
                rel="noopener noreferrer"
              >
                {"別タブで開く ↗"}
              </a>
            </span>
          </Fragment>
        ))}
        {!endpoints.length && <span>{"安全な接続先を確認できませんでした。"}</span>}
      </>
    );
  } else if (s === "failed") {
    r1 = (
      <>
        <button className="btn danger" data-a="destroy" disabled={!S.runtime?.labSessionId}>
          {"🗑 DESTROY"}
        </button>
      </>
    );
    r2 = (
      <>
        <span className="st stp">{"FAILED"}</span>
        <span className="sep" />
        <span>{"状態を確認して削除してください"}</span>
      </>
    );
  } else {
    r1 = (
      <>
        <button className="btn primary" data-a="deploy">
          {"▶ DEPLOY"}
        </button>
        <button className="btn danger" data-a="destroy">
          {"🗑 DESTROY"}
        </button>
      </>
    );
    r2 = (
      <>
        <span className="st stp">{"STOPPED"}</span>
        <span className="sep"></span>
        <span>{"IP：—"}</span>
      </>
    );
  }
  return (
    <>
      <div className="tm">
        <div className="r1">
          <b>{"演習環境"}</b>
          <div className="act" style={{ margin: "0" }}>
            {r1}
          </div>
        </div>
        <div className="r2">{r2}</div>
        {S.labFailure && <p className="lab-failure">{S.labFailure}</p>}
      </div>
    </>
  );
}

export function LabConsole({
  state: S,
  idPrefix = "console",
  showLead = true,
}: StateProps & { idPrefix?: string; showLead?: boolean }) {
  const activeService = "vulnerable";
  const logs = S.logs?.[activeService];
  const action = S.actions?.[`${activeService}:list_tmp`];
  const panelID = `${idPrefix}-${activeService}-panel`;
  return (
    <div className="console-content">
      {showLead && (
        <p className="console-lead">
          {
            "実行中のLabだけを観察できます。自由なOSコマンドは実行できず、教材レビュー済みのアクションだけが利用できます。"
          }
        </p>
      )}
      <section id={panelID} className="console-service" aria-label="脆弱コンテナ">
        <div className="console-section">
          <div className="console-section-heading">
            <div>
              <b>{"コンテナログ"}</b>
              <p>{"最大200行・64 KiBのスナップショットです。"}</p>
            </div>
            <button
              className="btn sm"
              data-a="logs"
              data-v={activeService}
              disabled={S.lab !== "running" || S.logLoading !== null}
            >
              {S.logLoading === activeService ? "取得中…" : "ログを更新"}
            </button>
          </div>
          {logs ? (
            <>
              {logs.truncated && <span className="pill amber">{"先頭を省略"}</span>}
              <pre className="log">{logs.content || "（出力はまだありません）"}</pre>
            </>
          ) : (
            <p className="console-empty">{"まだログを取得していません。"}</p>
          )}
        </div>
        <div className="console-section">
          <div className="console-section-heading">
            <div>
              <b>{"検証コンソール"}</b>
              <p>
                <code>{"ls -la /tmp"}</code>
                {" — マーカー作成を確認する固定アクション"}
              </p>
            </div>
            <button
              className="btn sm"
              data-a="list-tmp"
              data-v={activeService}
              disabled={S.lab !== "running" || S.actionLoading !== null}
            >
              {S.actionLoading === `${activeService}:list_tmp` ? "実行中…" : "実行"}
            </button>
          </div>
          {action ? (
            <>
              {action.truncated && <span className="pill amber">{"先頭を省略"}</span>}
              <pre className="log">{action.output || "（出力はありません）"}</pre>
            </>
          ) : (
            <p className="console-empty">{"まだ検証アクションを実行していません。"}</p>
          )}
        </div>
      </section>
      {(S.logFailure || S.actionFailure) && (
        <p className="lab-failure" role="alert">
          {S.logFailure || S.actionFailure}
        </p>
      )}
      <p className="console-notice">
        {"ログや出力にはLabへ入力した値が含まれる場合があります。共有しないでください。"}
      </p>
    </div>
  );
}

function NormalUsage() {
  return (
    <>
      <div className="stage">
        <div>
          <div className="card">
            <h3>{"やってみること"}</h3>
            <ol className="step">
              <li>{"演習環境を起動する"}</li>
              <li>{"「別タブで開く」で、脆弱版または修正版のログイン画面を開く"}</li>
              <li>{"任意のユーザー名・パスワードでログインを試す"}</li>
              <li>{"どちらの画面でもログインが拒否されることを確認する"}</li>
              <li>{"戻ってきて、拒否される前のサーバー側の処理を確認する"}</li>
            </ol>
            {"\n            "}
            <div className="note">
              {
                "このLabには有効な認証情報を用意していません。脆弱版・修正版ともログインが拒否されることが、演習では正常な動作です。"
              }
            </div>
          </div>
        </div>
        {"\n            "}
        <div className="card">
          <h3>{"ログインが拒否されるまでの処理"}</h3>
          <div className="log">
            <span className="ok">{"1 受信"}</span>
            {" POST /api/login\n"}
            <span className="ok">{"2 フック認証"}</span>
            {" 設定済みコマンドへ資格情報を渡す\n"}
            <span className="t">{"3 判定"}</span>
            {" hook.action=block → 拒否\n"}
            <span className="t">{"4 応答"}</span>
            {" 403 Forbidden"}
          </div>
          {"\n            "}
          <div className="reveal">
            {
              "両版とも最終的にはログインを拒否します。次は、拒否される前に入力がどのように扱われるかを確かめます。"
            }
          </div>
        </div>
      </div>
    </>
  );
}

function Reproduction({ lesson, state: S }: PhaseProps) {
  const affectedData = lesson.affectedData ?? [];
  const h = [
    "（ヒントなし）",
    "このCVEは「認証の結果が出る前」に何かが起きます。ログインの入力欄に注目してください。",
    "入力欄の値が、サーバー側でどこに渡されるかを、前のフェーズのログで確認してください。",
    "教材で用意した検証用マーカー入力（無害）を、レビュー済みデータとして表示します。",
  ];
  return (
    <>
      <div className="stage">
        <div>
          <div className="card">
            <h3>{"ガイド"}</h3>
            <ol className="step">
              <li>{"上のLabカードから脆弱版を別タブで開く"}</li>
              <li>{"次の値をログインフォームへ入力して、ログインを試す"}</li>
              <li>{"ログインが拒否された後、「観察結果を取得」を押す"}</li>
            </ol>
            {"\n            "}
            <div className="log" style={{ marginTop: "10px" }}>
              {"Username: id > /tmp/success; echo hook.action=block\n"}
              {"Password: anything"}
            </div>
            <p className="console-empty">
              {
                "この入力は隔離コンテナ内の /tmp/success にマーカーを作るだけです。認証失敗は想定どおりの結果です。"
              }
            </p>
            {"\n            "}
            {S.hint ? (
              <>
                <div className="reveal ng">
                  <b>
                    {"ヒント"}
                    {S.hint}
                  </b>
                  <br />
                  {h[S.hint]}
                </div>
              </>
            ) : (
              ""
            )}
            {"\n            "}
            <div className="act">
              <button className="btn" data-a="hint" disabled={S.hint >= 3 ? true : false}>
                {"ヒントを見る"}
              </button>
              <button
                className="btn primary"
                data-a="repro"
                disabled={S.lab === "running" ? false : true}
              >
                {"観察結果を取得"}
              </button>
              <button className="btn" data-a="rst">
                {"やり直す"}
              </button>
            </div>
            {"\n            "}
            {S.lab === "running" ? (
              ""
            ) : (
              <>
                <p style={{ fontSize: "13px", color: "var(--sub)", margin: "8px 0 0" }}>
                  {"上のDEPLOYでLabを起動すると押せます。"}
                </p>
              </>
            )}
          </div>
        </div>
        {"\n            "}
        <div>
          <div className="card">
            <h3>{"観察結果（隔離コンテナ内）"}</h3>
            <LabConsole state={S} idPrefix="observation" showLead={false} />
          </div>
          {"\n            "}
          <div className="card" style={{ marginTop: "14px" }}>
            <h3>{"影響を受けるデータ"}</h3>
            {affectedData.length === 0 ? (
              <p className="console-empty">{"-"}</p>
            ) : S.repro ? (
              <>
                <table className="data">
                  <tbody>
                    <tr>
                      <th>{"種類"}</th>
                      <th>{"内容"}</th>
                    </tr>
                    {affectedData.map((item, index) => (
                      <tr key={index}>
                        <td>{item.type}</td>
                        <td>{item.content}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </>
            ) : (
              <>
                <p style={{ color: "var(--sub)", fontSize: "13.5px", margin: "0" }}>
                  {"再現すると、到達可能になるデータの例がここに並びます。"}
                </p>
              </>
            )}
          </div>
        </div>
      </div>
    </>
  );
}

function RequestTrace({ state: S }: StateProps) {
  const detail = TRACE_DETAILS[S.pos];
  return (
    <>
      <div className="stage">
        <div className="card">
          <h3>{"リクエストの流れ（結果から戻る）"}</h3>
          <div className="tl">
            {NODES.map((n, i) => (
              <Fragment key={i}>
                <button
                  className={
                    "nd " +
                    (i <= S.pos ? "vis" : "") +
                    " " +
                    (i === S.pos ? "at" : "") +
                    " " +
                    (S.pick === i ? "pick" : "")
                  }
                  data-a="pick"
                  data-i={i}
                >
                  <i>{i + 1}</i>
                  <span>
                    <b style={{ fontSize: "14px" }}>{n[0]}</b>
                    <small>
                      {n[1]}
                      {"："}
                      {n[2]}
                    </small>
                  </span>
                  {i <= S.pos ? (
                    <>
                      <span
                        className={"pill " + (n[3] === "u" ? "red" : n[3] === "t" ? "amber" : "")}
                      >
                        {n[4] || "—"}
                      </span>
                    </>
                  ) : (
                    ""
                  )}
                </button>
              </Fragment>
            ))}
          </div>
          {"\n            "}
          <div className="act">
            <button className="btn" data-a="back" disabled={S.pos <= 0}>
              {"1つ戻る"}
            </button>
            <button className="btn" data-a="fwd" disabled={S.pos >= 5}>
              {"1つ進める"}
            </button>
          </div>
        </div>
        {"\n            "}
        <section className="card trace-detail" aria-label="この地点の様子">
          <h3>{"この地点の様子"}</h3>
          <h4>{`${S.pos + 1}. ${NODES[S.pos][0]}`}</h4>
          <p>{detail.description}</p>
          <p className="console-empty">{detail.codeLabel}</p>
          <pre className="log">
            <code>{detail.code}</code>
          </pre>
          <p>{detail.result}</p>
          <a
            href={`https://github.com/filebrowser/filebrowser/blob/v2.63.5/auth/hook.go#L${detail.line}`}
            target="_blank"
            rel="noreferrer"
          >
            {"File Browser 2.63.5の実装を見る"}
          </a>
        </section>
      </div>
    </>
  );
}

function PatchComparison() {
  return (
    <>
      <div className="stage">
        <div className="card">
          <h3>{"修正差分（演習と同じ抜粋）2.63.5 → 2.63.6"}</h3>
          {"\n            "}
          <p style={{ margin: "0 0 10px", fontSize: "14px" }}>
            {
              "赤い「−」の行が削除された処理です。この抜粋には追加行はありません。記号のない行は修正前からある処理です。"
            }
          </p>
          {"\n            "}
          <div className="diff" aria-label="演習コードの修正差分">
            {patchComparisonLines.map((line, index) => (
              <div
                key={index}
                className={line.removed ? "del" : undefined}
                data-change={line.removed ? "removed" : "context"}
              >
                {`${line.removed ? "- " : "  "}${line.code}`}
              </div>
            ))}
          </div>
          {"\n            "}
          <p style={{ fontSize: "12.5px", color: "var(--sub)", margin: "8px 0 0" }}>
            {"前章と同じ抜粋を使っています。関数全体の差分と回帰テストは"}
            <a
              href={
                "https://github.com/filebrowser/filebrowser/commit/34ae34e764d72540c039f1f5ea2ec4c974168c1f"
              }
              target="_blank"
              rel="noopener"
            >
              {"公式修正コミット"}
            </a>
            {"で確認してください。"}
          </p>
        </div>
        {"\n            "}
        <div>
          <div className="card">
            <h3>{"原因から修正まで"}</h3>
            {"\n            "}
            <p style={{ margin: "0 0 8px", fontSize: "14px" }}>
              <b>{"見つけた原因（フェーズ3・手順4）"}</b>
              <br />
              <code>{"os.Expand"}</code>
              {"がユーザー名とパスワードをコマンド引数へ展開し、"}
              <code>{"exec.Command"}</code>
              {
                "へ渡していました。このLabでは管理者の設定がシェルを呼ぶため、入力値がシェルの命令として解釈されます。"
              }
            </p>
            {"\n            "}
            <p style={{ margin: "0", fontSize: "14px" }}>
              <b>{"修正版で変わったこと"}</b>
              <br />
              {"引数への展開ループと、そのための"}
              <code>{"envMapping"}</code>
              {
                "を削除しました。exec.Commandの呼び出しと、資格情報をcmd.Envへ設定する2行は修正前からあり、そのまま残っています。環境変数を渡す処理を新設したわけではありません。"
              }
            </p>
          </div>
        </div>
      </div>
    </>
  );
}

function Remediation({ state: S }: StateProps) {
  return (
    <>
      <BehaviorComparison state={S} />
      <div className="card" style={{ marginTop: "14px" }}>
        <h3>{"実運用での対策"}</h3>
        <p style={{ margin: "0", fontSize: "14px" }}>
          {"根本対策は"}
          <b>{"修正版へ更新"}</b>
          {
            "すること。更新できない間の緩和策は、フック認証でシェルを経由しない設定にし、管理画面を信頼できるネットワークに限定することです。"
          }
        </p>
      </div>
    </>
  );
}

const phases = [
  NormalUsage,
  Reproduction,
  RequestTrace,
  PatchWorkshop,
  PatchComparison,
  Remediation,
];
const phaseLeads = [
  "このLabではログインが拒否されることが正常な動作です。拒否される前の処理を確認します。",
  "ガイドに沿って検証し、観察結果を確認します。",
  "結果から処理をたどり、入力値がコマンドとして扱われる流れを確認します。",
  "公式の修正差分を見る前に、原因となった処理を探してコードを編集します。",
  "実際の修正差分と、自分が見つけた原因との対応を確認します。",
  "修正版で同じ操作をもう一度行い、両版のログイン拒否と副作用の違いを確認します。",
];

export function LessonOutline({ state: S, id = "lesson-outline" }: StateProps & { id?: string }) {
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

export function LessonDetail({
  state: S,
  onPhaseChange,
}: StateProps & { onPhaseChange: (phase: number) => void }) {
  const l = L.find((x) => x.id === "fb")!;
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
    <>
      <div className="detail" ref={detail}>
        <div className="lesson-layout">
          <button className="back lesson-back" data-a="home">
            ← 教材一覧へ戻る
          </button>
          <LessonOutline state={S} />
          <div className="in">
            <h1 className="t">{l.title}</h1>
            {"\n             "}
            <div className="facts" style={{ marginTop: "8px" }}>
              <div>
                <span>{"CVE"}</span>
                {l.cve}
              </div>
              <div>
                <span>{"製品"}</span>
                {l.product}
              </div>
              <div>
                <span>{"Labの出どころ"}</span>
                {l.src}
                {"（"}
                {l.sha}
                {"）"}
              </div>
            </div>
            {"\n             "}
            <LabBox state={S} />
            {"\n             "}
            <div className="pro">
              <h2>{"概要"}</h2>
            </div>
            {"\n             "}
            <div className="prb">
              <p>
                {
                  "File Browser 2.63.5 は、ブラウザからファイルを管理できるWebアプリケーションです。"
                }
              </p>
              <p>
                {l.cve}
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
            {"\n             "}
            <div className="nav overview-actions">
              <button className="btn primary" data-a="begin">
                {S.revealed < 0 ? "学習を開始する ↓" : "Step 1を読み返す ↓"}
              </button>
            </div>
            <div className="lesson-sections">
              {phases.slice(0, S.revealed + 1).map((Phase, i) => (
                <section
                  key={PH[i][0]}
                  id={`phase-${i}`}
                  data-phase={i}
                  className="lesson-phase"
                  aria-labelledby={`phase-title-${i}`}
                >
                  <p className="phase-kicker">
                    STEP {String(i + 1).padStart(2, "0")} / {PH.length}
                  </p>
                  <h2 className="ph" id={`phase-title-${i}`} tabIndex={-1}>
                    {PH[i][0]}
                  </h2>
                  <p className="lead2">{phaseLeads[i]}</p>
                  <div className="phase-body">
                    <Phase lesson={l} state={S} />
                  </div>
                  <div className="nav">
                    <button className="btn" data-a="prev" data-i={i} disabled={i === 0}>
                      前の内容へ
                    </button>
                    <button
                      className="btn primary"
                      data-a="next"
                      data-i={i}
                      disabled={i === PH.length - 1 && S.done[i]}
                    >
                      {i === PH.length - 1
                        ? S.done[i]
                          ? "学習を完了しました"
                          : "完了する"
                        : `次へ：${PH[i + 1][0]} ↓`}
                    </button>
                  </div>
                </section>
              ))}
            </div>
          </div>
        </div>
      </div>
    </>
  );
}
