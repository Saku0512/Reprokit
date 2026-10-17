# 受講UI

## Reactサンプル画面

教材一覧、分野フィルター、5つの学習フェーズ、Lab操作ダイアログ、進捗、表示切替を再現する。
教材詳細は縦のナビゲーションと、上から読み進める構成にする。
初めて教材を開くと概要を表示し、「学習を開始する」でStep 1を下に表示する。
「次へ」で次の章を下へ追加し、滑らかにスクロールして移動する。
移動先は直前の章の下端を本文領域の上端へそろえ、次章の導入と余白が見える位置とする。
PCではナビゲーションの上端と教材タイトルをそろえ、教材一覧への戻るボタンをナビゲーションの上に置く。
表示済みの章と操作結果を残し、スクロールやナビゲーションから読み返せる。
スクロール中の現在位置をナビゲーションへ反映し、未表示の章は「次へ」で順番に開く。
OSで動きを減らす設定が有効な場合は、アニメーションを省いて移動する。
幅1,000px以下では学習ナビゲーションをヘッダーのハンバーガーメニューに収める。
ヘッダーはスクロール中も上部に固定し、章の選択後はメニューを閉じて見出しへ移動する。
メニューはEscapeキーと閉じるボタンでも閉じられる。
Lab操作・観察結果は元HTMLと同じデモ表示であり、RunnerやDockerは操作しない。
実Labに接続する従来の受講画面は `/runner`、管理画面は `/admin` に残している。

RunnerなしでReact画面を確認する場合は `npm --prefix web run dev:external` を実行する。
開発配信では初回アクセス時にReactと全画面のTailwind CSSをビルドする。ソース変更後はサーバーを再起動する。
`npm run web:build` はReact、CSS、フォントを `web/dist/generated/` に同梱する。
フォントはサンプルと同じNoto Sans JP / IBM Plex Monoを使用し、Google Fontsへの通信は不要。

`src/sample-app.jsx` が状態管理、`sample-views.jsx` が一覧と各学習フェーズ、
`sample-modal.jsx` が確認ダイアログ、`sample-data.js` がサンプルの固定データを担当する。
`src/sample.css`、`src/style.css`、`src/admin.css` はTailwind CSSの `@apply` で各画面のスタイルを定義する。
共通の読み込みと色の対応は `src/tailwind.css` に置き、既存のライト・ダークテーマのCSS変数を参照する。
既存のブラウザ標準スタイルを維持するためPreflightは読み込まない。
フォント、テーマ値、独自アニメーションなどは通常のCSSで定義する。
`scripts/frontend.mjs` がPostCSSとTailwindをesbuildへ組み込み、
`generated/app.css`、`generated/runner.css`、`generated/admin.css` を生成する。
画面へユーティリティクラスを直接追加することもできる。HTMLと `src/` 内のコードが検出対象になる。
HTMLの直接挿入は使わず、JSXで描画する。

## Runner接続画面

File Browserの認証フック教材を、背景、予想、手動再現、観察メモ、原因と修正、確認課題、後片付けまで進めるローカルWeb UI。
受講UIと、実Go Runnerへ接続するローカル起動処理を含む。
HTTP契約と教材本文は変更しない。

## 開発と組み込み

Node.js 22以上、`backend/go.mod`で指定したGo toolchain、Gitが必要。
Labを起動する場合はDocker Engine 28以上とDocker Compose v2.36.1以上も必要になる。

リポジトリ直下から実行する場合は、次のルートスクリプトを使う。

```sh
npm ci
npm run web:install
npm run web:build
npm run web:start
```

開発時は `npm run web:dev` でソースを直接配信する。
`start`と`dev`は `backend/cmd/reprokit` を一時領域へビルドし、Go Runnerと教材APIを起動してからWebを起動する。
Runnerは `127.0.0.1` の空きポートへ待ち受け、接続先とトークンを親Webシェルだけが読む専用パイプへ渡す。
URLやトークンの手入力は不要。
トークンは端末やファイルへ出力しない。

同じ起動処理で研修管理APIも接続し、`/admin` に管理画面を公開する。
管理ユーザー名は `REPROKIT_ADMIN_USERNAME`（既定 `admin`）、パスワードは `REPROKIT_ADMIN_PASSWORD` で指定する。
パスワード未指定時は起動ごとに一時パスワードを生成して端末へ表示し、再起動時に失効する。
ログイン後はHttpOnly Cookieを使い、Runnerの管理トークン自体はHTML、JavaScript、Cookieへ渡さない。

OIDCを使う場合は、IdPへ `http://127.0.0.1:4173/api/admin/oidc/callback` をRedirect URIとして登録し、次の値をすべて指定する。

```sh
REPROKIT_OIDC_ISSUER=https://idp.example.com
REPROKIT_OIDC_CLIENT_ID=reprokit
REPROKIT_OIDC_CLIENT_SECRET=...
REPROKIT_OIDC_REDIRECT_URL=http://127.0.0.1:4173/api/admin/oidc/callback
REPROKIT_ADMIN_SUBJECT=<初期組織管理者のOIDC sub>
npm run web:start
```

認可コード、PKCE、state、nonceを使い、ID Tokenの署名、issuer、audience、有効期限をRunnerで検証する。
Client SecretとPKCE verifierはRunner外へ出さず、認証後のWebセッションには組織IDとユーザーIDだけを保持する。
OIDCから返る `sub` は、あらかじめ組織ユーザーへ登録されたSubjectと完全一致する必要がある。
新しい状態保存先では `REPROKIT_ADMIN_SUBJECT` を初期組織管理者へ設定する。
既存の状態保存先では、パスワードでログインして `POST /api/identity/users` からIdPのSubjectを持つ組織管理者を追加してからOIDCを有効にする。

起動時にはコミット済みのRegistryとFile Browser教材を同じHEADのスナップショットから検証する。
作業ツリーの未コミット教材や設定変更は公開対象へ混ぜない。
固定ソースの取得は最初の「Labを準備する」まで遅らせ、共通のResolver、Materializer、Core検査を通す。
起動と教材閲覧ではGitの外部通信やDocker操作を行わない。
準備もコンテナを起動しない。

保存先は `REPROKIT_STATE_DIR`、未指定なら `$XDG_STATE_HOME/reprokit`、XDG未指定なら `~/.local/state/reprokit`。
保存先とその `study/`、`labs/`、`training/`、`identity/` は実ディレクトリかつ権限0700に限定する。
受講状態、研修の割り当て・提出、Labの検証情報を保存し、プロセスを再起動しても復元する。
Ctrl+CでWebとRunnerを終了する。
親Webシェルのパイプが切れた場合もRunnerを終了する。
アプリ終了はLab削除の操作ではないため、演習後は画面で停止と削除を行う。

既存Runnerへ接続する組み込み用途では、`REPROKIT_RUNNER_ORIGIN`と`REPROKIT_RUNNER_TOKEN`を起動元で設定し、`npm run web:start:external`を使う。
接続先は `http://127.0.0.1:<Runnerのport>`、トークンはRunnerが発行した値を指定する。
ソース配信には `dev:external` を使う。

Webシェルは既定で `http://127.0.0.1:4173` にだけ待ち受ける。
portを変える場合は起動元で `REPROKIT_WEB_PORT` を設定する。
ブラウザからは同じoriginの `/api/` を使い、シェルが起動時に固定したRunnerだけへ転送する。
Host、同一Origin、ローカルセッショントークンを検証し、転送先OriginもRunner自身のoriginへ固定する。
転送先のredirectはたどらず、CORSは許可しない。
接続設定がない場合、APIは503となり、`/runner` は再接続を案内する。
架空の教材やLab成功結果には切り替えない。

`dist/` は認証トークンや講師用回答を含まない静的成果物。
React画面には元HTMLのサンプル教材本文とデモ表示データを含む。
既存のWebシェルへ直接組み込む場合は、同じoriginで学習APIとRunner APIを提供し、信頼済みの初期化経路でHTMLの `meta[name="reprokit-session"]` にSessionTokenを設定する。
トークンを配布ファイルやlocalStorageへ保存しない。

## APIと保存

- 教材一覧、詳細、受講状態、段階的ヒント、回答保存は #10 の契約に従う。
  講師用の回答例や評価基準を要求しない。
- 初期対象は `command-injection/filebrowser-auth-hook`。
  カタログで公開されたVersionと、受講セッションが返す `vulnerable` / `fixed` 参照だけを使う。
- Prepareは取得、検査だけ。
  Startは検査済み、外部公開なしを確認し、確認ダイアログ後に `{confirmed:true}` を送る。
- StopとDeleteは別操作。
  削除は対象SessionとVolume方針を再表示し、既定は `retain`。
  削除後も状態取得と残存確認ができる。
- 回答は受講APIに保存する。
  全問保存後、ブラウザはStudy Session IDだけを研修APIへ渡し、Runnerが保存済み回答を提出物へ変換する。
  受講開始も固定されたローカル割り当てへ反映する。
  予想、観察、振り返りはブラウザの下書きに保存する。
  自由記述はLab操作へ転送しない。
- 受講IDとLab IDの対応をブラウザへ保存し、再読み込み後にAPIで状態を取得し直す。
  API一時障害で受講IDを捨てない。
- 自動更新は5秒間隔の状態取得のみ。
  準備、起動、停止、削除は明示操作で行う。
  学習記録を理解度の合格判定として表示しない。
- 管理画面は、公開済み教材、組織内の割り当て、割り当てごとの提出回答を取得する。
  認証済みの組織IDと管理者IDはRunner側で固定し、ブラウザ入力による別組織への切り替えを拒否する。
- 組織管理APIは `GET/POST /api/identity/users`、`PUT /api/identity/users/{id}/roles`、`POST /api/identity/users/{id}/disable`、`GET /api/identity/audit-events` を提供する。
  組織IDは認証セッションから確定し、別組織のIDを受け取らない。ユーザー作成・権限変更・無効化は組織管理者だけに許可し、認証成功・失敗とともに監査ログへ記録する。

## 検証

```sh
npm run web:lint
npm run web:test
npm run web:build
# Chromiumを初回だけ用意する場合
npm run web:browser:install
npm run web:test:e2e
# インストール済みChromeを使う場合
REPROKIT_TEST_BROWSER=chrome npm run web:test:e2e
# 専用環境でのみ、実LabをWeb UIから起動して比較する
npm run web:test:e2e:filebrowser
npm run verify:full
```

lintはJavaScript構文検査。
整形検査はルートの固定Prettierを使う `npm run format:check` が担当する。
全体検証スクリプトは `mapfile` を使うためBash 4以上が必要。
macOS標準Bash 3の場合は、Bash 4以上の `bin/` をPATHの先頭に指定して実行する。

HTTPテストはloopbackの一時サーバーだけを使う。
通常のブラウザテストは #9/#10 と同じ形式の受講者向けレスポンスをテスト内で返し、実際のDockerや脆弱Labを起動しない。
キーボードでの通し操作、明示起動のキャンセル、状態復元、失敗理由、Volume方針、小画面、非同期更新中のメモ保持を検証する。
スクリーンショットはシステム一時ディレクトリ内の `reprokit-web-<worktreeのhash>/learner-desktop.png` と `learner-mobile.png` に生成する。
生成JSONをルートのPrettier検査対象へ混ぜないため、テスト出力をソースツリー外に置く。

2026-10-17の確認結果（全体検証はBash 5.3で実行）：

```text
verify: full checks passed
Web tests: pass 14 / fail 0
Playwright (Chrome): 8 passed
Real Go Runner bootstrap and browser Prepare: PASS
```

Go APIの接続確認では、実際の学習APIとRunner HTTPハンドラーへWebシェル経由で接続した。
固定Lab参照、段階的ヒント、進捗8段階、回答3件の保存、明示操作と再読み込み後の復元を確認した。
実プロセスの起動テストでは本物のRunnerを使い、回答を保存してプロセス再起動後に復元した。
実ブラウザでも固定ソースを取得し、両版の接続先を持つ一つのSessionが安全性検査済みのpreparedになることを確認した。
起動リクエストは送っていない。
起動、停止、削除のHTTPテストでは実Coreと永続ストアを使い、Docker Providerだけを無害なテスト実装へ置き換えた。
Docker統合テストは明示的なオプトインをしていないためスキップした。

検証を追加するときの判断基準：

- 非同期のLab更新では学習本文を再描画しない。
  メモの入力ノードを保持し、遅延した状態取得で新しい操作結果を上書きしないことを確認する。
- Runnerの復元テストは保存ファイルの読み書きだけで済ませず、復元後の明示Startまで検証する。
  Lockfileに加え、Manifestと実行ファイルの参照が保存されていなければ起動時検証に失敗する。
- Host拒否は生のHTTP要求で検証する。
  fetchがHostを正規化する環境では、不正Hostを送ったつもりでも実際には正常な要求になる。

## 依存機能と未検証の範囲

- #10の学習APIと #13の固定Lab定義を含むmainをベースにしている。
  Web起動スクリプトがGoの起動口を呼び、実Runnerと学習APIを接続する。
- #13の比較Labは同じLab ID内に `vulnerable` / `fixed` の両serviceを持つ。
  Runnerが両方の安全な接続先を返した場合だけ両カードを同じSessionへ対応付け、表示する接続先を版ごとに分ける。
  起動、停止、削除が両版に適用されることを画面に明記する。
  一つの固定参照を使う現行 #9 でも準備リクエストを重複させない。
- 比較用の接続先がなければ修正版として推定しない。
  別Sessionの準備がRunnerで許可されない場合は失敗理由を表示する。
- `GET /api/labs/{id}/markers/{vulnerable|fixed}` は実行中のFile Browser比較Labに限定して
  `/tmp/success` を最大512 byte読む。service、パス、コマンドは固定であり、受講者入力をDocker操作へ渡さない。
  UIはこの承認済みの読取りだけで、脆弱版のマーカー作成と修正版での不在を表示する。
- 固定HTTP例はLessonの手順として表示する。
  送信先、コマンドを入力する画面やPoCの自動送信機能はない。
- `test:e2e:filebrowser` は `REPROKIT_RUN_FILEBROWSER_E2E=1` が必要な専用環境用テストである。
  Web UIからのPrepare/Start/Stop/Delete、固定ログイン入力、固定マーカーの脆弱版・修正版比較を行う。
  通常の`test:e2e`や`npm run verify:full`では起動しない。

このため、ブラウザテストの成功は実LabでCVEを再現、修正確認できたという意味ではない。
