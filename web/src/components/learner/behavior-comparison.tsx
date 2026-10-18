import type { StateProps } from "../../learner/lesson-data.ts";
import { markerCredentials } from "../../learner/lesson-data.ts";
import type { LabRef } from "../../shared/types.ts";
import { endpointURL } from "../../shared/study-api.ts";

const variants = [
  {
    service: "vulnerable",
    title: "脆弱版",
    version: "2.63.5",
    outcome: "入力が命令になる",
    steps: [
      ["入力をコマンド引数へ展開", "os.Expand(arg, envMapping)"],
      ["シェルが入力を命令として解釈", "「>」で書き込み、「;」で命令を連結"],
      ["認証前にファイルが作られる", "/tmp/success に id の実行結果"],
    ],
  },
  {
    service: "fixed",
    title: "修正版",
    version: "2.63.6",
    outcome: "入力をデータとして扱う",
    steps: [
      ["入力を引数へ展開しない", "資格情報は cmd.Env で渡す"],
      ["入力をシェル構文に再解釈しない", "値に含まれる「>」「;」は命令にならない"],
      ["ファイルは作られない", "/tmp/success への書き込みなし"],
    ],
  },
] satisfies {
  service: LabRef;
  title: string;
  version: string;
  outcome: string;
  steps: string[][];
}[];

function ObservedMarker({ state: S, service }: StateProps & { service: LabRef }) {
  const result = S.comparison[service];
  const marker = result?.marker;
  const expected = marker && marker.exists === (service === "vulnerable");
  const tone = marker ? (expected ? (marker.exists ? "danger" : "success") : "warning") : "";
  return (
    <div className="comparison-observation">
      <h5>Labの観察結果</h5>
      <div className={`marker-result ${tone}`} role="status" aria-atomic="true">
        {result?.loading ? (
          <p>マーカーを取得中…</p>
        ) : result?.failure ? (
          <>
            <b>取得できませんでした</b>
            <p>{result.failure}</p>
          </>
        ) : marker ? (
          <>
            <b className="marker-title">
              <span aria-hidden="true">{marker.exists ? "▣" : "□"}</span>
              {marker.exists ? "マーカーあり" : "マーカーなし"}
            </b>
            <p className="marker-path">
              <code>/tmp/success</code>：{marker.exists ? "作成の痕跡あり" : "見つかりません"}
            </p>
            {marker.exists && (
              <pre className="marker-content">{marker.content || "（内容は空です）"}</pre>
            )}
            <p>
              {expected
                ? service === "vulnerable"
                  ? "想定どおり、コマンド実行による書き込みの痕跡があります。"
                  : "想定どおり、書き込みの痕跡はありません。未送信でも同じ結果になるため、ログインを試したことと併せて判断してください。"
                : service === "vulnerable"
                  ? "想定と異なる結果です。同じ入力でログインを試し、再取得してください。"
                  : "想定と異なる結果です。接続先や過去の操作を確認し、必要ならLabを削除して再作成してください。"}
            </p>
          </>
        ) : (
          <>
            <b>未確認</b>
            <p>同じ入力でログインを試した後、マーカーを取得してください。</p>
          </>
        )}
      </div>
      <button
        className="btn"
        data-a="compare"
        data-v={service}
        disabled={S.lab !== "running" || result?.loading}
      >
        {result?.loading
          ? "取得中…"
          : `${service === "vulnerable" ? "脆弱版" : "修正版"}のマーカーを${marker ? "再取得" : "取得"}`}
      </button>
    </div>
  );
}

export function BehaviorComparison({ state: S }: StateProps) {
  const vulnerable = S.comparison.vulnerable?.marker;
  const fixed = S.comparison.fixed?.marker;
  return (
    <div className="card behavior-comparison">
      <h3>同じ入力で、認証前の副作用を比べる</h3>
      <p className="comparison-lead">
        ログイン拒否は両版で同じです。違いは、その前に入力が命令として扱われ、ファイルが作られるかどうかです。
      </p>
      <div className="comparison-input">
        <b>両版に使う同じ入力</b>
        <dl>
          <div>
            <dt>Username</dt>
            <dd>
              <code>{markerCredentials.username}</code>
            </dd>
          </div>
          <div>
            <dt>Password</dt>
            <dd>
              <code>{markerCredentials.password}</code>
            </dd>
          </div>
        </dl>
        <p>
          各版を別タブで開き、この値でログインを試してからマーカーを取得します。取得ボタンは入力を送信しません。
        </p>
      </div>
      <p className="comparison-caption">処理の流れ（教材の想定）</p>
      <div className="comparison-columns">
        {variants.map((variant) => {
          const endpoint = S.runtime?.endpoints.find((item) => item.service === variant.service);
          const url = endpoint && S.lab === "running" ? endpointURL(endpoint) : null;
          return (
            <section
              className="comparison-variant"
              data-service={variant.service}
              key={variant.service}
              aria-label={`${variant.title}の挙動`}
            >
              <div className="comparison-variant-heading">
                <h4>
                  {variant.title} <span className="mono">{variant.version}</span>
                </h4>
                <span className={`pill ${variant.service === "vulnerable" ? "red" : "green"}`}>
                  {variant.outcome}
                </span>
              </div>
              <ol className="comparison-flow">
                {variant.steps.map(([title, detail], index) => (
                  <li key={title}>
                    <span className="flow-number" aria-hidden="true">
                      {index + 1}
                    </span>
                    <div>
                      <b>{title}</b>
                      <small>{detail}</small>
                    </div>
                  </li>
                ))}
              </ol>
              <div className="comparison-response">
                <span>その後、両版ともログイン拒否</span>
                <code>403 Forbidden</code>
              </div>
              {url ? (
                <a
                  className="btn comparison-open"
                  href={url}
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  {variant.title}を別タブで開く ↗
                </a>
              ) : (
                <p className="console-empty">Labを起動して接続先を確認してください。</p>
              )}
              <ObservedMarker state={S} service={variant.service} />
            </section>
          );
        })}
      </div>
      {vulnerable && fixed && (
        <div className={`reveal ${vulnerable.exists && !fixed.exists ? "" : "ng"}`} role="status">
          <b>
            {vulnerable.exists && !fixed.exists
              ? "マーカーの有無に差があります"
              : "想定した差をまだ確認できていません"}
          </b>
          <p>
            {vulnerable.exists && !fixed.exists
              ? "脆弱版には実行の痕跡が残り、修正版にはありません。同じ入力を両版へ送った場合、ログイン拒否だけでは見えない副作用の差を確認できます。"
              : "脆弱版でマーカーあり、修正版でマーカーなしが想定結果です。各版の接続先と入力を確認し、再取得してください。"}
          </p>
        </div>
      )}
    </div>
  );
}
