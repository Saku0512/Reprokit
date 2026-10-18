import type { StateProps } from "../../learner/lesson-data.ts";

export function LabConsole({
  state: S,
  idPrefix = "console",
  showLead = true,
}: StateProps & { idPrefix?: string; showLead?: boolean }) {
  const activeService = "vulnerable";
  const logs = S.logs?.[activeService];
  const marker = S.markers?.[activeService];
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
              <b>{"マーカー"}</b>
              <p>
                <code>{"cat /tmp/success"}</code>
                {" — idの出力だけを確認する固定の読み取り操作"}
              </p>
            </div>
            <button
              className="btn sm"
              data-a="marker"
              data-v={activeService}
              disabled={S.lab !== "running" || S.markerLoading !== null}
            >
              {S.markerLoading === activeService ? "確認中…" : "確認"}
            </button>
          </div>
          {marker ? (
            <pre className="log">
              {marker.exists
                ? marker.content || "（idの出力はありません）"
                : "（マーカーはありません）"}
            </pre>
          ) : (
            <p className="console-empty">{"まだマーカーを確認していません。"}</p>
          )}
        </div>
      </section>
      {(S.logFailure || S.markerFailure) && (
        <p className="lab-failure" role="alert">
          {S.logFailure || S.markerFailure}
        </p>
      )}
      <p className="console-notice">
        {"ログや出力にはLabへ入力した値が含まれる場合があります。共有しないでください。"}
      </p>
    </div>
  );
}
