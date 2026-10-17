# CLI v0.1 廃止記録

> **決定：2026-10-17。** ReprokitはLabを操作するユーザー向けCLIを提供しない。以前のCLI設計は採用しないため、本書は移行上の記録として残す。

## 置き換え先

Labの取得、準備、起動、状態確認、停止、削除は、ローカルWeb UIからRunner APIを通じて行う。ブラウザはDockerを直接操作せず、Runnerが同じGo CoreのManifest、Lockfile、Compose安全性検査を必ず実行する。

| 旧CLIの責務 | 置き換え先 |
| --- | --- |
| 検索・教材選択 | Web UIのLessonカタログ |
| `create`相当の取得・検査 | Runner APIのPrepareLab |
| `start` | Runner APIのStartLab |
| `info`・`list` | Runner APIのGetLabStatusとWeb表示 |
| `stop` | Runner APIのStopLab |
| `delete` | Runner APIのDeleteLab |

## 禁止事項

- Labの状態を変更するCLIサブコマンドを実装しない。
- CLIから任意のCompose、Docker引数、ホストパス、イメージ、Git URL、コマンドを受け付けない。
- Web UIとは異なる安全性検査・Docker操作経路を作らない。

## ローカル起動

配布時はReprokitアプリケーションがローカルWeb Runnerを管理プロセスとして起動する。Runnerは`127.0.0.1`だけで待ち受け、Labを自動起動しない。Labの起動はWeb UIで利用者が注意事項を確認した後の明示操作だけに限定する。

詳細な安全境界は[学習インターフェース](interface-study.md)と[法人研修ドメイン契約](b2b-domain-contract-v0.1.md)を正本とする。
