# Labイメージの定義

このディレクトリは、Lab用コンテナイメージのビルド定義をCVE・製品バージョンごとに管理する。

| ディレクトリ | 用途 | 状態 |
| --- | --- | --- |
| `CVE-2026-54088/2.63.5` | File Browserの脆弱版 | 既存Labのイメージ定義 |
| `CVE-2026-54088/2.63.6` | File Browserの修正版 | 既存Labのイメージ定義 |
| `CVE-2026-71557/5.19.1` | go-gitの脆弱版 | Lab候補 |
| `CVE-2026-71557/5.19.2` | go-gitの修正版 | Lab候補 |

File Browserのイメージはコミット`8c7e9fd9a4ec56c82d9747e10ebba29fc03111c5`の定義から`linux/amd64`向けにビルドし、Docker Hubへ公開した。次のdigestは匿名で参照できることを確認済みである。

| バージョン | 公開済みイメージ |
| --- | --- |
| 2.63.5 | `docker.io/saku0512/reprokit_cve-2026-54088@sha256:cdf1d4e63e4630f0f473e7f9cfd381c20450e65a87b86bceb9c944883ff6cd95` |
| 2.63.6 | `docker.io/saku0512/reprokit_cve-2026-54088@sha256:22164b8c14c41f5aa4556f21442af35bc460a7d5ff85e262579fd37d5e3205a5` |

既存のFile Browser Labは引き続きVulhub forkの固定Composeに記載されたGHCRイメージを参照する。Docker Hubのイメージへ切り替えるには、イメージと構成をレビューし、固定ComposeとRegistryの参照を更新する。

go-gitのイメージはコミット`8c7e9fd9a4ec56c82d9747e10ebba29fc03111c5`の定義から`linux/amd64`向けにビルドし、Docker Hubへ公開した。次のdigestは匿名で参照できることを確認済みである。

| バージョン | 公開済みイメージ |
| --- | --- |
| 5.19.1 | `docker.io/saku0512/reprokit_cve-2026-71557@sha256:2c32850969c685ccac825ea5479fa86f8d319223cb11aae4cb55bc0361447267` |
| 5.19.2 | `docker.io/saku0512/reprokit_cve-2026-71557@sha256:81b410dd1c0418ebd680ed90341695e37581326db4812c8f362beb0c72832e97` |

これらはLab候補のイメージであり、ReprokitのRegistryには未登録である。Vulhub fork側のComposeはまだ旧イメージを参照する。環境構成とイメージをレビューしてから固定参照を更新する。

各定義は[Saku0512/vulhubの固定コミット `00dff09cf33abbc0099cc16c8b4298c4921d93aa`](https://github.com/Saku0512/vulhub/tree/00dff09cf33abbc0099cc16c8b4298c4921d93aa/base)の`base/`から取得した。原著作権表示とライセンスは[LICENSE.vulhub](LICENSE.vulhub)に記載する。

現在のRunnerは引き続きRegistryで固定したVulhub forkのComposeと公開済みイメージdigestを使用する。このディレクトリへの追加はRunnerの参照先や公開済みイメージを変更せず、go-git Labを利用可能にするものでもない。ビルド定義を更新した場合は出典・依存関係・安全性をレビューし、イメージを昇格してdigestを確認した上で、RegistryとLessonを別途更新する。

各Dockerfileのビルドコンテキストは対応するバージョンディレクトリである。go-gitのDockerfileはビルド時に`go mod tidy`で依存関係を解決するため、現在の定義だけでは依存関係や生成イメージの完全な再現性は保証されない。公開済みdigestとの一致も未検証である。これらの定義は脆弱な環境を含むため、レビュー前の自動実行や通常の検証には使わない。
