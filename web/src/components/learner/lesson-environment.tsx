import { Fragment } from "react";
import type { StateProps } from "../../learner/lesson-data.ts";
import { endpointURL } from "../../shared/study-api.ts";

export function LessonEnvironment({ state: S }: StateProps) {
  const s = S.lab;
  const endpointRank = (service: string) =>
    service === "vulnerable" ? 0 : service === "fixed" ? 1 : 2;
  const endpoints = (S.runtime?.endpoints ?? [])
    .map((endpoint) => ({ ...endpoint, url: endpointURL(endpoint) }))
    .filter((endpoint) => endpoint.url)
    .sort((left, right) => endpointRank(left.service) - endpointRank(right.service));
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
  );
}
