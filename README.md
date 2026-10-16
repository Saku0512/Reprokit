# Reprokit

**実際のCVEを、処理経路を追いながら自分で試して理解するための、初学者向け体験型セキュリティ学習ツールです。**

> [!IMPORTANT]
> 現在は設計・初期実装段階です。記載しているWeb UI、CLI、教材は未実装または開発中です。

## 製品概要

Reprokitは、脆弱性の説明を読むだけでなく、管理されたローカル環境で安全な実験を行い、入力がどの処理を通って危険な動作へ到達するのかを一段ずつ学ぶOSSです。

学習画面はローカルWeb UI、環境の取得・起動・停止はGo製の共通CoreとCLIが担当します。脆弱なLabは自動起動せず、公開ポートは原則として`127.0.0.1`へ限定します。

## 初期教材

| 教材 | 主題 | Labの提供元 |
| --- | --- | --- |
| CVE-2021-41773 | Apache HTTP Serverのパス正規化不備によるパストラバーサル | Vulhubの固定コミットを参照し、Reprokit互換アダプターで実行 |
| CVE-2026-54088 | File Browser Hook Authenticationの未認証OSコマンドインジェクション | Reprokit管理のVulhub互換Lab |

教材は「入力 → 変換 → 信頼境界 → 危険な処理 → 影響 → 修正」の順に表示し、脆弱版と修正版の違いまで説明します。詳細は[初期教材方針](docs/initial-lessons.md)を参照してください。

## Labの扱い

- Vulhubを標準のLabカタログとして内部利用します。
- ReprokitはVulhubのCLIを起動するのではなく、固定したGitコミットと環境ディレクトリを取得し、検査後にDocker Composeで実行します。
- Vulhubに未収録のCVEは、同様のディレクトリ構造とDocker Composeを持つ**Vulhub互換Lab**としてReprokit側で管理します。
- 独自LabもManifest、Lockfile、安全性検査、明示起動の規則をVulhub由来Labと共有します。

## 学習の流れ

1. 教材を選び、前提知識と学習目標を読む。
2. 脆弱性へ到達する処理経路を一段ずつ確認する。
3. 固定されたLabを取得し、安全性と公開ポートを確認する。
4. Labを明示的に起動し、無害な検証を自分で実行する。
5. 観察結果、根本原因、修正方法を理解する。
6. Labを停止・削除する。

## 開発技術（予定）

- Core / CLI / Local Web Server: Go
- Runtime: Docker Compose
- Lab catalog: Vulhub + Reprokit管理のVulhub互換Lab
- Metadata: YAML / JSON Schema
- Frontend: ローカルWeb UI（詳細技術は選定中）

## ドキュメント

- [プロダクト方針](docs/product-vision.md)
- [初期教材方針](docs/initial-lessons.md)
- [Vulhub連携MVP](docs/vulhub-learning-mvp.md)
- [アーキテクチャ](docs/architecture-v0.1.md)
- [学習インターフェース](docs/interface-study.md)
- [セキュリティ仕様](docs/security-v1.md)

## 安全上の注意

Reprokitが扱うLabには意図的に脆弱なソフトウェアが含まれます。自分が所有する環境または明示的な許可を得た環境だけで利用し、第三者のシステムを対象にしないでください。Dockerを完全な隔離境界とはみなしません。

## ライセンス

[MIT License](LICENSE)
