import type { PhaseProps, StateProps } from "./lesson-data.ts";
import { Fragment } from "react";
import { NODES, TRACE_DETAILS } from "./lesson-data.ts";
import { BehaviorComparison } from "./behavior-comparison.tsx";
import { LabConsole } from "./lab-console.tsx";
import { patchComparisonLines } from "./patch-exercise.ts";
import { PatchWorkshop } from "./patch-workshop.tsx";

export function NormalUsage() {
  return (
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
          <div className="note">
            {
              "このLabには有効な認証情報を用意していません。脆弱版・修正版ともログインが拒否されることが、演習では正常な動作です。"
            }
          </div>
        </div>
      </div>
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
        <div className="reveal">
          {
            "両版とも最終的にはログインを拒否します。次は、拒否される前に入力がどのように扱われるかを確かめます。"
          }
        </div>
      </div>
    </div>
  );
}

export function Reproduction({ lesson, state: S }: PhaseProps) {
  const affectedData = lesson.affectedData ?? [];
  const hints = [
    "（ヒントなし）",
    "このCVEは「認証の結果が出る前」に何かが起きます。ログインの入力欄に注目してください。",
    "入力欄の値が、サーバー側でどこに渡されるかを、前のフェーズのログで確認してください。",
    "教材で用意した検証用マーカー入力（無害）を、レビュー済みデータとして表示します。",
  ];
  return (
    <div className="stage">
      <div>
        <div className="card">
          <h3>{"ガイド"}</h3>
          <ol className="step">
            <li>{"上のLabカードから脆弱版を別タブで開く"}</li>
            <li>{"次の値をログインフォームへ入力して、ログインを試す"}</li>
            <li>{"ログインが拒否された後、「観察結果を取得」を押す"}</li>
          </ol>
          <div className="log" style={{ marginTop: "10px" }}>
            {"Username: id > /tmp/success; echo hook.action=block\n"}
            {"Password: anything"}
          </div>
          <p className="console-empty">
            {
              "この入力は隔離コンテナ内の /tmp/success にマーカーを作るだけです。認証失敗は想定どおりの結果です。"
            }
          </p>
          {S.hint ? (
            <div className="reveal ng">
              <b>
                {"ヒント"}
                {S.hint}
              </b>
              <br />
              {hints[S.hint]}
            </div>
          ) : null}
          <div className="act">
            <button className="btn" data-a="hint" disabled={S.hint >= 3}>
              {"ヒントを見る"}
            </button>
            <button className="btn primary" data-a="repro" disabled={S.lab !== "running"}>
              {"観察結果を取得"}
            </button>
            <button className="btn" data-a="rst">
              {"やり直す"}
            </button>
          </div>
          {S.lab !== "running" && (
            <p style={{ fontSize: "13px", color: "var(--sub)", margin: "8px 0 0" }}>
              {"上のDEPLOYでLabを起動すると押せます。"}
            </p>
          )}
        </div>
      </div>
      <div>
        <div className="card">
          <h3>{"観察結果（隔離コンテナ内）"}</h3>
          <LabConsole state={S} idPrefix="observation" showLead={false} />
        </div>
        {affectedData.length > 0 && (
          <div className="card" style={{ marginTop: "14px" }}>
            <h3>{"影響を受けるデータ"}</h3>
            {S.repro ? (
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
            ) : (
              <p style={{ color: "var(--sub)", fontSize: "13.5px", margin: "0" }}>
                {"再現すると、到達可能になるデータの例がここに並びます。"}
              </p>
            )}
          </div>
        )}
      </div>
    </div>
  );
}

export function RequestTrace({ state: S }: StateProps) {
  const detail = TRACE_DETAILS[S.pos];
  return (
    <div className="stage">
      <div className="card">
        <h3>{"リクエストの流れ（結果から戻る）"}</h3>
        <div className="tl">
          {NODES.map((node, index) => (
            <Fragment key={index}>
              <button
                className={
                  "nd " +
                  (index <= S.pos ? "vis" : "") +
                  " " +
                  (index === S.pos ? "at" : "") +
                  " " +
                  (S.pick === index ? "pick" : "")
                }
                data-a="pick"
                data-i={index}
              >
                <i>{index + 1}</i>
                <span>
                  <b style={{ fontSize: "14px" }}>{node[0]}</b>
                  <small>
                    {node[1]}
                    {"："}
                    {node[2]}
                  </small>
                </span>
                {index <= S.pos ? (
                  <span
                    className={"pill " + (node[3] === "u" ? "red" : node[3] === "t" ? "amber" : "")}
                  >
                    {node[4] || "—"}
                  </span>
                ) : null}
              </button>
            </Fragment>
          ))}
        </div>
        <div className="act">
          <button className="btn" data-a="back" disabled={S.pos <= 0}>
            {"1つ戻る"}
          </button>
          <button className="btn" data-a="fwd" disabled={S.pos >= 5}>
            {"1つ進める"}
          </button>
        </div>
      </div>
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
  );
}

export function PatchComparison() {
  return (
    <div className="stage">
      <div className="card">
        <h3>{"修正差分（演習と同じ抜粋）2.63.5 → 2.63.6"}</h3>
        <p style={{ margin: "0 0 10px", fontSize: "14px" }}>
          {
            "赤い「−」の行が削除された処理です。この抜粋には追加行はありません。記号のない行は修正前からある処理です。"
          }
        </p>
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
        <p style={{ fontSize: "12.5px", color: "var(--sub)", margin: "8px 0 0" }}>
          {"前章と同じ抜粋を使っています。関数全体の差分と回帰テストは"}
          <a
            href="https://github.com/filebrowser/filebrowser/commit/34ae34e764d72540c039f1f5ea2ec4c974168c1f"
            target="_blank"
            rel="noopener"
          >
            {"公式修正コミット"}
          </a>
          {"で確認してください。"}
        </p>
      </div>
      <div>
        <div className="card">
          <h3>{"原因から修正まで"}</h3>
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
  );
}

export function Remediation({ state: S }: StateProps) {
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

export const lessonPhases = [
  NormalUsage,
  Reproduction,
  RequestTrace,
  PatchWorkshop,
  PatchComparison,
  Remediation,
];

export const lessonPhaseLeads = [
  "このLabではログインが拒否されることが正常な動作です。拒否される前の処理を確認します。",
  "ガイドに沿って検証し、観察結果を確認します。",
  "結果から処理をたどり、入力値がコマンドとして扱われる流れを確認します。",
  "公式の修正差分を見る前に、原因となった処理を探してコードを編集します。",
  "実際の修正差分と、自分が見つけた原因との対応を確認します。",
  "修正版で同じ操作をもう一度行い、両版のログイン拒否と副作用の違いを確認します。",
];
