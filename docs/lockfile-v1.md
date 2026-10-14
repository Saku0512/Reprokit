# Lockfile v1 仕様

> v0.1の採用形式。**`reprokit.lock`はローカルの`create`で生成するファイル**であり、配布元が任意に作ったものを信頼してはならない。

## ファイル名・形式

- ファイル名：`reprokit.lock`
- 形式：YAML、`lockVersion: 1`
- [JSON Schema Draft 2020-12](../schemas/lock-v1.schema.json)で検証する。
- 固定する対象：Manifest原文、Registryスナップショット、Gitの取得元とコミット、元Compose原文、解決済み実効Compose、イメージのタグ参照とdigest、platform。
- 実効Composeファイルは`resolved.compose.yaml`として保存し、**ファイルのUTF-8バイト列そのもの**のSHA-256を記録する。JSON/YAMLの再フォーマット時にハッシュが変わるため、保存後のバイト列を基準にする。
- manifest・元Composeのハッシュも取得後に保存した実際のバイト列を基準とする。

## 形式例

※ `sha256:`のdigestおよび64文字のハッシュは説明用ダミー。起動できるLockfileの例ではない。

```yaml
lockVersion: 1
lab: vulhub/log4j-solr

registry:
  revision: 1111111111111111111111111111111111111111
  manifestPath: manifests/vulhub/log4j-solr.yaml

manifest:
  sha256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

source:
  url: https://github.com/vulhub/vulhub.git
  revision: 240f7aab24cfded9d5adbd0198143c3a28157101
  subdirectory: log4j/CVE-2021-44228
  composeSha256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

runtime:
  type: docker-compose
  platform: linux/amd64
  resolvedComposeSha256: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"

images:
  solr:
    reference: vulhub/solr:8.11.0
    digest: "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
    platform: linux/amd64
```

`images`は**Composeサービス名**をキーとする。各サービスのイメージを対応アーキテクチャ向けdigestへ固定し、`resolved.compose.yaml`には`image: ...@sha256:...`の形式で反映する。

## 生成・利用ルール

1. `create`でManifestを検証し、Registryコミット・Gitリビジョン・オリジナルComposeを固定する。
2. Compose構文・安全性・外部入力を検査して解決済みComposeを生成する。未固定の環境変数・外部ファイル・暗黙の`build`等は拒否する。
3. 使用する全サービスのイメージdigestをプラットフォームごとに解決する（タグは可変なので起動時にタグへ戻らない）。
4. 実効Composeのバイト列を書き出しSHA-256を記録する。
5. `start`時はLockfile、Manifest、ソースCompose、実効Composeのハッシュ、イメージ参照とdigestを照合する。不一致なら**起動拒否**。
6. Lockfileを暗黙更新しない。依存関係の更新が必要なら再`create`で**別インスタンス**を作成する（v0.1では`update`コマンドなし）。
7. `stop`や`list`では必要最小限の情報を参照し、実行中コンテナとの対応関係はComposeのproject識別子を用いる。

## 再現性の境界

Gitコミットやdigestの固定は、ホストカーネル、Dockerの挙動、時刻、外部通信、永続Volumeデータまで固定するものではない。再現性は「固定した入力群と実行条件を追跡できる」意味であり、CVEが必ず再現できるという保証ではない。

