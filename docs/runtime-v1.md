# Docker Compose Runtime v0.1.0

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

