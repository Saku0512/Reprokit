# CLI v0.1.0 仕様

> コマンド例は**採用設計**であり、まだ実行可能とは限らない。

## サブコマンド

| コマンド | 主な用途 |
| --- | --- |
| `reprokit search <CVE-ID>` | Registryから候補Labを探す |
| `reprokit create <CVE-ID> [--lab <lab-id>] [--name <name>]` | ソース取得・解決・固定。**起動しない** |
| `reprokit create <lab-id> [--name <name>]` | Lab IDの直接指定 |
| `reprokit info <instance-name>` | ローカル環境の取得元・ポート・権限・lock情報を表示 |
| `reprokit start <instance-name>` | lock整合性と安全性検査後に起動 |
| `reprokit stop <instance-name>` | 対象のCompose projectを停止 |
| `reprokit delete <instance-name> [--volumes]` | インスタンスを削除。Volumeは既定で維持 |
| `reprokit list` | 管理中のインスタンス一覧と状態 |
| `reprokit registry update` | Registryのキャッシュを手動更新 |

### 選択規則

```bash
reprokit create CVE-2021-44228
reprokit create CVE-2021-44228 --lab vulhub/log4j-solr --name log4shell-test
reprokit create vulhub/log4j-solr --name another-lab
reprokit start log4shell-test
reprokit stop log4shell-test
reprokit delete log4shell-test
reprokit delete another-lab --volumes
```

- CVE候補**0件**なら検索結果なしとしてエラー。
- 候補**1件**なら自動選択。
- 候補**複数**なら対話端末で選択。非対話端末ではエラーとし`--lab`を要求する。
- `--lab`を明示した場合、そのCVEの候補に含まれなければ拒否する。
- `--name`省略時はLab名部分（例`log4j-solr`）を候補名とする。同名が既に存在すれば**上書きせずエラー**。別名を指定して複数インスタンスを作れる。
- ローカルインスタンス名：小文字英数字と`-`、先頭は小文字英字、長さ1～40文字。内部のID（UUID）とは分離する。
- `info`・`start`・`stop`・`delete`は**ローカルインスタンス名**を受け取る。CVE IDやLab IDとの曖昧な照合はしない。
- 非対話時に確認が必要な場合は非ゼロ終了し、危険な処理を自動承認しない。
- `delete`時は実行中なら安全に停止してからCompose projectを削除する。Volumeは`--volumes`指定時のみ削除する。

### エラー/終了コード（推奨）

| Code | 意味 |
| --- | --- |
| 0 | 成功 |
| 1 | 実行中の処理で失敗（I/O、Docker等） |
| 2 | CLIの入力・引数が不正 |
| 3 | CVE/Lab/インスタンスが存在しない、または複数候補で選べない |
| 4 | Manifest/Lockfile/Registry不正、または整合性不一致 |
| 5 | 危険な構成や未対応構成のため拒否 |

エラーは標準エラー出力へ、検索結果や一覧は標準出力へ表示。成功していない操作を成功と報告しない。

## v0.1で扱わないコマンド

`reprokit update`、任意スクリプト実行、起動時の無条件`--force`、外部公開を許可するフラグ、複数Registryの動的追加は対象外。

