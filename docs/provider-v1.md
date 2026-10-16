# Provider / 取得方式 v0.1.0

## 採用方針（2026-10-16更新）

- **標準のLabカタログは[`Saku0512/vulhub`](https://github.com/Saku0512/vulhub)**とする。
- このリポジトリは`vulhub/vulhub`のforkであり、upstreamから既存環境を継承しつつ、未収録CVEの独自Labを追加する。
- v0.1ではVulhub専用CLIや専用ネットワークプロトコルを作らず、HTTPS Gitリポジトリから取得する共通処理を使う。
- `spec.source.type`は`git`のみ。`url`、固定`revision`、`subdirectory`を共通インターフェースとして扱う。
- カスタムProviderプラグイン、独自取得プロトコル、複数カタログの動的追加はv0.2以降に延期する。

Vulhubは検証環境を収録するGitリポジトリであり、Labを起動するCLIではない。「内部的にVulhubを使う」とは、Reprokitがforkの固定コミットと対象ディレクトリを取得し、自身の安全性検査とDocker Compose Runtimeで実行することを指す。

## カタログ構成

```text
Saku0512/vulhub
├── httpd/CVE-2021-41773/       # upstreamから継承
└── filebrowser/CVE-2026-54088/ # Reprokit用に追加
```

- forkの`master`はupstream更新を取り込む基準ブランチとする。
- アプリは`master`を実行時に追従しない。Registry保守工程が採用する40文字のforkコミットSHAを決め、Manifestへ書き込む。
- クライアントの`registry update`は`jphacks/sd_2601`のRegistryスナップショットだけを取得し、forkの`master`を再解決しない。
- 同じRegistryスナップショット内では、すべてのLabをManifestに記録された同じforkコミットから取得する。
- upstream由来かfork追加かを教材画面と監査情報に表示する。
- fork追加Labも将来`vulhub/vulhub`へ提案可能なCVE単位レイアウトを保つ。

## 初期教材の割り当て

| CVE | 種別 | fork内のパス |
| --- | --- | --- |
| CVE-2021-41773 | upstream由来 | `httpd/CVE-2021-41773` |
| CVE-2026-54088 | fork追加 | `filebrowser/CVE-2026-54088` |

両方のManifestは`source.url: https://github.com/Saku0512/vulhub.git`を使い、同じRegistry更新で解決した`revision`を記録する。Labごとの差は`subdirectory`で表現する。

## 取得フロー

1. Registry保守工程が`Saku0512/vulhub`の採用コミットSHAを決め、レビュー済みManifestへ記録する。
2. クライアントは固定されたRegistryスナップショットから`metadata.id`でManifestを特定する。
3. Manifestのスキーマ、CVE・Lab IDの一致、HTTPS Git URL、固定`revision`、安全な`subdirectory`を検証する。
4. Git URLとコミットSHAをキーにキャッシュを参照し、存在しなければ取得する。
5. 指定コミットを検証し、`subdirectory`配下を独立したステージング領域へ複製する。
6. upstream由来情報、fork追加情報、Reprokit用Composeの差分、参照先の範囲外リンクを検査する。
7. `ghcr.io/saku0512/*`の昇格済みイメージをplatform別digestへ解決する。Docker Hubの`vulhub/*`へ独自イメージを公開できるとは仮定しない。
8. Composeの許可リスト検査、Lockfile生成を行う。
9. すべて成功した場合のみインスタンスを原子的に登録する。`create`では起動しない。

## ProviderとLab IDの違い

`vulhub/httpd-cve-2021-41773`と`vulhub/filebrowser-cve-2026-54088`は、同じカタログ内の別Lab IDである。名前空間は実行時プラグインを意味しない。upstream由来/独自追加は表示・監査用メタデータであり、実際の取得は同じGit処理へ収束させる。

## upstream同期と再現性

- forkをupstreamへ同期しても、既存のLockfileが参照するコミットSHAは変更しない。
- 新しいupstream状態や独自Lab更新を使うには、Registryスナップショットを明示的に更新する。
- 更新途中で`master`を再解決せず、index、Manifest、Labが別コミットから混ざることを防ぐ。
- forkを所有していることは安全性の保証ではない。独自LabにもSchema、Compose許可リスト、digest固定、パス検証を適用する。

## キャッシュと障害時の責任分担

ソースは`Git URL + revision`で共有キャッシュを利用できるが、作成中のインスタンスは独立したステージング領域で検証する。取得・検証・digest解決・Lockfile生成がすべて成功した時だけ登録し、失敗時は未完成のインスタンスを残さない。


## イメージ公開境界（2026-10-17確定）

forkへのファイル追加権限と、Docker Hubの`vulhub/*`名前空間への公開権限は別である。独自LabのイメージはforkのGitHub Actionsでビルドし、管理可能な`ghcr.io/saku0512/*`へ公開する。upstreamへPRを送り公式イメージとして採用されることは将来の任意工程であり、Reprokitの起動条件にしない。詳細は[Lab昇格設計](lab-catalog-runtime-v0.1.md)を参照する。
