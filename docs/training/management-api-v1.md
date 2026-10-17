# 研修管理・受講連携 API v1

Issue #15 のローカルMVPで、研修管理データをWeb UIと受講画面へ接続する契約を定める。
管理プレーンとLab実行の責務は分けたまま、同じloopback Runnerに用途別の認証でマウントする。

## 認証境界

- 管理APIは、起動ごとに生成する `X-Reprokit-Management-Session` を必須とする。この値は親Webシェルだけへ専用パイプで渡し、ブラウザへ公開しない。
- Web管理画面はユーザー名とパスワードでログインし、HttpOnlyかつSameSite=StrictのセッションCookieを受け取る。WebシェルだけがCookieを管理トークンへ交換する。
- 管理APIの `organizationId`、`actorId`、`evaluatorId` は認証済み主体の値にRunnerが固定する。異なる値を明示した要求は拒否する。
- 受講APIは既存のローカル学習セッショントークンを必須とし、起動時に固定した組織、受講者、割り当てだけを操作する。
- どちらのAPIもloopback Hostと同一Originを検証する。

## 管理API

管理画面から利用する主要な取得操作は次の通り。

| メソッド | パス | 結果 |
| --- | --- | --- |
| `GET` | `/api/training/courses` | 認証済み組織のコース版一覧 |
| `GET` | `/api/training/assignments` | 認証済み組織の割り当て一覧 |
| `GET` | `/api/training/assignments/{id}` | 割り当て1件 |
| `GET` | `/api/training/assignments/{id}/submissions` | 割り当てに属する提出回答一覧 |
| `GET` | `/api/training/audit-events` | 監査イベント一覧 |
| `GET` | `/api/training/exports/results.csv` | 回答本文を含まない結果CSV |

コース、コホート、割り当ての作成と提出評価は既存のPOST操作を使う。ブラウザは組織IDや操作主体を選択しない。

## 受講API

| メソッド | パス | 入力 | 結果 |
| --- | --- | --- | --- |
| `GET` | `/api/training/learner/assignment` | なし | 固定された割り当てとコース |
| `POST` | `/api/training/learner/start` | `{}` | 割り当てを受講中へ遷移 |
| `POST` | `/api/training/learner/submissions` | `studySessionId` | 保存済み回答から提出を作成 |

提出APIは回答本文、Lesson ID、受講者IDをブラウザ入力として受け取らない。
RunnerがprivateなStudy Sessionを読み、割り当てに含まれる固定Lesson版であることを研修Serviceでも検証してから提出へ保存する。
Labの準備、起動、観察結果は提出や修了判定へ含めない。
