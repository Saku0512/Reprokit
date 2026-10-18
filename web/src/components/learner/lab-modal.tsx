import type { LearnerState, StateProps } from "../../learner/lesson-data.ts";
export function LabModal({
  state: S,
  onChange,
}: StateProps & { onChange: (patch: Partial<LearnerState>) => void }) {
  if (S.modal === "deploy")
    return (
      <>
        <div className={"ov"}>
          <div className={"dlg"} role={"dialog"} aria-modal={"true"}>
            <h3>{"Labを起動しますか？"}</h3>
            <p style={{ fontSize: "14px", margin: "0 0 10px" }}>
              {
                "固定コミットの取得と安全性の検査のあと、意図的に脆弱なソフトウェアを起動します。公開は127.0.0.1のみですが、Dockerは完全な隔離ではありません。重要な端末では専用VMを使ってください。"
              }
            </p>
            <label style={{ display: "flex", gap: "8px", fontSize: "14px" }}>
              <input
                type={"checkbox"}
                id={"ok"}
                onChange={(e) => onChange({ ok: e.target.checked })}
                checked={S.ok ? true : false}
              />
              {"自分のPC内のLabにだけ試します"}
            </label>
            <div className={"act"} style={{ justifyContent: "flex-end" }}>
              <button className={"btn"} data-a={"mc"}>
                {"やめる"}
              </button>
              <button className={"btn primary"} data-a={"go-deploy"} disabled={S.ok ? false : true}>
                {"起動する"}
              </button>
            </div>
          </div>
        </div>
      </>
    );
  else if (S.modal === "destroy")
    return (
      <>
        <div className={"ov"}>
          <div className={"dlg"} role={"dialog"} aria-modal={"true"}>
            <h3>{"Labを削除しますか？"}</h3>
            <label style={{ fontSize: "14px" }}>
              {"Volume："}
              <select
                id={"vol"}
                value={S.vol}
                onChange={(e) => onChange({ vol: e.target.value === "rm" ? "rm" : "keep" })}
              >
                <option value={"keep"}>{"保持する"}</option>
                <option value={"rm"}>{"このLabのものだけ削除"}</option>
              </select>
            </label>
            <div className={"act"} style={{ justifyContent: "flex-end" }}>
              <button className={"btn"} data-a={"mc"}>
                {"やめる"}
              </button>
              <button className={"btn danger"} data-a={"go-destroy"}>
                {"削除する"}
              </button>
            </div>
          </div>
        </div>
      </>
    );
  else return "";
}
