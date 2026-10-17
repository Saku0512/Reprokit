import { Fragment, useEffect, useRef } from "react";
import { L, PH, NODES } from "./data.js";
import { endpointURL } from "./api.js";
export function LessonList({ state: S }) {
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
                deg = (n / 5) * 360;
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
                          n === 5 ? (
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
                          <em>{"/ 5"}</em>
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

function LabBox({ state: S }) {
  const s = S.lab;
  const endpoints = (S.runtime?.endpoints ?? [])
    .map((endpoint) => ({ ...endpoint, url: endpointURL(endpoint) }))
    .filter((endpoint) => endpoint.url);
  const endpointTitle = (service) =>
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
              <a className="btn sm" href={endpoint.url} target="_blank" rel="noopener noreferrer">
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
          <b>{"◎ ターゲットマシン"}</b>
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

export function LabConsole({ state: S, idPrefix = "console", showLead = true }) {
  const services = [
    ["vulnerable", "脆弱コンテナ"],
    ["fixed", "修正版コンテナ"],
  ];
  const activeService = services.some(([service]) => service === S.consoleService)
    ? S.consoleService
    : "vulnerable";
  const activeTitle = services.find(([service]) => service === activeService)?.[1];
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
      <div className="console-switch" role="tablist" aria-label="観察するコンテナ">
        {services.map(([service, title]) => (
          <button
            key={service}
            className="btn"
            role="tab"
            aria-selected={activeService === service}
            aria-controls={`${idPrefix}-${service}-panel`}
            data-a="console-service"
            data-v={service}
          >
            {title}
          </button>
        ))}
      </div>
      <section id={panelID} className="console-service" role="tabpanel" aria-label={activeTitle}>
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

function NormalUsage({ lesson: l, state: S }) {
  return (
    <>
      <div className="stage">
        <div>
          <div className="card">
            <h3>{"やってみること"}</h3>
            <ol className="step">
              <li>{"右の「別タブで開く」で、Labのログイン画面を開く"}</li>
              <li>{"正しいユーザー名・パスワードでログインする"}</li>
              <li>{"わざと間違えて、ログインに失敗する様子も見る"}</li>
              <li>{"戻ってきて、サーバー側のログを確認する"}</li>
            </ol>
            {"\n            "}
            <div className="note">
              {"Labの画面は別タブで動きます。このタブは、観察と解説に使います。"}
            </div>
          </div>
        </div>
        {"\n            "}
        <div className="card">
          <h3>{"サーバー側で起きたこと（Runnerが取得）"}</h3>
          <div className="sw">
            <button data-a="lg" data-v="ok" aria-pressed={S.login === "ok"}>
              {"成功したログイン"}
            </button>
            <button data-a="lg" data-v="ng" aria-pressed={S.login === "ng"}>
              {"失敗したログイン"}
            </button>
          </div>
          {"\n            "}
          <div className="log">
            {S.login === null ? (
              "（上のボタンで、Labで試した結果の流れを表示）"
            ) : S.login === "ok" ? (
              <>
                <span className="ok">{"1 受信"}</span>
                {" POST /api/login\n"}
                <span className="ok">{"2 フック認証"}</span>
                {" 設定済みコマンドへ資格情報を渡す\n"}
                <span className="ok">{"3 判定"}</span>
                {" 終了コード0 → 許可\n"}
                <span className="ok">{"4 応答"}</span>
                {" 200 / セッション発行"}
              </>
            ) : (
              <>
                <span className="ok">{"1 受信"}</span>
                {" POST /api/login\n"}
                <span className="ok">{"2 フック認証"}</span>
                {" 設定済みコマンドへ資格情報を渡す\n"}
                <span className="t">{"3 判定"}</span>
                {" 終了コード1 → 拒否\n"}
                <span className="t">{"4 応答"}</span>
                {" 403 Forbidden"}
              </>
            )}
          </div>
          {"\n            "}
          <div className="reveal">
            {
              "ログインは「入力を受け取る → 外部コマンドで判定 → 結果を返す」の順に動きます。次で、この途中に何が起きるかを確かめます。"
            }
          </div>
        </div>
      </div>
    </>
  );
}

function Reproduction({ state: S }) {
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
              <li>{"ログインが拒否された後、脆弱コンテナを選んで「観察結果を取得」を押す"}</li>
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
            <h3>{"影響を受ける架空データ"}</h3>
            {S.repro ? (
              <>
                <table className="data">
                  <tbody>
                    <tr>
                      <th>{"種類"}</th>
                      <th>{"内容（すべて架空）"}</th>
                    </tr>
                    <tr>
                      <td>{"ユーザー"}</td>
                      <td>{"sample-admin / sample-guest"}</td>
                    </tr>
                    <tr>
                      <td>{"ファイル"}</td>
                      <td>{"dummy-report.txt / dummy-keys.txt"}</td>
                    </tr>
                    <tr>
                      <td>{"設定"}</td>
                      <td>{"fake-token = 0000-DUMMY"}</td>
                    </tr>
                  </tbody>
                </table>
                <p style={{ fontSize: "12.5px", color: "var(--sub)", margin: "6px 0 0" }}>
                  {"実際の事故で見える可能性があるものを示す、教材用の架空データです。"}
                </p>
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

function RequestTrace({ state: S }) {
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
        <div className="card">
          <h3>{"この地点の様子"}</h3>
          <div className="log">
            {
              [
                "受信直後の値: 外部入力（未検証）",
                "分岐: Hook Authentication",
                "保持: 値は加工されていない",
                <>
                  <span className="t">{"展開: 値がコマンド引数へ"}</span>
                </>,
                <>
                  <span className="u">{"実行: 展開後の文字列を解釈"}</span>
                </>,
                <>
                  <span className="t">{"確定: すでにコマンドは実行済み"}</span>
                </>,
              ][S.pos]
            }
          </div>
        </div>
      </div>
    </>
  );
}

function PatchComparison({ state: S }) {
  const h = S.diffHl;
  return (
    <>
      <div className="stage">
        <div className="card">
          <h3>{"修正差分（要約）2.63.5 → 2.63.6"}</h3>
          {"\n            "}
          <p style={{ margin: "0 0 10px", fontSize: "14px" }}>
            {
              "赤い行が脆弱版で削除された処理、緑の行が修正版で追加された処理です。行をクリックすると、下の説明と対応する行が強調されます。"
            }
          </p>
          {"\n            "}
          <div className="diff">
            <div className={"del " + (h === "a" ? "hl" : "")} data-a="hl" data-v="a">
              {"- args := os.Expand(hookCommand, lookup(username, password))"}
            </div>
            <div className={"del " + (h === "a" ? "hl" : "")} data-a="hl" data-v="a">
              {'- cmd := exec.Command("sh", "-c", args)'}
            </div>
            <div> </div>
            <div className={"add " + (h === "b" ? "hl" : "")} data-a="hl" data-v="b">
              {'+ cmd := exec.Command("sh", "-c", hookCommand)'}
            </div>
            <div className={"add " + (h === "b" ? "hl" : "")} data-a="hl" data-v="b">
              {'+ cmd.Env = append(cmd.Env, "USERNAME="+username, "PASSWORD="+password)'}
            </div>
          </div>
          {"\n            "}
          <p style={{ fontSize: "12.5px", color: "var(--sub)", margin: "8px 0 0" }}>
            {"学習用に簡略化した概念図です。実際の差分は"}
            <a
              href={
                "https://github.com/filebrowser/filebrowser/security/advisories/GHSA-m93h-4hw7-5qcm"
              }
              target="_blank"
              rel="noopener"
            >
              {"公式アドバイザリ"}
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
              <b>{"あなたが見つけた原因（フェーズ3・手順4）"}</b>
              <br />
              <code>{"os.Expand"}</code>
              {"がユーザー名・パスワードをコマンド文字列の中へ展開し、その文字列を次の"}
              <code>{"sh -c"}</code>
              {
                "がシェルの命令として解釈します。入力値がデータではなくコマンドの一部になっていたことが問題です。"
              }
            </p>
            {"\n            "}
            <p style={{ margin: "0", fontSize: "14px" }}>
              <b>{"修正版で変わったこと"}</b>
              <br />
              {"文字列への展開をやめ、コマンド本体は設定済みの"}
              <code>{"hookCommand"}</code>
              {
                "のまま実行します。ユーザー名・パスワードは環境変数として別に渡すため、コマンドの構文に混ざりません。"
              }
            </p>
            {"\n            "}
            {h === "a" ? (
              <>
                <div className="reveal">
                  {"赤い行が原因に対応します。特に"}
                  <code>{"os.Expand"}</code>
                  {"の行で入力値をコマンド文字列へ混ぜ、次の"}
                  <code>{"sh -c"}</code>
                  {"の行でシェルに解釈させていました。"}
                </div>
              </>
            ) : h === "b" ? (
              <>
                <div className="reveal">
                  {
                    "緑の行が修正に対応します。固定のコマンド本体と、環境変数で渡す入力値を分けています。"
                  }
                </div>
              </>
            ) : (
              <>
                <p style={{ color: "var(--sub)", fontSize: "13.5px", margin: "8px 0 0" }}>
                  {
                    "赤い行は「入力値をコマンドに混ぜた原因」、緑の行は「入力値をコマンドから分離する修正」です。"
                  }
                </p>
              </>
            )}
          </div>
        </div>
      </div>
    </>
  );
}

function Remediation({ lesson: l, state: S }) {
  const none = (
    <>
      <span style={{ color: "var(--sub)" }}>{"未実行"}</span>
    </>
  );
  return (
    <>
      <div className="card">
        <h3>{"同じ操作を、修正版（2.63.6・8081）で再実行"}</h3>
        <p style={{ margin: "0", fontSize: "14px" }}>
          {"フェーズ2と同じ入力で、攻撃は防がれ、普通のログインは成功することを確かめます。"}
        </p>
        {"\n            "}
        <div className="cmp" style={{ marginTop: "12px" }}>
          <div>
            <b style={{ fontSize: "14px" }}>{"脆弱版 2.63.5"}</b>
            <div className={"res " + (S.runV ? "bad" : "")} style={{ marginTop: "6px" }}>
              {S.runV ? (
                <>
                  <span className="pill red">{"マーカーが作られた"}</span>
                  <br />
                  {"入力が危険な実行経路に届きました。"}
                </>
              ) : (
                none
              )}
            </div>
            <div className="act">
              <button className="btn" data-a="runV" disabled={S.lab === "running" ? false : true}>
                {"検証用入力を送る"}
              </button>
            </div>
          </div>
          {"\n            "}
          <div>
            <b style={{ fontSize: "14px" }}>{"修正版 2.63.6"}</b>
            <div className={"res " + (S.runF ? "good" : "")} style={{ marginTop: "6px" }}>
              {S.runF ? (
                <>
                  <span className="pill green">{"防がれた"}</span>
                  <br />
                  {"マーカーは作られません。"}
                </>
              ) : (
                none
              )}
              {S.normF ? (
                <>
                  <hr style={{ border: "0", borderTop: "1px solid var(--line)" }} />
                  <span className="pill green">{"通常利用は成功"}</span>
                  <br />
                  {"demo-user は通常どおりログインできました。"}
                </>
              ) : (
                ""
              )}
            </div>
            <div className="act">
              <button
                className="btn primary"
                data-a="runF"
                disabled={S.lab === "running" ? false : true}
              >
                {"同じ入力を送る"}
              </button>
              <button className="btn" data-a="normF" disabled={S.lab === "running" ? false : true}>
                {"通常のログインを試す"}
              </button>
            </div>
          </div>
        </div>
        {"\n            "}
        {S.runF && S.normF ? (
          <>
            <div className="reveal">
              <b>{"確認できました。"}</b>
              <br />
              {
                "防ぐべきものは防ぎ、必要な使い方は壊していません。最後にDESTROYでLabを片付けましょう。"
              }
            </div>
          </>
        ) : (
          ""
        )}
      </div>
      {"\n            "}
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

const phases = [NormalUsage, Reproduction, RequestTrace, PatchComparison, Remediation];
const phaseLeads = [
  "まず、このプロダクトが本来どう動くかを確認します。",
  "ガイドに沿って検証し、観察結果と、影響を受ける架空データを確認します。",
  "結果から処理をたどり、入力値がコマンドとして扱われる流れを確認します。",
  "実際の修正差分と、自分が見つけた原因との対応を確認します。",
  "修正版で同じ操作をもう一度行い、防がれることと通常利用の成功を確認します。",
];

export function LessonOutline({ state: S, id = "lesson-outline" }) {
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

export function LessonDetail({ state: S, onPhaseChange }) {
  const l = L.find((x) => x.id === "fb");
  const detail = useRef(null);

  useEffect(() => {
    const container = detail.current;
    let frame;
    const updatePosition = () => {
      const bounds = container.getBoundingClientRect();
      const top = Math.max(0, bounds.top) + Math.min(window.innerHeight * 0.25, 160);
      let phase = -1;
      for (const section of container.querySelectorAll("[data-phase]")) {
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
                      disabled={i === 4 && S.done[i]}
                    >
                      {i === 4
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
