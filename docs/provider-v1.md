# Provider / 取得方式 v0.1.0

## 採用方針

- **最初の対応対象はVulhub**。ただしVulhub専用の取得ロジックは作らず、Manifestで指定された任意のHTTPS Gitリポジトリから取得する共通処理を構築する。
- `spec.source.type`は`git`のみ。`url`、固定`revision`、`subdirectory`を共通インターフェースとして扱う。
- カスタムProviderインターフェース、独自の取得プロトコル、外部Registryのプラグイン対応はv0.2以降に延期する。

## 取得フロー

1. Registryから`metadata.id`でManifestを特定する。
2. Manifestのスキーマ、CVE・Lab IDの一致、Git URLを検証する。
3. Git URLとコミットSHAをキーにキャッシュを参照し、存在しなければ取得する。
4. 指定コミットを検証し、`subdirectory`配下をインスタンス用領域に複製する。
5. 参照先の範囲外リンクなどを検査し、Compose・依存イメージの解決へ進む。
6. 元ファイルのハッシュと解決済みの情報をLockfileへ記録する。

GitHub APIが特定の事業者であることを前提にしない。取得できるかどうかは各GitホストのHTTPSアクセスと認証・安全性制約に従う。

## ProviderとLab IDの違い

`vulhub/log4j-solr`の`vulhub`はManifest内の**名前空間**であり、実行時に同名の特別なProviderプラグインが必要という意味ではない。同じCVEが別Gitリポジトリにある場合も別Lab IDとして登録できる。

