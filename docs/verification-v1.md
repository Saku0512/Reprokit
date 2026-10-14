# 状態・検証仕様 v0.1.0

## 2種類の状態を分ける

**ライフサイクル**：`Created → Running → Stopped`（`Stopped → Running`へ再起動可能）。`delete`後は登録を削除する。起動失敗は成功状態にせず、前状態とエラー記録を残す。`list`はDockerの実状態と照合し、ズレは報告する。

**検証レベル**：`Indexed / Resolved / Bootable / Verified`。

| 検証レベル | 条件 |
| --- | --- |
| `Indexed` | Registryに適合Manifestが存在する |
| `Resolved` | Git・Compose・全イメージを固定し、lock整合性が確認済み |
| `Bootable` | 指定条件下で起動し、Manifestの基本チェックが成功 |
| `Verified` | 当該CVE専用の手順で期待する結果を確認。v0.1では判定しない |

レベルは単純な単調増加のグローバル値ではない。**Instance ID、lockfileのハッシュ、実施日時、実行条件**に結び付けた検証履歴として扱う。停止後も過去の成功記録は保持しつつ、現在稼働中と混同しない。

## Bootableのチェック

- v0.1ではManifestの`spec.checks`に記載した**TCP**チェックのみ対応。
- サービス名はCompose上のサービス名。**コンテナ側**のTCPポートに到達できることを検証する（ホスト公開ポートとは区別）。
- 各接続のタイムアウト：**3秒**。
- リトライ間隔：**2秒**。
- 起動とチェックを含む全体タイムアウト：**60秒**。
- 複数checksがある場合、**すべて成功**したときだけBootableとする。
- `checks`未指定の環境では起動状態を確認できても**Bootableへ自動昇格しない**。
- TCP接続成功は、脆弱性が存在することやExploitが動作することの証明ではない。

## 検証ログ

`$XDG_STATE_HOME/reprokit/logs/`に、Instance ID、Lab ID、LockfileのSHA-256、検証日時、チェックごとの成功・失敗・エラーメッセージを保存。機密情報やトークンはログに残さない。

## v0.1対象外

`Verified`の自動判定、Exploit実行、任意スクリプトでのチェック、HTTP/DNS/プロセス別の高度なチェック、外部端末からの動作確認。

## チェック用コンテナの実装方式（追加確定）

- `spec.checks`を持つ環境では`create`時に`alpine:3`をplatform別digestへ解決し、`reprokit.lock`の`checker`に固定する。
- `start`後、対象Composeサービスと**同じProject Bridge**へ短命なチェック用コンテナを参加させる。固定イメージ・固定引数の`nc -z -w 3 <service> <container-port>`相当を使い、シェル文字列補間はしない。
- チェッカーはホスト公開ポート、Bind Mount、追加Capabilityを持たず、root filesystemはread-only、`no-new-privileges`。チェック後に必ず削除する。
- 対象サービスが所属するネットワークを確認してから接続する。3秒/回、2秒間隔、全体60秒を上限とし、全checks成功のみBootable判定。
- `checks`があるのにLockfileの`checker`が欠けている、またはdigest不一致なら判定せずエラー。`checks`未指定ならBootableと判定しない。
