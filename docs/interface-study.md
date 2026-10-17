# Reprokit 学習インターフェース — ローカルWeb Runner構成

> **決定：2026-10-17。** 学習とLab操作の入口はローカルWeb UIだけとする。Dockerを含む状態変更は、同じ端末のGo Runner APIだけが実行する。ユーザー向けCLIは提供しない。

## 結論

**Web UIは学習と明示操作の画面、Go Runnerは安全な実行境界である。** ブラウザはDockerへ直接接続せず、任意のCompose、Docker引数、パス、イメージ、Git URL、コマンドをRunnerへ渡せない。

最初の縦断対象はFile Browser CVE-2026-54088だけとする。教材の説明、Labの準備と明示起動、脆弱版・修正版の比較、停止・削除までをWebで完結させる。Apache HTTP Serverや追加CVEは、この対象を完走してから別Issueで扱う。

## アーキテクチャ

```text
                   Reprokit（ローカル環境）
┌────────────────────────────────────────────────────────┐
│ Browser / Web UI                                        │
│ 教材・手順・明示操作                                    │
└───────────────────────────┬────────────────────────────┘
                            │ 固定IDと定義済み操作だけ
                            ▼
┌────────────────────────────────────────────────────────┐
│ Local Web Runner（Go, 127.0.0.1限定）                   │
│ Lesson API / Runner API / Go Core                        │
│ Registry / Manifest / Lockfile / Security / Runtime      │
└───────────────────────────┬────────────────────────────┘
                            ▼
                    Local Docker Compose
```

- **Local Web Runner**：Web UIとRunner APIをローカルの`127.0.0.1`だけで提供する。配布時はアプリケーションの管理プロセスとして起動し、利用者がLab操作用のCLIを実行する方式は採用しない。
- **Go Core**：Registry、Manifest、Lockfile、セキュリティ検査、Labの取得・準備・起動・停止・削除を実装する。Runner API以外にLabの状態を変更する入口を作らない。
- **Web UI**：教材、手順、ヒント、期待結果、原因・対策、Lab状態、片付けを扱う。操作リクエストにはLessonの固定参照、Runner発行Lab Session ID、定義済みの確認値・Volume方針だけを使う。
- **Docker Compose**：学習者のローカル環境上でだけ動作する。共有インターネットサーバーに意図的に脆弱なLabを起動しない。

## 操作フロー

1. Web UIが公開済みLesson内の`labRef`を選ぶ。
2. Runner APIが固定Labを解決し、取得、Lockfile生成、Compose安全性検査を実行して`prepared`にする。この段階でコンテナは起動しない。
3. 利用者がWeb UIで注意事項を確認して明示起動する。Web UIはRunner発行Lab Session IDと定義済み確認値だけを送る。
4. Runner APIがManifest、Lockfile、解決済みCompose、イメージdigest、安全性検査結果を再確認してからDocker Composeを起動する。
5. Web UIはloopback接続先、コンテナ状態、安全性概要、失敗理由を表示する。
6. 利用者がWeb UIから停止または削除を選ぶ。Runner APIは対象Projectだけを操作し、Volumeの保持・削除結果と残存リソースを返す。

## ローカルWebサーバーのセキュリティ

- APIとWeb画面はIPv4 `127.0.0.1`だけで待ち受ける。外部公開、リモート制御、無条件CORSをv0.1では提供しない。
- Host検証、Origin検証、CSRF対策、ランダムなローカルセッション認証を適用する。セッショントークンをURLやログへ記録しない。
- 状態変更APIはPOSTまたはDELETEに限定し、明示確認が必要な操作を分離する。
- APIはManifest、Lockfile、Runtime、Compose許可リストの検査を迂回できない。
- 教材Markdownをサニタイズし、Content Security Policyを設定する。

## MVPの実装順序

1. File Browser CVE-2026-54088のLessonと脆弱版・修正版Lab参照を確定する。
2. 固定取得、安全性検査、Prepare/Start/Status/Stop/Deleteを持つRunner APIを実装する。
3. Web UIで説明、明示起動、観察、原因・対策、片付けを完走できるようにする。
4. 無害なfixtureで、検査失敗・起動失敗・残存リソースを含むE2Eを検証する。
