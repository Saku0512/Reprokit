import { useState } from "react";
import { checkPatch, patchHints, starterPatch } from "../../learner/patch-exercise.ts";
import type { PatchResult } from "../../learner/patch-exercise.ts";

export function PatchWorkshop() {
  const [code, setCode] = useState(starterPatch);
  const [hint, setHint] = useState(0);
  const [result, setResult] = useState<PatchResult | null>(null);

  return (
    <div className="patch-workshop">
      <div className="card">
        <h3>修正する場所を探す</h3>
        <p>
          認証フックへ資格情報を渡す処理です。入力がコマンドの一部になる箇所を探し、
          フックがユーザー名とパスワードを受け取れる状態を保ちながら修正してください。
        </p>
        <p className="console-empty" id="patch-context">
          File Browser
          2.63.5のauth/hook.go、RunCommandからの抜粋です（インデントを調整）。a.Commandは管理者の設定、
          a.Credはログイン入力です。envMappingはUSERNAMEとPASSWORDに対応する資格情報を返します。
          必要なimportと周辺の処理は省略しています。
        </p>
        <label className="patch-label" htmlFor="patch-code">
          修正するコード（Go）
        </label>
        <textarea
          id="patch-code"
          className="patch-editor"
          aria-describedby="patch-context patch-check-scope"
          value={code}
          maxLength={12000}
          spellCheck={false}
          autoCapitalize="off"
          autoCorrect="off"
          wrap="off"
          onChange={(event) => {
            setCode(event.target.value);
            setResult(null);
          }}
        />
        <div className="act">
          <button className="btn primary" onClick={() => setResult(checkPatch(code))}>
            修正内容をチェック
          </button>
          <button
            className="btn"
            onClick={() => {
              setCode(starterPatch);
              setResult(null);
            }}
          >
            元のコードに戻す
          </button>
          <button
            className="btn"
            disabled={hint === patchHints.length}
            onClick={() => setHint(hint + 1)}
          >
            修正のヒントを見る
          </button>
        </div>
        {hint > 0 && (
          <ol className="patch-hints">
            {patchHints.slice(0, hint).map((text) => (
              <li key={text}>{text}</li>
            ))}
          </ol>
        )}
        <p className="console-empty" id="patch-check-scope">
          この章は抜粋の構造をチェックします。Goのビルド、コードの実行、Labへの反映は行いません。
          実際の挙動は、後の章で公式の修正版Labを使って比較します。
        </p>
        <p className="console-empty">
          教材一覧へ戻るか、ページを再読み込みすると、コードは初期化されます。
        </p>
      </div>
      <section className="card" aria-label="修正チェックの結果" aria-live="polite">
        <h3>修正チェックの結果</h3>
        {result ? (
          <>
            <p className={`pill ${result.passed ? "green" : "amber"}`}>
              {result.passed ? "抜粋の修正条件を満たしました" : "修正を見直してください"}
            </p>
            {result.error && <p>{result.error}</p>}
            <ul className="patch-checks">
              {result.checks.map((check) => (
                <li key={check.title}>
                  <b>
                    {check.passed ? "✓ 確認済み：" : "要修正："}
                    {check.title}
                  </b>
                  <p>{check.detail}</p>
                </li>
              ))}
            </ul>
            {result.passed && <p>次の章で公式パッチと照らし、変更の理由を確認しましょう。</p>}
          </>
        ) : (
          <p className="console-empty">コードを編集してチェックすると、結果が表示されます。</p>
        )}
      </section>
    </div>
  );
}
