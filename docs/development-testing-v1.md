# 開発・テスト環境 v0.1.0

> **確定した設計値**。CIやGoコードはまだ実装済みではない。

## 開発環境

| 項目 | 採用する値 |
| --- | --- |
| 言語 | Go |
| Goの初期基準 | **Go 1.27.2**（2026-10-08リリース、セキュリティ修正を含む） |
| `go.mod` | `go 1.27`、`toolchain go1.27.2`（コード作成時に設定） |
| 正式対応OS | Linux |
| CIの標準ランナー | Ubuntu 24.04 / amd64 |
| Docker Engine | **28.0.0以上** |
| Docker Compose | **v2.36.1以上**（`config --lock-image-digests`のサポートを基準） |
| Manifest対象 | `linux/amd64` / `linux/arm64`（arm64実行テストは対応ランナーで実施） |

Goはパッチレベルのセキュリティ修正に追従する。`toolchain`は開発の初期基準であり、将来の修正版へ更新する。

## GitHub Actionsで行うテスト（実装予定）

PR時とmainへのPush時に以下を実行する。

1. YAML厳格パース、Manifest JSON Schema、CVE ID・Lab ID・固定Git SHAの検証。
2. ManifestからRegistryを決定的に生成し、`registry/index.json`と一致することを確認。
3. Registry/Lockfile JSON Schema、参照ファイル一覧・digest/ハッシュ不一致・不正パスの否定テスト。
4. `gofmt`、`go vet ./...`、`go test ./...`、必要に応じ`go test -race ./...`。
5. CLIの候補0/1/複数、非対話エラー、`--offline`、同名インスタンス、失敗時ロールバックの統合テスト。
6. **無害な専用イメージと環境**によるDocker Compose起動・TCPチェック・停止・Volume保持・削除の検証。
7. 危険なCompose設定（`build`、privileged、Hostネットワーク、Bind Mount、未固定変数、外部ポート公開等）を拒否するテスト。

公開CIでは危険なVulhubの実検証環境を**起動しない**。CVEの実験は専用VMで別途実施する。CIは最低限のGitHubトークン権限・シークレットで実行する。

## 受け入れ条件

- `create`がコンテナを起動しないこと。
- イメージdigest、元Compose、解決済みCompose、参照ファイルの改変を検知して`start`を拒否できること。
- 同じLabから複数インスタンスを作成でき、他インスタンスのコンテナとVolumeを誤操作しないこと。
- ホスト公開が`127.0.0.1`限定で、未公開ポートの内部サービスにもTCPチェック可能であること。
- Volumeをデフォルト保持し、削除記録から所有関係を追えること。
- Registryのスナップショットが取得途中で更新されても別コミットのManifestと混ざらないこと。

参考：[Go公式リリース履歴](https://go.dev/doc/devel/release)、[Compose設定リファレンス](https://docs.docker.com/reference/cli/docker/compose/config)、[Dockerポート公開](https://docs.docker.com/engine/network/port-publishing)。
