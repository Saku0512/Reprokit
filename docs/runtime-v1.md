# Docker Compose Runtime v0.1.0

> **現行実装（2026-10-27）**: File Browser LabはReprokitのローカルGitに含まれる固定コミットのComposeと、Docker Hubのdigest固定イメージを使用する。旧Vulhub構成は以下の歴史的な設計記録であり、実行時の取得元ではない。詳細は[Lab移行](lab-catalog-runtime-v0.1.md)を参照。

## Vulhub連携時の適用方針（2026-10-16更新）

教育MVPでは[`Saku0512/vulhub`](https://github.com/Saku0512/vulhub)の固定スナップショットから、upstream由来Labとfork追加Labを利用する。どちらにも同じ許可リスト、Lockfile、明示起動の規則を適用し、安全性検査を省略しない。

元環境が本書の許可リストに適合する場合に限りそのまま取り込み、非対応の設定（`build`、bind mount、`command`等）が必要な場合は**個別のレビュー済み互換アダプターを管理対象にして、上流コミット・元ファイル・差分を固定・検査**する。自動で制限を弱める機能は付けない。

最初の縦断対象はforkの`filebrowser/CVE-2026-54088`だけとする。元Composeの`profiles`は許可リスト外のため直接実行せず、脆弱版2.63.5と修正版2.63.6を異なるloopbackポートで起動する`reprokit.compose.yml`を使用する。Apache HTTP Serverなど追加CVEは、この最初の実装を完了した後に別Issueで扱う。

Docker Hubの`vulhub/*`はupstream管理の名前空間であり、独自Lab用イメージの公開先として仮定しない。詳細は[Lab昇格設計](lab-catalog-runtime-v0.1.md)。

詳細：[Vulhub連携・教育MVP](vulhub-learning-mvp.md)。

## 基本方針

- Runtimeは**Docker Composeのみ**。Dockerfile/Compose `build`はv0.1で非対応。
- インスタンスごとに固有のCompose Project名を割り当てる。Container名をManifest側で固定する設定は許可しない。
- `start`はデタッチモード、`stop`は停止、`delete`はProject削除。
- DockerイメージはLockfileに記録したdigestを必ず参照し、起動時にタグを追跡し直さない。
- イメージ未取得の場合は**固定digestのみ**を取得する。ビルドや任意の更新は行わない。

## 採用する初期値

| 項目 | 値 |
| --- | --- |
| Project名 | `reprokit_<instance-id-derived>`。Instance ID由来で衝突チェック |
| コンテナ起動 | デタッチ |
| ポートのバインド | `127.0.0.1`のみ |
| ポート競合 | 起動を中止してエラー |
| ネットワーク | Lab本体はインスタンス専用の内部Bridge（外向き通信なし）。公開時だけRunner所有の固定プロキシを経由 |
| Image | digest固定、プラットフォーム明示 |
| 自動再起動 | 無効化（`no`相当） |
| 起動/基本チェックの全体タイムアウト | 60秒 |
| Docker Volume | 削除時も既定では保持 |
| Compose `build` | 拒否 |
| ネットワーク外向き通信 | `none`。Labごとに必要な通信をレビューして明示対応するまでは許可しない |

## ポート公開の変換

例（配布元のCompose）：

```yaml
ports:
  - "8983:8983"
```

保存する`resolved.compose.yaml`では次のように固定する。

```yaml
ports:
  - "127.0.0.1:8983:8983"
```

IPv6・外部IPへの公開、ホストポート番号未指定の自動割当、プロトコルが不明な指定などをv0.1では拒否する。複雑なComposeポート構文を黙って変換しない。ホストポートが重複していたら自動変更しない。

**注意：** ループバックへの公開は、コンテナ内からの外向き通信やDockerホスト上の他プロセスからのアクセスを遮断しない。古いDocker Engineではlocalhost公開の隔離に既知の制限があるため、v0.1で必要なDocker Engineの最低バージョン・動作確認を安全性検査に含める（[安全性仕様](security-v1.md)）。

## Compose入力の範囲

初期実装は、構成を決定論的に評価できる単一Composeファイルに限定する。

- `build`、`include`、`extends`、`env_file`、秘密情報・外部設定ファイル等の未固定参照は拒否。
- 解決時に外部環境変数に依存する`\${...}`展開は拒否（固定値をManifestへ追加するのは将来の仕様）。
- イメージ標準の起動動作を基本とし、`command`/`entrypoint`などコンテナ動作を上書きする設定はv0.1では拒否。
- named volumeは利用可能だが、ホストのbind mount・ホストネットワーク・特権設定は拒否。
- Composeの実効構成と、そこから作成する`resolved.compose.yaml`のハッシュをLockfileで照合する。

## Docker Compose 許可リスト（追加確定）

v0.1は元のCompose YAMLを**厳格な許可リスト**で検証し、正規化後の実効Composeも再検査する。許可されないキーは警告ではなく拒否する。

| 位置 | 許可するキー | 許可する形式 |
| --- | --- | --- |
| 最上位 | `version`、`services`、`volumes`、`networks` | `version`は旧形式の互換入力のみ。`name`は拒否 |
| サービス | `image` | 全サービス必須。digest固定できる参照のみ |
| サービス | `ports` | TCP短縮形式`"HOST:CONTAINER"`または`"127.0.0.1:HOST:CONTAINER"`。各ポート1～65535、前者は後者に正規化 |
| サービス | `environment` | 文字列→文字列の固定値マップ。null・配列・`$`による補間は不可 |
| サービス | `depends_on` | サービス名の短形式配列。存在・循環を検査 |
| サービス | `volumes` | 宣言済みnamed Volumeの`volume-name:/container/path[:ro|rw]`のみ |
| サービス | `networks` | 宣言済みのproject内Bridge名の配列。1サービス最大1ネットワーク |
| 最上位`volumes` | `name: {}` | 外部指定・driver_opts・明示名を禁止 |
| 最上位`networks` | `name: {}` | project専用Bridgeのみ。`external`/`ipam`/driver_opts禁止 |

`volumes`・`networks`の省略は可能。入力Composeのネットワークはproject専用Bridgeだけを許可する。`guardrails.egress: none`を指定したLabでは、解決済みComposeにRunnerが`internal: true`の専用Bridgeを付与し、Labサービスをそこへ接続する。Docker Engine 29系では内部Bridgeだけに接続したコンテナの公開ポートが無視されるため、Runnerは各公開ポートについて固定digestのCaddyプロキシを生成する。プロキシだけが通常Bridgeと内部Bridgeの両方へ接続し、`127.0.0.1`公開を受け、固定した対応LabサービスのHTTPポートへだけ中継する。プロキシのイメージ、コマンド、宛先、ポートはRunner定数であり、利用者・Manifest・元Composeから指定できない。Labサービス自身は通常Bridgeへ接続しない。`restart`は未指定か`"no"`だけを許し、実効Composeでは`restart: "no"`に固定。`expose`・`user`・`container_name`・`command`・`entrypoint`・`healthcheck`・`build`・`secrets`・`configs`・`include`・`extends`・`env_file`・`profiles`・`deploy`・`storage_opt`その他許可外の属性は拒否する。YAMLアンカー、マージ、エイリアスもv0.1では拒否する。

## 実行ガードレール

各Lab Manifestの`spec.runtime.guardrails`はLockfileへ固定し、Prepare時とStart直前に同じ解決済みComposeを生成して照合する。利用者・Web UI・元Composeから上限を変更できない。

```yaml
guardrails:
  maxRuntimeSeconds: 1800
  cpuCores: 1
  memoryMiB: 512
  pids: 128
  diskMiB: 1024
  egress: none
```

- `maxRuntimeSeconds`は60〜14,400秒、CPUは1〜4コア、メモリは128〜4,096 MiB、PID数は64〜512、ディスクは256〜8,192 MiBに限定する。
- Runnerは`deploy.resources.limits`でCPU・メモリ・PID数を付与する。ディスクはDockerの各コンテナが報告する書込み量（`SizeRw`）をProject単位で2秒ごとに合計し、`diskMiB`を超えたSessionを`disk_limit_exceeded`として停止する。監視値を取得できない場合も`disk_monitor_unavailable`として停止する。
- Dockerの`storage_opt.size`はファイルシステム依存であり、解決済みComposeへ付与しない。書込み量の監視はファイルシステムquotaではないため、監視頻度の間に上限を超える可能性がある。高い隔離保証が必要な演習は、専用VMと容量制限済みのホストで実行する。
- `egress`は現時点で`none`だけを許可する。外部通信が必要な教材は、宛先・必要性・実装する隔離方法を別途レビューして対応する。
- Runner生成の入口プロキシはLabサービスではない。プロキシの通常Bridge接続はloopback公開を実現するためだけに使い、Labサービスの外向き通信を許可しない。プロキシの固定イメージ参照もLockfileの`images`へ記録し、起動前に再照合する。
- 起動後は期限で自動停止する。Runnerの再起動時には、前回のRunnerが残したrunning/error Sessionを削除して、保持指定のVolumeだけを記録する。

## コンテナログの閲覧

学習者が実行状態を確認できるよう、Runnerは所有する実行中のLab Sessionについて、`vulnerable`または`fixed`サービスのログを読み取り専用で返せる。対象Serviceは固定の比較対象だけであり、コンテナ名、Compose Project名、Docker引数、コマンド、パスはHTTP入力から指定できない。Runnerが生成する入口プロキシを含む内部サービスのログは公開しない。

返す内容は`docker compose logs`の追従しないスナップショットで、最新200行かつ64 KiBまでとする。上限を超えた場合は末尾を返し、切り詰めたことを明示する。停止・削除済み・未起動Sessionのログ取得は拒否する。ログにはLabへの入力など機微な値が含まれ得るため、同一のローカルSessionトークンを持つ学習画面だけに返し、画面にはその注意を表示する。

## 検証アクション

OSコマンドインジェクションの観察などで必要なコンテナ内確認は、任意コマンドの実行機能ではなく、Runnerに固定した検証アクションとしてだけ提供する。v0.1のFile Browser比較Labで許可するアクションは`list_tmp`だけであり、実行内容は`ls -la /tmp`に固定する。対象サービスは`vulnerable`または`fixed`だけで、サービス、実行ファイル、引数、作業ディレクトリ、環境変数、DockerオプションをHTTP入力から指定できない。シェルは介在させず、入口プロキシを含むRunner内部サービスにも実行できない。

検証アクションは実行中Sessionにだけ許可し、標準出力・標準エラーを合わせて64 KiBまで返す。追従・対話入力・ファイル書込み・ネットワーク通信・複数コマンド・コマンド置換は提供しない。各アクションは教材のレビューと固定Lab定義の変更に合わせて個別に追加する。

### 固定マーカーの観察

File Browser教材では、ログイン入力によって作られる`/tmp/success`を、`GET /api/labs/{id}/markers/{vulnerable|fixed}`で読み取り専用に観察できる。これは任意コマンド実行ではなく、`vulnerable`または`fixed`に限定した固定パスの読取りである。内容は最大512 byteで、脆弱版に記録された`id`の出力を受講画面へ返す。対象Sessionが起動中でない場合、未宣言サービス・Runner生成プロキシ・任意パスの指定はいずれも拒否する。

明示的な`0.0.0.0`/外部IP/IPv6バインド、UDP、ポート範囲・省略・動的割当・長形式は非対応。ループバック公開は外向き通信を阻止しない。Docker Image本来のENTRYPOINTは第三者コードとして実行されるため、元Composeによる上書きを拒否しても安全性は保証できない。

## 専用TCPチェック用コンテナ

`checks`が指定されている場合、Prepare時に`alpine:3`のplatform別digestを解決しLockfileの`checker`へ記録。Start時はdigest固定イメージで**短命な専用チェックコンテナ**を作り、対象サービスと同じproject Bridgeに接続して`nc -z -w 3 <service> <port>`相当を実行する。Runnerが作った引数配列を使用し、シェル文字列の組み立てをしない。

チェッカーは`--rm`、`--cap-drop=ALL`、`--read-only`、`no-new-privileges`、ホストポート・bind mountなし。必要なBridge以外へは接続しない。3秒/回、2秒間隔、起動から計60秒以内。`checks`未定義ならBootable判定しない。
