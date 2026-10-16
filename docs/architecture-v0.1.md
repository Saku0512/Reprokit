# Reprokit v0.1.0 採用設計

> 2026-10-14 時点の**採用済み技術設計**。教育への全面転換を踏まえた現行のプロダクト方針は[こちら](product-vision.md)。各機能はまだ実装済みではない。ルート `README.md` は変更しない。

## 2026-10-15：教育MVPの開発優先順位を変更

**現行のMVPは[Vulhubを標準カタログとするWeb学習体験](vulhub-learning-mvp.md)を優先。** 初期教材は[CVE-2021-41773とCVE-2026-54088](initial-lessons.md)。前者はVulhub由来Lab、後者はReprokit管理のVulhub互換Labとして、同じGo Coreと安全性規則で扱う。以前の環境管理中心のロードマップを製品の最優先課題とはしない。

下記のManifest/Registry/Lockfile/Runtime仕様は**将来的に拡張可能な安全な基盤設計**として維持。高度な共通Registryや汎用環境管理の完全実装を、最初の教材Web MVPの必須条件とはしない。一方、起動前の安全性検査、取得元の固定、明示起動は維持する。

## スコープ

**プロダクトの主目的はCVEを体験しながら学ぶセキュリティ教育。** Registry・Manifest・Lockfile・Runtimeはそのためのローカル検証環境基盤として提供する。脆弱性スキャナーやExploit生成ツールではない。

[プロダクトビジョン](product-vision.md)と[教材設計方針](learning-content.md)が教育上の要件を定める。**学習画面はローカルWeb UI、環境管理にはGo CoreとCLIを共通利用する構成を採用済み**（[UI設計](interface-study.md)）。

**データの流れ：** `学習者 → Web教材/CLI → Go Core → Lesson + Lab Resolver（Vulhub / Vulhub互換） → Manifest/Lock → Docker Compose`。

Web UIはGo製のローカルHTTPサーバーから提供し、`reprokit ui`で起動する。Labの自動起動は行わない。既存の環境定義・再現性・安全性の仕様はWebとCLIで共通。

| 項目 | v0.1.0 採用仕様 | 文書 |
| --- | --- | --- |
| Manifest | YAML / `reprokit.dev/v1alpha1` / Gitコミット固定 / Compose専用 | [Manifest](manifest-v1alpha1.md) |
| Registry | GitHubの静的JSON索引、Manifestから生成、キャッシュ24h、Git SHAでスナップショット識別 | [Registry](registry-v1.md) |
| Lockfile | `reprokit.lock`（YAML）、Manifest/ソース/Compose/イメージを固定、変更時は起動拒否 | [Lockfile](lockfile-v1.md) |
| Web UI / CLI | 教材の主画面はローカルWeb。Go CoreをCLI/HTTP APIで共通利用。`reprokit ui`で起動 | [UI構成](interface-study.md) / [CLI](cli-v1.md) |
| ローカル保存 | LinuxのXDG規約に従い設定・環境・キャッシュ・ログを分離 | [Storage](local-storage-v1.md) |
| 実行 | Docker Compose projectごとに隔離、デタッチ、公開ポートは`127.0.0.1`に制限 | [Runtime](runtime-v1.md) |
| 安全対策 | 未対応/危険な設定を拒否し、Dockerは完全な隔離境界とみなさない | [Security](security-v1.md) |
| Provider | v0.1は共通Git取得処理でVulhub由来LabとVulhub互換Labを解決。専用プロトコルはv0.2以降 | [Provider](provider-v1.md) |
| 検証 | ライフサイクルと検証レベルを分離。起動/TCPの基本チェックまで | [Verification](verification-v1.md) |

## リポジトリ上の予定構成

```text
cmd/reprokit/          # Go CLIとローカルWeb起動エントリ（今後実装）
web/                  # 教材表示用フロントエンド（技術選定は未確定）
lessons/              # CVEごとの教材メタデータと本文（今後実装）
labs/                 # Reprokit管理のVulhub互換Labと互換アダプター（今後実装）
internal/{cli,registry,lesson,manifest,resolver,lockfile,runtime,security,storage,server}/
registry/index.json
registry/manifests/vulhub/
schemas/manifest-v1alpha1.schema.json
schemas/registry-v1.schema.json
schemas/lock-v1.schema.json
examples/
docs/
tests/                # 今後実装
```

## 教育MVPの完了条件（最新）

1. Webで**CVE-2021-41773とCVE-2026-54088**を選び、日本語で前提・脆弱性の概要・学習目標を読める。
2. Vulhub由来LabとVulhub互換Labを共通の取得・安全性確認フローで扱い、学習者が**明示起動**できる。
3. Webのステップ解説とPoC操作例を見ながら、学習者自身がローカルLabで検証し、観察結果を理解できる。
4. 原因と修正・緩和方法、片付けまで説明し、停止・削除できる。
5. 初学者が上記の一連の体験を完走できるかユーザーテストする。

## 従来の技術基盤の完了条件（後続の拡張ロードマップ）

- `reprokit ui`で初心者向けWeb教材画面を開ける。既存のCLI環境操作も独立して使える。
- 初学者が選べる初期2教材があり、学習目標・処理経路・手順・観察ポイント・原因と対策まで提示できる。
- 取得・起動だけでは完成とせず、両教材を初学者が最後まで進められるかを検証する。
- 環境構築を支える以下の技術要件も満たす。

- CVE IDで検索し、複数候補があれば環境を選択できる。
- Manifestの厳格なバリデーションとGit固定コミットでのソース取得ができる。
- 取得した構成から実効Composeを決定し、利用する全イメージをdigest固定できる。
- Manifest・ソース・Compose・イメージの整合性を検査して起動できる。
- Docker Composeの危険な構成を拒否でき、公開ポートをループバックへ限定できる。
- インスタンスを複数管理し、起動・停止・削除・Volume削除を明示的に操作できる。
- TCP到達性でBootable判定でき、Verifiedと混同しない。

## 仕様の優先順位

- Manifestの形状は[ManifestのJSON Schema](../schemas/manifest-v1alpha1.schema.json)を正本とする。
- Registryの形状は[RegistryのJSON Schema](../schemas/registry-v1.schema.json)を正本とする。
- Lockfileの形状は[LockfileのJSON Schema](../schemas/lock-v1.schema.json)を正本とする。
- CLI動作やComposeに対する意味検証は、このドキュメント群の規則を参照する。
- [概要資料](reprokit-overview.md)は元のREADMEの保存コピーであり、古いManifest例等は本仕様に優先しない。

## 追加確定：実装前の7項目（2026-10-14）

1. Composeの**許可リスト**を採用。image・固定TCP ports・固定値environment・短形式depends_on・named volumes・project専用Bridgeのみサポート。詳細は[Runtime](runtime-v1.md)。
2. GitHub APIで公式RegistryのmainコミットSHAを一度だけ解決。index/Manifestを同じSHAから取得し、キャッシュ24時間。明示的`--offline`対応。[Registry](registry-v1.md)
3. TCPチェックは短命な専用コンテナで内部Bridge経由に実行。`alpine:3`のdigestも固定。[Verification](verification-v1.md)
4. Lockfileに参照ファイル`source.files`のSHA-256、チェック用イメージ`checker`を追加。[Lockfile](lockfile-v1.md)
5. Volumeは既定保持し、削除後の所有関係を`volumes.json`に保存。[Local Storage](local-storage-v1.md)
6. `create`はステージングと原子的rename。`start`失敗は対象Projectだけロールバック。[Local Storage](local-storage-v1.md)
7. Go 1.27.2、Linux、Docker Engine 28以上、Compose v2.36.1以上、GitHub Actionsのテスト方針を採用。[開発・CI](development-testing-v1.md)

これらは**仕様の採用**であり実装完了ではない。
