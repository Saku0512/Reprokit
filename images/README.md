# Labイメージの定義

このディレクトリは、Lab用コンテナイメージのビルド定義をCVE・製品バージョンごとに管理する。

| ディレクトリ | 用途 | 状態 |
| --- | --- | --- |
| `CVE-2026-54088/2.63.5` | File Browserの脆弱版 | 既存Labのイメージ定義 |
| `CVE-2026-54088/2.63.6` | File Browserの修正版 | 既存Labのイメージ定義 |
| `CVE-2026-71557/5.19.1` | go-gitの脆弱版 | 実行可能Lab |
| `CVE-2026-71557/5.19.2` | go-gitの修正版 | 実行可能Lab |

File Browserのイメージはコミット`8c7e9fd9a4ec56c82d9747e10ebba29fc03111c5`の定義から`linux/amd64`向けにビルドし、Docker Hubへ公開した。次のdigestは匿名で参照できることを確認済みである。

| バージョン | 公開済みイメージ |
| --- | --- |
| 2.63.5 | `docker.io/saku0512/reprokit_cve-2026-54088@sha256:cdf1d4e63e4630f0f473e7f9cfd381c20450e65a87b86bceb9c944883ff6cd95` |
| 2.63.6 | `docker.io/saku0512/reprokit_cve-2026-54088@sha256:22164b8c14c41f5aa4556f21442af35bc460a7d5ff85e262579fd37d5e3205a5` |

File Browser Labは[固定Compose](CVE-2026-54088/docker-compose.yml)と上記のDocker Hubイメージを使う。RunnerはReprokitリポジトリ内の固定GitコミットからComposeを読み、安全性検査とLockfileの照合を行う。専用E2Eは実イメージの起動、脆弱版・修正版の挙動、停止と削除を検証する。

go-gitのイメージはコミット`8c7e9fd9a4ec56c82d9747e10ebba29fc03111c5`の定義から`linux/amd64`向けにビルドし、Docker Hubへ公開した。次のdigestは匿名で参照できることを確認済みである。

| バージョン | 公開済みイメージ |
| --- | --- |
| 5.19.1 | `docker.io/saku0512/reprokit_cve-2026-71557@sha256:2c32850969c685ccac825ea5479fa86f8d319223cb11aae4cb55bc0361447267` |
| 5.19.2 | `docker.io/saku0512/reprokit_cve-2026-71557@sha256:81b410dd1c0418ebd680ed90341695e37581326db4812c8f362beb0c72832e97` |

go-git Labは[Registry](../registry/manifests/reprokit/go-git-cve-2026-71557.yaml)とLessonに登録済みである。Runnerは固定コミットのCompose、上記のdigest、レビュー済みの`/usr/local/bin/poc`だけを使用する。実行E2Eは脆弱版の書き換え検出、修正版の拒否、リソース削除まで検証する。

第三者由来の原著作権表示とライセンスは[LICENSE-third-party.txt](LICENSE-third-party.txt)に保持する。

各Dockerfileのビルドコンテキストは対応するバージョンディレクトリである。go-gitのDockerfileはビルド時に`go mod tidy`で依存関係を解決するため、現在の定義だけでは依存関係や生成イメージの完全な再現性は保証されない。公開済みdigestとの一致も未検証である。これらの定義は脆弱な環境を含むため、レビュー前の自動実行や通常の検証には使わない。
