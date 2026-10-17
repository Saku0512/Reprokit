# ローカル学習 API v1

Issue #10 の Lesson カタログと受講状態 API。
`backend/internal/lesson` の `NewLocalServer` が、#9 の Runner と同じ loopback listener・セッショントークンで提供する。
Lab Resolver / Runtime と既存の `/api/labs` 契約は変更しない。

## 組み込み

サーバー側で固定した教材ディレクトリを `LoadCatalog(root, allowed)` へ渡す。
`allowed` は Registry のレビュー済み Lab ID だけを許可する必須の関数であり、HTTP 入力から生成しない。
教材ディレクトリへの配置は公開操作として扱い、講師がレビューした教材だけを配置する。
リポジトリの `lessons/` にある草稿を無条件で公開しない。
この API 自体は教材レビューや公開承認を自動化しない。

読み込みは直下の `.json` だけを対象とし、Schema 相当の型・必須キー・追加キー・参照整合性を検証する。
重複キー、同じ Lesson ID / Version の重複、未承認 Lab、壊れた教材、1 MiB を超える教材、シンボリックリンクは公開せず、除外理由を呼び出し元へ返す。
Runner Prepare の JSON 本文が #9 の4096 byte上限に収まらない論理参照も公開しない。
同一 ID / Version の重複は両方とも除外する。
ディレクトリ自体を読めない場合は初期化に失敗する。
一覧は ID、Version の辞書順で固定され、ステップは教材の配列順を維持する。
教材を更新する場合は新しい Version として配置し、再起動時も受講中の旧 Version を残す。

`NewSessionStore(stateDirectory, catalog)` で保存先を初期化する。
保存先は専用ディレクトリ（`0700`）とし、既存ディレクトリが他ユーザーへ公開されていれば初期化を拒否する。
受講状態は `0600` の JSON を一時ファイルに書き、同期後に rename で置き換える。
一つの保存先に一つの SessionStore を使う。
受講 Session ID はランダムな32桁の16進数で、Runner の Lab Session ID とは別物である。
UI は開始時に返された受講 Session ID を保持し、同じ ID で再開する。
この保存はローカルの学習状態であり、法人向けの評価・修了認定ではない。

`NewLocalServer(catalog, store, runnerService)` には #9 の `runnerapi.Service` を渡す。
返された `Listener` と `Handler` を `http.Server.Serve` に渡し、`SessionToken` は信頼する Web シェルの初期化経路でだけ共有する。
既存 listener に組み込む場合は `NewHandler(catalog, store, runnerHandler, token)` を使える。
Runner と学習 API のトークンは同じ値を指定する。
別のポートへの CORS 接続や、HTTP によるトークン取得 API は提供しない。

## 共通条件

すべてのリクエストに `X-Reprokit-Session` が必要（GET を含む）。
Host は `127.0.0.1` に限定し、Origin があれば同じ HTTP origin であることを確認する。
全レスポンスに `Cache-Control: no-store` を付ける。
JSON 本文は `Content-Type: application/json` とし、追加キー、必須キー不足、null、重複キー、複数 JSON 値を拒否する。
未定義の query parameter と同名 parameter の重複も拒否する。

| 結果 | HTTP |
| --- | --- |
| 取得・更新成功 | 200 |
| 受講セッション作成 | 201 |
| 不正な入力・存在しないステップ/質問/論理 Lab 参照 | 400 |
| トークンなし・不一致 | 401 |
| 非 loopback Host・別 Origin | 403 |
| 未公開 Lesson Version・存在しない受講 Session | 404 |
| 許可しない HTTP method | 405 |
| 保存先の破損・入出力失敗 | 500（内部パスは返さない） |

## 教材と学習操作

| Method | Path | 入力と応答 |
| --- | --- | --- |
| GET | `/api/lessons` | 概要配列（id、version、title、audience、durationMinutes、product） |
| GET | `/api/lessons/detail?lessonId=...&lessonVersion=...` | 版固定の教材詳細 |
| POST | `/api/study-sessions` | `{"lessonId":"command-injection/filebrowser-auth-hook","lessonVersion":"1.0.0"}` → 受講状態 |
| GET | `/api/study-sessions/{id}` | 保存済み受講状態を復元 |
| POST | `/api/study-sessions/{id}/advance` | 本文なし、または `{}`。StepIndex を一つ進める |
| POST | `/api/study-sessions/{id}/hints/{stepId}` | 本文なし、または `{}`。ヒントを一つ開示 |
| PUT | `/api/study-sessions/{id}/answers/{questionId}` | `{"answer":"受講者の回答"}`。既存回答を上書き保存 |
| GET | `/api/study-sessions/{id}/labs/{labRef}` | 当該 Lesson に固定された Runner Prepare 用の参照を取得（実行しない） |

教材詳細は元 Lesson と同じ `apiVersion` / `kind` / `metadata` と、受講者用の `spec` を返す。
`steps[].hints` は返さず `steps[].hintCount` を返す。
`assessment[]` は `id` と `question` だけを返し、`answerExample` と `criteria` は配信しない。
本文、期待結果、観察点、原因・修正・比較・再発防止は教材詳細に含む。

受講状態は以下の形式。

```json
{
  "id": "0123456789abcdef0123456789abcdef",
  "lessonId": "command-injection/filebrowser-auth-hook",
  "lessonVersion": "1.0.0",
  "stepIndex": 0,
  "revealedHints": {},
  "answers": {},
  "hints": {}
}
```

`stepIndex` は0始まりで、全ステップを進めるとステップ数と等しくなる。
前のステップを画面で読み直すことと、保存される到達位置は独立して扱える。
ヒントは到達済みのステップだけで開け、1リクエストにつき一つ増える。
`revealedHints` は開示数、`hints` は開示済み本文の配列を step ID ごとに保持する。
戻る操作やサーバー再起動で開示済みヒントを失わない。
回答は質問ごとに UTF-8 の16 KiB以下、受講状態ファイル全体は1 MiB以下とする。
保存に失敗した場合は成功レスポンスを返さない。

## Runner への接続

受講中 Lesson の論理 Lab 参照取得は、次の3フィールドだけを返す。

```json
{
  "lessonId": "command-injection/filebrowser-auth-hook",
  "lessonVersion": "1.0.0",
  "labRef": "vulnerable"
}
```

UI はこの値を `POST /api/labs` へ送り、準備結果の Runner 発行 `labSessionId` を以後の操作に使う。
準備ではカタログに公開された Lesson Version と宣言済み `labRef` を再検証する。
準備操作は起動しない。
Runner が設定済みの参照と一致するかの最終判定は #9 が担う。
#9 の `LifecycleService` が保持できるのは一つの固定参照であり、別版 Lab の追加や切り替えはこの API で暗黙に行わない。

| Method | Path | 許可する入力 |
| --- | --- | --- |
| POST | `/api/labs` | 公開 Lesson の lessonId、lessonVersion、labRef のみ |
| GET | `/api/labs/{labSessionId}` | Runner 発行 ID のみ |
| POST | `/api/labs/{labSessionId}/start` | `{"confirmed":true}` のみ |
| POST | `/api/labs/{labSessionId}/stop` | 本文なし、または `{}` |
| DELETE | `/api/labs/{labSessionId}?volumePolicy=retain` | `retain` または `delete`。本文なし、または `{}` |

ライフサイクルの状態・接続先・安全性結果・失敗情報・削除後の残存は #9 のレスポンスをそのまま返す。
未発行の ID に対する存在確認と拒否も Runner が行う。
任意の Compose、Docker 引数、ホストパス、image、URL、コマンド、Lab ID を受け取る操作はない。
自由記述の回答は受講状態へだけ保存し、Runner へ転送しない。
