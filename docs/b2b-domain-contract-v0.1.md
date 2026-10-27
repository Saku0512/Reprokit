# Reprokit 法人研修ドメイン契約 v0.1

> **ステータス：レビュー待ち（2026-10-17）**  
> 対象Issue：[#5 ToB研修MVPのドメイン契約とコード所有境界を確定する](https://github.com/jphacks/sd_2601/issues/5)  
> 本書は、法人向け研修MVPのコンポーネント間の契約を定める。Lesson SchemaやHTTP APIの完全仕様ではなく、後続実装が互いの内部実装へ依存しないための最小境界を正本とする。2026-10-27以降、開発・保守は@Saku0512が担当し、以前の担当者によるレビューや承認は必要としない。

## 変更提案：Reprokit管理Labへの移行

> **移行実施中（2026-10-27）**。共有契約とCompose案のレビュー承認後、File Browser LabをReprokit管理へ切り替えた。旧Sessionと研修記録の扱いは下記を参照する。

- 実行時にReprokit Labのリポジトリ、イメージ、Composeを取得せず、Reprokit管理のGitコミットとDocker Hub上のdigest固定イメージを使う。Reprokit Labは出典・ライセンス情報として保持する。
- 新しいFile Browser Lab IDは`reprokit/filebrowser-cve-2026-54088`とする。LessonのLab ID変更には新しいLesson Versionを発行し、UIから渡す入力を任意のGit URL、パス、イメージ、コマンドへ拡張しない。
- 既存AssignmentとSubmissionは元のLesson Versionに固定されたままとし、履歴を黙って書き換えたり削除したりしない。現行カタログはv1.1.0を提供し、v1.0.0の受講・再実行は提供しない。旧割り当てを新教材に自動移行せず、新しい割り当てを作成する。
- 新Runnerは旧Lab Sessionの再起動を拒否する。旧Sessionの停止・削除に必要な処理は残し、Sessionや研修データを自動削除しない。既存の教材版は新しいLabへ暗黙に書き換えない。
- 新規LabのManifest・Lockfileは取得元のGit SHA、Composeハッシュ、イメージdigestを固定し、PrepareとStartの安全性検査を維持する。構成、イメージ、出典およびライセンスを人がレビューするまで新Labを実行可能なRegistryに登録しない。

影響範囲はRegistry、Manifest、Runnerの固定Lab判定、LessonのLab参照・バージョン、受講・研修の保存データ、およびローカルAPIのLab識別子である。具体的な実行側の段階案は[Lab昇格設計](lab-catalog-runtime-v0.1.md)を参照する。

## 1. 設計原則

1. **学習と実行を分離する。** Lessonは学習内容、Labは再現環境を表す。
2. **管理状態と実行状態を分離する。** 管理プレーンは研修を管理し、RunnerはローカルLabの実状態を管理する。
3. **参照は固定する。** AssignmentとLab Sessionは利用開始時のLesson版を保持し、公開中の更新へ暗黙追従しない。
4. **ブラウザに実行権限を渡さない。** UIは許可済みのLessonとLab IDだけをローカルAPIへ渡し、任意コマンド、Compose、URL、パスを渡さない。
5. **実行前に検査する。** Labの取得は起動を意味せず、固定リビジョンの解決、安全性検査、明示確認後にだけ起動する。
6. **CVE IDを学習の主キーにしない。** CVEは根拠情報であり、Lesson IDとLab IDは製品内の安定識別子を使う。
7. **提出物で理解を確認する。** 細かな操作ログを、原因・修正・再発防止の説明に代わる評価材料にしない。

## 2. 境界と責務

```text
Management Plane                         Local Execution Plane
┌──────────────────────────┐            ┌──────────────────────────┐
│ Organization / Lesson     │            │ Reprokit Local Server    │
│ Assignment / Submission   │            │ Runner / Go Core         │
│ Completion / Audit        │            │ Lab Session / Lockfile   │
│ Published Lesson metadata │            │ Docker Compose Runtime   │
└─────────────┬────────────┘            └─────────────┬────────────┘
              │ versioned Lesson package              │ approved Lab only
              └──────────────────────┬─────────────────┘
                                     ▼
                              Learner Web UI
```

### 2.1 管理プレーン

管理プレーンは次を正本として持つ。

- Organizationとユーザー所属
- 公開済みLesson Version
- Assignmentの教材、対象、期限、完了条件
- SubmissionとCompletion
- Lessonの公開状態とレビュー履歴
- 研修管理上の監査イベント

管理プレーンは、コンテナの実行状態、ホスト上のパス、Dockerリソース、認証情報を正本として持たない。

### 2.2 ローカル実行プレーン

ローカル実行プレーンは次を正本として持つ。

- 承認済みLabの取得状態
- 解決済みManifestとLockfile
- 安全性検査結果
- Lab Sessionの実行状態
- Docker Compose project、コンテナ、ネットワーク、Volume
- ローカルに公開した接続先とポート

RunnerはAssignment、評価結果を管理しない。Runnerが返す状態は研修の完了を意味しない。

### 2.3 Lessonパッケージ

Lessonパッケージは管理プレーンとローカル実行プレーンを接続する版管理された入力である。本文、学習手順、確認課題、根拠情報と、承認済みLabへの論理参照を含む。Composeファイルや任意の実行コマンドをUIからRunnerへ転送する用途には使わない。

## 3. ドメイン用語

| 用語 | 責務 | 安定識別子 | 版固定 |
| --- | --- | --- | --- |
| Organization | 法人単位のテナント境界 | `organizationId` | しない |
| Lesson | 実在事例を学ぶ教材の論理的なまとまり | `lessonId` | `lessonVersion` |
| Lab | 脆弱版または修正版の再現環境 | Manifestの`metadata.id` | Manifest SHA、source revision、image digestをLockfileで固定 |
| Assignment | Lessonを対象者へ割り当てた記録 | `assignmentId` | 作成時の`lessonVersion`を固定 |
| Submission | 受講者が提出した回答 | `submissionId` | 対象`assignmentId`と`lessonVersion`を保持 |
| Completion | 完了条件を満たした判定記録 | `completionId` | 判定時のAssignmentとSubmissionを参照 |
| Lab Session | 受講者端末上の一時的なLab実行単位 | `labSessionId` | `lessonVersion`、Lab ID、Lockfileを固定 |

### 3.1 Organization

法人単位のデータ境界を表す。MVPが単一組織であっても、管理データには将来の組織分離を妨げない`organizationId`を持たせる。Lab SessionのDocker内部データをOrganizationへアップロードすることは既定動作にしない。

### 3.2 Lesson

学習目標、前提知識、実在事例、手順、観察点、原因、修正、再発防止、確認課題を持つ。CVE IDや原典は証拠情報として保持するが、CVE IDをLesson IDや表示名に使うことを必須にしない。

### 3.3 Lab

再現環境だけを表し、教材本文や受講状態を持たない。既存の`reprokit.dev/v1alpha1` Lab Manifestを正本とし、`metadata.id`を論理IDとして参照する。具体的なGit SHA、Composeハッシュ、image digestは解決後のLockfileで固定する。

### 3.4 Assignment

あるLesson Versionを特定の受講者またはグループへ割り当てる。期限と再受講可否を保持し、その教材の提出回答が承認されたときに完了する。受講開始後にLessonを更新する場合は、新しいAssignmentまたは明示的な移行操作を必要とする。

### 3.5 Submission

受講者が提出した確認課題の回答を保持する。Labのコンテナログ、シェル履歴、ホスト上のファイルをSubmissionへ自動添付しない。

### 3.6 Completion

Assignmentの完了判定を表す。Labを起動した、PoCの期待結果が得られた、技術的なBootable/Verified判定に成功した、という事実だけではCompletionにしない。

### 3.7 Lab Session

ローカル端末または企業管理の隔離環境上で動く一時的な実行単位。1つのLab Sessionは、1つのLesson Version、1つのLab参照、1つの解決済みLockfileに結び付く。

## 4. エンティティ間の関係

```text
Organization
├─ User / Group
├─ Lesson Version ── references ──> Lab
└─ Assignment ── pins ──> Lesson Version
   ├─ Submission ── answers ──> Lesson Version
   └─ Completion ── evaluated from ──> Submission

Learner device
└─ Lab Session ── pins ──> Lesson Version + Lab ID + Lockfile
```

Labは複数Lessonから参照できる。Lessonは、役割が異なる複数Labを参照できる。初期用途では`vulnerable`と`fixed`を使用する。

## 5. LessonからLabを参照する契約

### 5.1 論理形式

Lesson Schemaは[#7](https://github.com/jphacks/sd_2601/issues/7)で確定するが、Lab参照は少なくとも次の意味を持たなければならない。

```yaml
apiVersion: reprokit.dev/v1alpha1
kind: Lesson
metadata:
  id: filebrowser/hook-authentication-command-injection
  version: 1.0.0
  title: Hook Authenticationの認証不備によりコマンドが実行される
spec:
  evidence:
    cves:
      - CVE-2026-54088
    references:
      - type: vendor-advisory
        url: https://example.invalid/advisory
  labs:
    - ref: vulnerable
      labId: reprokit/filebrowser-cve-2026-54088
      purpose: reproduce
    - ref: fixed
      labId: reprokit/filebrowser-cve-2026-54088-fixed
      purpose: compare-fix
```

この例は最終Schemaではなく、後続実装が保持すべき意味契約を示す。

### 5.2 必須規則

- `metadata.id`と`metadata.version`の組でLesson Versionを一意にする。
- `labs[].ref`はLesson内で一意とし、UIと学習手順はこの論理名を使う。
- `labs[].labId`はRegistryにあるLab Manifestの`metadata.id`を指す。
- LessonにGit URL、ホストパス、Composeパス、image tagを直接記載しない。
- Labを使う学習ステップは、存在する`labs[].ref`だけを参照する。
- Runnerは`labId`をRegistryから解決し、ManifestとLockfileの検証を必ず通す。
- Lesson更新時にLab参照を変えた場合は、新しいLesson Versionとして公開する。
- 脆弱版と修正版の同時起動または切り替え方式はRunner側が決め、LessonはRuntime内部へ依存しない。

## 6. ローカルWeb UIとRunner APIの最小インターフェース

Web UIはDockerを直接操作しない。`127.0.0.1`に限定して待ち受けるRunner APIへ、許可済みの操作だけを送る。CLIによるLab操作は提供しない。HTTPのパスやJSON Schemaの最終形は実装Issueで確定する。本節は、Web UIが利用できる操作と渡してよい値を定める。

### 6.1 操作一覧

| 操作 | 入力 | 主な出力 | 状態変更 |
| --- | --- | --- | --- |
| ListLessons | なし、または教材フィルター | 公開Lessonの概要 | なし |
| GetLesson | `lessonId`、`lessonVersion` | Lesson本文とLab論理参照 | なし |
| PrepareLab | `lessonId`、`lessonVersion`、`labRef` | `labSessionId`、取得・検査結果 | `absent`から`prepared` |
| GetLabStatus | `labSessionId` | 状態、接続先、安全性概要、失敗情報 | なし |
| StartLab | `labSessionId`、明示確認 | 起動後の状態 | `prepared`から`running` |
| StopLab | `labSessionId` | 停止後の状態 | `running`から`stopped` |
| DeleteLab | `labSessionId`、Volume削除方針 | 削除結果、残存リソース | 任意状態から`deleted` |

### 6.2 許可する入力

- 公開済みの`lessonId`
- 固定済みの`lessonVersion`
- Lesson内で宣言された`labRef`
- Runnerが発行した`labSessionId`
- 定義済みの明示確認値
- 定義済みのVolume削除方針

### 6.3 禁止する入力

- 任意のCVE IDを実行対象として直接渡すこと
- 任意のGit URL、ブランチ、コミットを渡すこと
- 任意のComposeファイル、ホストパス、imageを渡すこと
- 任意のシェルコマンド、PoCコマンド、環境変数を渡すこと
- 任意のポートや外部公開アドレスを指定すること
- 第三者のホスト名やURLを攻撃対象として渡すこと

### 6.4 最小レスポンス

Runnerは少なくとも次をUIへ返す。

```json
{
  "labSessionId": "01J...",
  "lesson": {
    "id": "filebrowser/hook-authentication-command-injection",
    "version": "1.0.0",
    "labRef": "vulnerable"
  },
  "lab": {
    "id": "reprokit/filebrowser-cve-2026-54088",
    "sourceRevision": "0123456789abcdef0123456789abcdef01234567"
  },
  "state": "prepared",
  "safety": {
    "checked": true,
    "externalExposure": false
  },
  "guardrails": {
    "maxRuntimeSeconds": 1800,
    "cpuCores": 1,
    "memoryMiB": 512,
    "pids": 128,
    "diskMiB": 1024,
    "egress": "none"
  },
  "endpoints": [],
  "failure": null
}
```

`endpoints`はRunnerが決定したループバック接続先だけを含む。内部ファイルパス、Docker認証情報、セッショントークンを返さない。`guardrails`は固定済みLabの実行上限を表示するための値であり、UIから変更できない。running時には`expiresAt`で自動停止時刻を、停止後には`stopReason`で理由（例: `manual`、`expired`、`runner_restarted`）を返す。

## 7. Lab Sessionの状態遷移

```text
absent
  │ PrepareLab
  ▼
preparing ── failure ──> error
  │ resolve + fetch + lock + safety check
  ▼
prepared
  │ StartLab
  ▼
starting ─── failure ──> error
  │
  ▼
running ── StopLab ──> stopping ──> stopped
  │                                      │
  └────────────── DeleteLab ─────────────┘
                         ▼
                      deleting ──> deleted
```

- `PrepareLab`はコンテナを起動しない。
- `StartLab`は`prepared`だけから開始し、安全性検査を再確認する。
- `StopLab`と`DeleteLab`は可能な範囲で冪等にする。
- `error`は失敗した操作、利用者向けメッセージ、再試行可能性を保持する。
- `deleted`後に再利用する場合は、新しいLab Sessionを作る。
- 学習進捗とLab Session状態は独立させる。Labを削除してもSubmissionは削除しない。

## 8. 受講状態の最小契約

### 8.1 Assignment状態

```text
assigned -> in_progress -> submitted -> completed
                          └───────────> needs_retry
```

- 初回の教材閲覧または明示的な開始で`in_progress`になる。
- 必須回答の提出で`submitted`になる。
- 評価基準を満たしたときだけ`completed`になる。
- Lab起動、停止、技術検証の成功だけで`completed`にしない。

### 8.2 Submissionの最小データ

- `submissionId`
- `organizationId`
- `assignmentId`
- `learnerId`
- `lessonId`と`lessonVersion`
- 課題ごとの回答
- 提出日時
- 評価状態と評価者（評価済みの場合）

PoCの生ログ、シェル履歴、Lab内のデータは既定では含めない。

## 9. セキュリティ境界

- ローカルHTTP APIは`127.0.0.1`だけでListenする。
- Host、Origin、CSRF、ローカルセッションを検証する。
- Web UIはRunner APIを介して同じGo Coreの安全性検査を利用する。別のCLI操作経路は提供しない。
- Manifestの検証、Lockfileの整合性検査、Compose許可リストを迂回するAPIを作らない。
- RunnerはDockerを完全な隔離境界として説明しない。
- 管理プレーンへDocker socket、レジストリ認証、ホストパス、Lab内の機密情報を送信しない。
- AIによるCVE取り込みは本契約の外側にあり、承認済みLabへ昇格するまでRunnerから参照できない。

## 10. コードの責務と保守

@Saku0512がCore / Runner、Learning / Training、共有領域を含むリポジトリ全体を保守する。以下はコンポーネントの責務を示すもので、別担当者によるレビューを要求するものではない。

### 10.1 Core / Runner

主な所有範囲：

- `backend/cmd/reprokit/`（ローカルWeb Runnerの起動のみ。Lab操作CLIは持たない）
- `backend/internal/lab/`
- `backend/internal/runtime/`
- `backend/internal/security/`
- `backend/internal/manifest/`
- `backend/internal/lockfile/`
- `backend/internal/registry/`
- `registry/`
- `schemas/manifest-*`
- `schemas/lock-*`
- Lab/Runtime/Securityに直接関係するテストと文書

### 10.2 Learning / Training

主な所有範囲：

- `web/`
- `lessons/`
- `backend/internal/lesson/`
- `schemas/lesson-*`
- `docs/training/`
- Lesson、受講体験、研修ガイドに直接関係するテストと文書

### 10.3 共有領域

次は複数コンポーネントに影響する領域とし、変更時に互換性と影響範囲を確認する。

- `backend/go.mod`、`backend/go.sum`
- `README.md`
- `backend/cmd/reprokit/`の公開サブコマンド構成
- ローカルHTTP APIの公開契約
- `backend/internal/training/`
- `docs/b2b-domain-contract-v0.1.md`
- CI設定
- リポジトリ全体へ影響するビルド設定

## 11. 開発ルール

1. **1 Issue 1ブランチ**とし、ブランチ名にIssue番号を含める。
2. 変更は目的ごとに小さく分け、無関係な作業を混ぜない。
3. 共有領域を変更する場合は、必要に応じて機能実装とは別の小さなPRに分離する。
4. 契約変更時は本書または対応Schemaを更新し、互換性と移行への影響を記録する。
5. Lesson側はRunner内部パッケージへ直接依存せず、公開インターフェースだけを使う。
6. Runner側はLesson本文やUI構造を解釈せず、解決済みのLab参照だけを受け取る。

### 推奨ブランチ名

- `feat/<issue>-<short-description>`
- `fix/<issue>-<short-description>`
- `docs/<issue>-<short-description>`

## 12. 変更管理

本契約を変更するPRは、次を記載する。

- 変更する契約と理由
- 影響を受けるIssue、Schema、API、保存データ
- 後方互換性の有無
- 移行方法
- 実施した検証と未確認事項

破壊的変更では、Lesson Version、Lab Manifest、Lockfile、保存済みSubmissionのどれに影響するかを明記する。

## 13. 後続Issueへの決定事項

| Issue | 本書で固定する入力 | 後続Issueで決めること |
| --- | --- | --- |
| #6 Lab Resolver | LessonはLab IDだけを参照し、Runnerが固定リビジョンへ解決する | 取得・キャッシュ・Lockfile生成の実装 |
| #7 Lesson Schema | LessonとLabを分離し、論理`labRef`からLab IDを参照する | JSON Schemaの厳密な形状 |
| #8 Security | Start前に必ず安全性検査を通す | Compose許可・拒否ルールのコード |
| #9 Runtime | PrepareとStartを分離し、状態遷移を守る | Docker Compose操作の実装 |
| #10 Lesson API | UIは公開Lesson Versionを取得する | 保存方式とHTTPパスの詳細 |
| #11 Learner UI | 任意実行入力を持たず、定義済み操作だけを呼ぶ | UIフレームワークと画面実装 |
| #13 Compare | Lessonは`vulnerable`、`fixed`等の論理参照を使う | 同時起動か切り替えかの実装 |
| #15 Training API | AssignmentがLesson Versionを固定し、Submissionで理解を記録する | 永続化とAPIの詳細 |

## 14. レビュー項目

- [ ] 用語と責務に重複または空白がない
- [ ] LessonからLabへの参照が既存Manifest/Registryと両立する
- [ ] UIが任意の実行入力をRunnerへ渡せない
- [ ] Runnerの状態と研修完了状態が分離されている
- [ ] コンポーネントの責務と変更の影響範囲が明確である
- [ ] 共有領域の変更手順が明記されている
