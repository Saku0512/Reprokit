# Docker Compose Runtime v0.1.0

## Vulhub連携時の適用方針（2026-10-16更新）

教育MVPでは[`Saku0512/vulhub`](https://github.com/Saku0512/vulhub)の固定スナップショットから、upstream由来Labとfork追加Labを利用する。どちらにも同じ許可リスト、Lockfile、明示起動の規則を適用し、安全性検査を省略しない。

元環境が本書の許可リストに適合する場合に限りそのまま取り込み、非対応の設定（`build`、bind mount、`command`等）が必要な場合は**個別のレビュー済み互換アダプターを管理対象にして、上流コミット・元ファイル・差分を固定・検査**する。自動で制限を弱める機能は付けない。

初期教材のCVE-2021-41773はfork内の元Composeが`build: .`を使用するため、固定Dockerfileからfork CIで事前ビルドしたレビュー済みイメージを`ghcr.io/saku0512/*`へ公開し、digest固定して許可リスト内のComposeから使う。CVE-2026-54088はforkの`filebrowser/CVE-2026-54088`へ追加済み。元Composeの`profiles`は許可リスト外のため直接実行せず、脆弱版2.63.5と修正版2.63.6を異なるloopbackポートで起動する`reprokit.compose.yml`を使用する。

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
| ネットワーク | インスタンス専用Bridge |
| Image | digest固定、プラットフォーム明示 |
| 自動再起動 | 無効化（`no`相当） |
| 起動/基本チェックの全体タイムアウト | 60秒 |
| Docker Volume | 削除時も既定では保持 |
| Compose `build` | 拒否 |
| ネットワーク外向き通信 | 標準Bridgeの範囲で可能。完全隔離とは主張しない |

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

`volumes`・`networks`の省略は可能。省略時はComposeがproject専用のdefault Bridgeを作る。`restart`は未指定か`"no"`だけを許し、実効Composeでは`restart: "no"`に固定。`expose`・`user`・`container_name`・`command`・`entrypoint`・`healthcheck`・`build`・`secrets`・`configs`・`include`・`extends`・`env_file`・`profiles`・`deploy`その他許可外の属性は拒否する。YAMLアンカー、マージ、エイリアスもv0.1では拒否する。

明示的な`0.0.0.0`/外部IP/IPv6バインド、UDP、ポート範囲・省略・動的割当・長形式は非対応。ループバック公開は外向き通信を阻止しない。Docker Image本来のENTRYPOINTは第三者コードとして実行されるため、元Composeによる上書きを拒否しても安全性は保証できない。

## 専用TCPチェック用コンテナ

`checks`が指定されている場合、`create`で`alpine:3`のplatform別digestを解決しLockfileの`checker`へ記録。`start`時はdigest固定イメージで**短命な専用チェックコンテナ**を作り、対象サービスと同じproject Bridgeに接続して`nc -z -w 3 <service> <port>`相当を実行する。CLIが作った引数配列を使用し、シェル文字列の組み立てをしない。

チェッカーは`--rm`、`--cap-drop=ALL`、`--read-only`、`no-new-privileges`、ホストポート・bind mountなし。必要なBridge以外へは接続しない。3秒/回、2秒間隔、起動から計60秒以内。`checks`未定義ならBootable判定しない。
