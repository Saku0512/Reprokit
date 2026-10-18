---
name: reprokit-verify
description: Format and verify Reprokit changes. Use when running tests, checking formatting, diagnosing a failed project check, or preparing a change for review.
---

# Reprokit Verify

Use the repository scripts as the single verification interface. Do not substitute global formatter versions or invent commands from another project.

## Select the check

- Documentation-only or configuration-only change: run `npm run format:check` and `git diff --check`.
- Ordinary implementation change: run `npm run verify` during development.
- Core, Runtime, Security, shared-contract, dependency, or pre-PR verification: run `npm run verify:full`.
- Run a narrower package test first when diagnosing a failure, then rerun the applicable repository command.

If `node_modules` is missing or `package-lock.json` changed, run `npm ci` before verification. Do not use a globally installed Prettier.

## Workflow

1. Inspect the changed paths and choose the smallest applicable check above.
2. Run `npm run format` only when formatting changes are authorized; otherwise start with `npm run format:check`.
3. Run the selected verification command.
4. Fix failures caused by the current change, then rerun the failed check.
5. Report commands run, pass/fail status, and every skipped or unavailable check.

## Safety boundaries

- General verification must not start real vulnerable Vulhub labs.
- Docker integration tests require `REPROKIT_RUN_DOCKER_TESTS=1` and must use harmless dedicated fixtures.
- Never weaken a test, formatter, schema, or security rule merely to obtain a passing run.
- Do not modify unrelated files to resolve a pre-existing failure.
- A missing component is `skipped`, not `passed`.

## Script behavior

- `scripts/format.sh check|write` applies `gofmt` from `backend/go.mod` to Go and the pinned Prettier to supported repository files.
- `scripts/verify.sh quick|full` checks formatting and whitespace, then runs available Go checks from `backend/` and Web checks from `web/`. `full` adds the race suite and opted-in harmless Docker integration tests.

## Known failure modes

- 既存の教材へ質問UIを追加・変更するときは、前の回答を残して下へ進み、上へ戻って読み返せることをブラウザで確認する。進行方式を確認せず一問表示へ置き換えると、教材との操作の一貫性が崩れるため。
- 依存関係のインストールと、それを読むlint・ビルド・テストは並列に実行しない。インストールの終了コードを確認してから開始する。展開途中の依存ファイルを読むと、コードの不具合と区別しにくい解決エラーになる。
- メニューや固定ヘッダーを変更した場合は、本文を下へスクロールしてから開閉し、閉じた後の読書位置と見出しの可視性を確認する。スクロール固定で対象要素が切り替わると位置が先頭へ戻るため、`window.scrollY`だけでなく本文のスクロール要素も検証する。
- スクロール量を調整する場合は、要求された要素の境界を実座標で検証する。次章の見出しが見えることやフォーカスの移動だけでは、直前の章の下端など指定された位置で止まったことを確認できない。質問カードへの移動ではカード上端と質問番号も対象とし、ヘッダー下端との間に余白が残ることを前進・後戻り・途中再開で確認する。見出しだけに合わせると、その上のカードや質問番号がヘッダーに隠れるため。
- DOMの型付けでは、条件付き表示要素の取得を必須要素へ一括置換しない。画面遷移ごとの存在条件を維持し、E2Eで確認する。TypeScriptの型検査だけでは、現在の画面に要素が存在することは保証できない。
- 画面の初期位置や操作順を変えたときは、初期表示からの遷移をE2Eで検証する。旧初期位置へ直接到達した前提の期待値を残さず、前進・後戻り・端のボタン状態など既存の検証項目も維持する。
- コード編集の演習と解説差分が続く教材では、差分から復元した修正前・修正後のコードを演習と照合する。既存処理を追加行として説明しない。共通の抜粋から表示を生成し、挙動の概念図を公式パッチとして扱わない。章単位の動作確認だけでは、章をまたぐ内容の不一致を見落とすため。
- 表示文言や要素を変更した場合は、固定HTMLを使う見た目比較の参照側も確認する。意図した変更だけを参照側へ反映して比較を続け、古い表示との差をアプリの不具合と混同しない。
- Tailwindを使う画面では、状態値をそのままクラス名にせず、data属性や接頭辞で既存ユーティリティとの衝突を避ける。`fixed`が`position: fixed`として解釈されるような衝突は型検査では分からないため、ブラウザの実座標とクリック操作で配置を確認する。
- 記号や空白が意味を持つ検証入力は、項目名を含まない値全体のコピー導線を用意し、実クリップボードとの一致をE2Eで確認する。再現失敗を入力手順だけで説明する前に、実際の入力値・送信先・実ログを照合する。
