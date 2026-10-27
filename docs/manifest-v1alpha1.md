# Reprokit Manifest v1alpha1 仕様

> **v0.1.0向けに採用した設計仕様です。** ここに記載したCLI・環境検証機能はまだ実装済みではありません。
>
> [機械可読なJSON Schema（Draft 2020-12）](../schemas/manifest-v1alpha1.schema.json) / [Manifestサンプル](../examples/reprokit-log4j-solr.reprokit.yaml)

## Reprokit Labを元にしたMVPとの関係（2026-10-15）

初期教材のLab定義は**Reprokit LabのGitコミットSHA + ディレクトリ + Composeを参照する方式**を優先し、元環境を新規作成しない。Manifest/JSON Schemaは技術的な参照仕様として維持するが、教育MVPの開発では少数のレビュー済みLabから始める。上流で未対応の構成があれば実行せず、教材対象から外すか、レビュー済み互換差分を固定して扱う（[MVP仕様](learning-mvp.md)）。

## 教育目的への移行について

このManifestは**教材が使う検証環境のみを定義**する技術仕様として維持する。教材本文や学習目標は別のLessonで定義する（[教材設計方針](learning-content.md)）。Manifestのスキーマは今回変更しない。

## 設計方針

- Manifestは**YAML**。標準のファイル名は `reprokit.yaml`。
- `apiVersion: reprokit.dev/v1alpha1` と `kind: Lab` を必須とする。
- v0.1では**Gitソース（固定コミットSHA）＋Docker Compose**だけに対応する。
- CVEと検証環境は**多対多**で扱う。CVEは検索キーであり、`metadata.id`は環境の固有ID。
- 任意のシェルコマンド・セットアップスクリプトはManifestから実行できない。
- Dockerイメージdigest、ファイルハッシュ、解決済み構成の固定は**別ファイル `reprokit.lock`**に記録する。
- `reprokit create`で取得・検査・保存するが、自動起動しない。`reprokit start`で明示的に起動する。
- 未知のフィールド、未知のバージョン、不正な形式は黙って読み飛ばさずエラーにする。

## Manifestの例

```yaml
apiVersion: reprokit.dev/v1alpha1
kind: Lab

metadata:
  id: reprokit/log4j-solr
  name: Log4Shell on Apache Solr

spec:
  cves:
    - CVE-2021-44228

  source:
    type: git
    url: https://github.com/Saku0512/Reprokit.git
    revision: 240f7aab24cfded9d5adbd0198143c3a28157101
    subdirectory: log4j/CVE-2021-44228

  runtime:
    type: docker-compose
    file: docker-compose.yml
    platform: linux/amd64

  target:
    product: Apache Solr
    version: "8.11.0"
    vulnerableComponent:
      name: Log4j
      version: "2.14.1"

  checks:
    - type: tcp
      service: solr
      port: 8983
```

このGitコミットとディレクトリにはReprokit Labの実在するCompose定義が含まれます。例に記載したからといって、起動成功や脆弱性の再現成功を保証するものではありません。

## フィールド一覧

| フィールド | 必須 | 内容・制約 |
| --- | --- | --- |
| `apiVersion` | 必須 | `reprokit.dev/v1alpha1`固定 |
| `kind` | 必須 | `Lab`固定 |
| `metadata.id` | 必須 | `provider/lab`形式、小文字英数字・`.`・`_`・`-`。各セグメント1〜64文字 |
| `metadata.name` | 必須 | 1〜256文字、空白だけの文字列は禁止 |
| `spec.cves` | 必須 | CVE IDの配列、1件以上、重複不可 |
| `spec.source.type` | 必須 | `git`固定 |
| `spec.source.url` | 必須 | HTTPSのGit URL |
| `spec.source.revision` | 必須 | 小文字16進40桁のGitコミットSHA |
| `spec.source.subdirectory` | 必須 | Gitリポジトリ内の安全な相対パス |
| `spec.runtime.type` | 必須 | `docker-compose`固定 |
| `spec.runtime.file` | 必須 | `subdirectory`からの相対Composeパス。`.yml`または`.yaml` |
| `spec.runtime.platform` | 必須 | `linux/amd64`または`linux/arm64` |
| `spec.target` | 任意 | 製品情報。指定時は`product`が必須 |
| `spec.target.version` | 任意 | 製品バージョン |
| `spec.target.vulnerableComponent` | 任意 | コンポーネント名・バージョン。指定時は両方必須 |
| `spec.checks` | 任意 | 起動確認の配列。指定時は1件以上 |
| `spec.checks[].type` | 必須* | `tcp`固定 |
| `spec.checks[].service` | 必須* | Composeサービス名 |
| `spec.checks[].port` | 必須* | コンテナ側TCPポート（1〜65535） |

* `spec.checks`内の各要素に必須。

JSON Schemaはトップレベルだけでなくネストされたオブジェクトもすべて`additionalProperties: false`で未知のキーを拒否します。

## Schemaだけでは検証できない条件

構文の検証に加え、CLI側で次の意味検証を実施します。

1. YAMLキー重複、過剰なエイリアス展開や不正型をパース時に拒否する。
2. Gitコミットが実在すること、`subdirectory`とComposeファイルが存在することを確認する。
3. パス正規化後の範囲外参照、シンボリックリンク、アーカイブ展開時の逸脱を防ぐ。
4. URLがHTTPSであっても認証情報、リダイレクト、接続先、SSRFの問題を別途検査する。
5. 実際のComposeを解析し、`checks[].service`が存在すること、`port`がコンテナ内TCPポートを示すことを確認する。TCPチェックはホストへのポート公開を必要としない。
6. Composeの`build`、特権モード、ホストネットワーク、Dockerソケット等の危険なマウント、およびスクリプト実行など未対応・危険な構成を拒否する。ポート公開は既定でループバックに制限する。
7. 実効Compose設定と使用Dockerイメージを取得・検査し、プラットフォームごとのdigestを解決してlockfileに固定する。
8. `checks`の成功は起動・到達性確認にすぎず、CVEの脆弱性を再現できた証拠にはならない。

## Registry・Manifest・Lockfileの境界

`Registry → Manifest → Lockfile → Docker Compose`

- **Registry**はCVE IDから検証環境を引く索引。Manifest内の`spec.cves`から自動生成する構想。静的なGitHub管理から始める。
- **Manifest**は環境の識別子・対応CVE・取得元・実行方式・対象と起動確認方法を宣言する正本。
- **Lockfile**は取得したManifestのハッシュ、Gitコミット、ファイルハッシュ、イメージdigestとplatform、実効構成を固定する。**lockfileの厳密な形式は [Lockfile v1仕様](lockfile-v1.md) と [JSON Schema](../schemas/lock-v1.schema.json) で確定済み**。
- **Runtime**はv0.1ではDocker Compose。`create`と`start`を分離する。

登録状態も `Indexed`（索引あり）、`Resolved`（依存解決済み）、`Bootable`（起動と基本チェック成功）、`Verified`（CVE固有の再現テスト成功）を区別する。

## v0.1.0で対象外

Dockerfileのビルド、Composeの`build`、任意のシェル／セットアップスクリプト、Docker Compose以外のRuntime、高度なHTTP/DNS等の検証、CVE再現テストの自動化、独自Registryサーバー。

## 関連する採用仕様

- [v0.1設計概要](architecture-v0.1.md)
- [Registry v1](registry-v1.md)
- [Lockfile v1](lockfile-v1.md)
- [CLI v0.1](cli-v1.md)
- [ローカル管理](local-storage-v1.md)
- [Docker Runtime](runtime-v1.md)
- [セキュリティ](security-v1.md)
- [Provider](provider-v1.md)
- [検証・状態管理](verification-v1.md)

## 旧README資料について

[`docs/reprokit-overview.md`](reprokit-overview.md) は [Saku0512/ReprokitのREADME](https://github.com/Saku0512/Reprokit/blob/main/README.md) を保存した**概要のスナップショット**です。そこには`schemaVersion`や`source.commit`などManifest設計前の古い例が含まれます。**v0.1のManifest仕様は本書およびJSON Schemaが優先**です。
