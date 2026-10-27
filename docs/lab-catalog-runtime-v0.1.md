# Vulhub fork連携・Lab昇格設計 v0.1

> **移行案・レビュー待ち（2026-10-27）**: File Browserの実行時Vulhub依存をなくすため、Reprokitリポジトリの固定コミットにある`images/CVE-2026-54088/docker-compose.yml`とDocker Hubの固定digestをLabの取得元にする。現行RegistryはまだVulhub forkを参照する。提案する変更と承認手順は本書末尾を参照。

> 決定日: 2026-10-17。`Saku0512/vulhub`をReprokitのLabソースカタログとして使用し、レビュー済みの構成だけを固定イメージへ昇格して実行する。

## 1. この設計で決めること

ReprokitはVulhub forkの`master`やDockerfileを利用者の端末で直接実行しない。役割を次の4層に分離する。

| 層 | 正本 | 責務 |
| --- | --- | --- |
| Labソース | `Saku0512/vulhub` | Dockerfile、元Compose、README、脆弱版と修正版の再現材料 |
| イメージ昇格 | forkのGitHub Actions + GHCR | 固定コミットからレビュー済みイメージをビルドし、公開digestを確定 |
| Reprokit Registry | `jphacks/sd_2601` | 採用Lab、fork SHA、Compose、対応CVEを固定 |
| Lesson | `jphacks/sd_2601` | 初学者向けの処理経路、操作、観察、修正比較を定義 |

2026-10-17に確認したforkのスナップショットは`a94285e057faee8c7232e100116dc750cf17e257`。これは設計時点の観察値であり、利用するスナップショットはRegistryのManifest更新時に明示的に確定する。

## 2. 現在のFile Browser Lab

`filebrowser/CVE-2026-54088`には次が存在する。

- File Browser 2.63.5の脆弱版イメージ定義
- File Browser 2.63.6の修正版イメージ定義
- Hook Authenticationを有効にするentrypoint
- loopback限定のComposeポート
- 無害な`/tmp/success`マーカーによるPoC
- 脆弱版と修正版の比較手順

ただし、元ComposeはReprokit v0.1ではそのまま実行しない。

- `profiles`はRuntimeの許可リスト外。
- `vulhub/filebrowser:2.63.5`と`vulhub/filebrowser:2.63.6`はReprokitが管理する公開イメージではない。
- 元Compose内のタグはLockfile生成時にdigestへ解決する必要がある。

したがって「forkへLabが存在する状態」と「Reprokitで実行可能な状態」を分ける。

### 2026-10-17の昇格結果

固定ソース`a94285e057faee8c7232e100116dc750cf17e257`から、次のlinux/amd64イメージを一度だけビルドしてGHCRへ公開した。両方とも匿名pull可能であることを確認済み。

| 役割 | 不変参照 |
| --- | --- |
| 脆弱版2.63.5 | `ghcr.io/saku0512/reprokit-filebrowser@sha256:49ef35c535495a5eb9443690dc171e3b1fca878ac830ee681f3209f3aa18cbdf` |
| 修正版2.63.6 | `ghcr.io/saku0512/reprokit-filebrowser@sha256:b297fdc439d384e88e7a29b619fdc74126c78284188016bc1e26b4dc2a16b47b` |

公開に使用した手動実行workflowは成功確認後にforkの`master`から削除済み。イメージはGHCRに保持し、通常のpushで再公開されるCIは残していない。

Vulhub upstreamへは[CVE-2026-54088環境のPR #805](https://github.com/vulhub/vulhub/pull/805)を提出済み。upstreamへのマージや`vulhub/*`イメージ公開はReprokitの前提にしない。

## 3. Lab昇格パイプライン

### 3.1 入力

- forkの40文字コミットSHA
- 採用Labのディレクトリ
- 使用するDockerfileと参照ファイル
- 対象platform（v0.1は`linux/amd64`を先行）

### 3.2 CI処理

1. 許可されたforkコミットをcheckoutする。
2. 対象Dockerfileと親イメージdigestを記録する。
3. forkのGitHub Actions内でイメージをビルドする。
4. 最小限の起動確認と、教材用の非破壊的な回帰確認を行う。
5. 成功したイメージをGHCRへ公開する。
6. platform別digest、ソースSHA、Dockerfile SHA-256を成果物として出力する。
7. RegistryのManifest/Lock生成に使う更新情報を作る。

初期の公開名は次の形式とする。

```text
ghcr.io/saku0512/reprokit-filebrowser:2.63.5-<source-short-sha>
ghcr.io/saku0512/reprokit-filebrowser:2.63.6-<source-short-sha>
ghcr.io/saku0512/reprokit-httpd-cve-2021-41773:<source-short-sha>
```

タグは人間向けの表示にだけ使い、Reprokitの`resolved.compose.yaml`は必ず`@sha256:...`で参照する。

### 3.3 昇格条件

- ビルド元がforkの固定SHAである。
- ベースイメージがdigest固定されている。
- イメージが公開pull可能である。
- 対象platformのmanifest digestを取得できる。
- コンテナ起動時にホストのbind mount、特権モード、Docker socketを要求しない。
- PoCが外部サービスや第三者ホストへ通信しない。
- 検証結果と限界を記録する。

## 4. Reprokit用Composeアダプター

アダプターはfork内の各Labディレクトリへ`reprokit.compose.yml`として置く。元のVulhub Composeは出典として保持し、上書きしない。

File Browserは脆弱版と修正版を同時に比較できる1つのLabとする。`profiles`は使わず、異なるloopbackポートを割り当てる。

```yaml
services:
  vulnerable:
    image: ghcr.io/saku0512/reprokit-filebrowser:2.63.5-<source-short-sha>
    ports:
      - "127.0.0.1:8080:80"
  fixed:
    image: ghcr.io/saku0512/reprokit-filebrowser:2.63.6-<source-short-sha>
    ports:
      - "127.0.0.1:8081:80"
```

`create`時に両イメージをplatform別digestへ解決し、実効Composeへ書き換える。ホストポートが使用中なら自動変更せず、利用者へ競合を表示する。

Apache HTTP Serverは元Composeの`build: .`を実行しない。fork CIが環境DockerfileをビルドしたGHCRイメージを、同じ形式の`reprokit.compose.yml`から参照する。

## 5. Registryの固定方法

クライアントはforkの`master`を解決しない。Registryを更新する保守工程だけがforkの採用SHAを決め、Manifestへ40文字SHAを書き込む。

```yaml
spec:
  source:
    type: git
    url: https://github.com/Saku0512/vulhub.git
    revision: a94285e057faee8c7232e100116dc750cf17e257
    subdirectory: filebrowser/CVE-2026-54088
  runtime:
    type: docker-compose
    file: reprokit.compose.yml
    platform: linux/amd64
```

更新単位は次のとおり。

1. forkの採用SHAを決定する。
2. そのSHAからイメージを昇格する。
3. 公開digestを確認する。
4. 同じSHAを初期2教材のManifestへ反映する。
5. Registry indexを再生成し、CIでSchema・意味検証する。
6. `jphacks/sd_2601`のPRをレビューしてマージする。

これにより、Registryの途中取得中にforkの`master`が進んでも、異なる版のLabが混ざらない。

## 6. LessonとLabの境界

Lessonは「なぜ起きるか」を教え、Labは「どの環境を動かすか」を定義する。Lesson本文へ任意の実行可能シェルを埋め込まない。

File Browser教材は次の経路を正本とする。

1. **入力**: 未認証の`POST /api/login`でusername/passwordを受け取る。
2. **保持**: Hook Authenticationが資格情報を保持する。
3. **展開**: 2.63.5が`os.Expand`で値をフック引数へ展開する。
4. **信頼境界**: HTTP入力がプロセス起動の引数へ移る。
5. **sink**: 管理者設定が`sh -c`等を使うと、展開値がシェル構文として解釈される。
6. **観察**: `/tmp/success`のような隔離コンテナ内の無害なマーカーで到達を確認する。
7. **修正比較**: 2.63.6では資格情報をコマンド文字列へ展開せず、環境変数として渡す。
8. **片付け**: Labを停止し、必要ならVolumeを明示削除する。

各ステップは少なくとも次を持つ。

```yaml
id: expand-credentials
stage: transform
title: 資格情報がフック引数へ展開される
component: File Browser 2.63.5
input: username / password
output: expanded hook arguments
trust: untrusted-to-process
explanation: 外部入力がコマンド引数へ移る地点を説明する
hintLevel: 1
```

Web UIはステップを1件ずつ表示し、「予想する → 手動で試す → 観察する → 解説を見る」の順を守る。ブラウザから任意コマンドを自動実行しない。

## 7. CoreとWeb UIの責務

```text
Web UI
  ├─ Lesson表示・進行・ヒント
  └─ 許可済みLab操作のみ
          ↓ localhost API
Go Core
  ├─ Registry / Manifest検証
  ├─ fork固定SHAの取得
  ├─ Compose許可リスト検査
  ├─ image digest解決・Lockfile生成
  └─ Docker Compose lifecycle
          ↓
127.0.0.1限定のローカルLab
```

Web APIは任意のイメージ名、Composeパス、シェル文字列を受け取らない。入力はRegistryに存在するLesson ID、Lab ID、Instance IDだけに限定する。

## 8. 状態モデル

```text
Indexed → Resolved → Ready → Running → Stopped
             │          │        │
             └─Error────┴─Error──┘
```

- `Indexed`: Registryに登録済み。
- `Resolved`: fork SHA、Compose、イメージdigestを解決済み。
- `Ready`: Lockfileと実効Composeの検査済み。まだ起動していない。
- `Running`: 利用者の明示操作で起動中。
- `Stopped`: Projectを停止済み。
- 教材の「観察できた」は学習進捗であり、Runtime状態や`Verified`と混同しない。

## 9. 実装順序

1. **完了**：固定ソースからFile Browser 2.63.5/2.63.6をGHCRへ一度だけ公開し、匿名pull可能なdigestを確定する。
2. **次に実施**：`filebrowser/CVE-2026-54088/reprokit.compose.yml`を追加する。
3. File BrowserのManifestとRegistry indexを追加する。
4. Git取得、Compose検査、digest解決、Lockfile生成を実装する。
5. 明示起動・停止・削除を実装する。
6. File Browserの経路型LessonとWeb UIを縦に完成させる。
7. 同じパイプラインでApache HTTP Serverの事前ビルドイメージを昇格する。
8. CVE-2021-41773教材を追加し、初期2教材を完走テストする。

forkに追加済みのその他のCVEは将来の候補として保持するが、初期MVPのUIには自動掲載しない。RegistryとLessonのレビューを通過したものだけを公開教材として扱う。

## 10. 完了条件

- File Browserの脆弱版・修正版イメージが認証なしでpullでき、platform別digestを解決できる。
- Reprokit用Composeがv0.1許可リストを通過する。
- Manifestがforkの固定SHAと`reprokit.compose.yml`を参照する。
- `create`が起動せずにLockfileと実効Composeを生成する。
- `start`がdigest固定イメージだけをloopbackへ公開する。
- 同じ入力を2.63.5と2.63.6へ送り、教材上で結果の違いを説明できる。
- PoCの成功と失敗を誇張せず、実行結果と未検証事項を区別して表示する。
- 停止・削除までWeb UIから完走できる。

## 11. Reprokit内の固定Labへの移行案

本節は共有契約・安全性レビュー待ちであり、公開済みLabの取得元を変更する決定ではない。@Saku0512と@yui-szkが影響と移行手順をレビューする。[共有契約の移行案](b2b-domain-contract-v0.1.md)では新しいLab IDとLesson Versionを使う。

1. File BrowserのComposeとDocker Hubイメージの出典、ライセンス、安全性、起動と脆弱版・修正版の挙動をレビューする。`images/CVE-2026-54088/docker-compose.review.yaml`は案であり、このままRegistryに登録しない。
2. 承認済みComposeを`images/CVE-2026-54088/docker-compose.yml`としてReprokitにコミットし、取得可能な固定コミットSHAを確定する。Composeとイメージのdigestはレビュー対象に含める。
3. 別の変更でManifestのGit URLを`https://github.com/Saku0512/Reprokit.git`、subdirectoryを`images/CVE-2026-54088`、revisionをその固定SHAへ変更する。Manifestを含むコミット自身のSHAをManifestに書こうとせず、先に確定したソースコミットを参照する。
4. Prepare時には従来どおり固定コミットを取得し、Composeを検査し、Manifest・Registry・ソース・Compose・イメージdigestをLockfileに記録する。Start前にも整合性を再検査する。ブラウザからGit URLやパスを選択できるようにしない。
5. 新規Sessionは新しい固定ソースを利用する。移行前に旧Runnerで既存Labを停止し、リソースとVolumeを確認する。旧Sessionや研修履歴は自動削除せず、その後の参照・管理方法を共有契約のレビューで確定する。既存Lockfileを黙って書き換えたり、新しいイメージへ暗黙に更新したりしない。

この方式でVulhub forkへの実行時取得は不要になるが、Reprokitの固定GitソースとDocker Hubへのアクセスは残る。Vulhubの原典とライセンスは出典として維持する。Registryを更新する前に、固定SHAの取得可能性、Docker Hub digest、Compose安全性、未変更のLesson参照、旧Sessionの処理を確認する。

