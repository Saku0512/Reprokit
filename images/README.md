# Labイメージの定義

このディレクトリは、Lab用コンテナイメージのビルド定義をCVE・製品バージョンごとに管理する。

| ディレクトリ | 用途 | 状態 |
| --- | --- | --- |
| `CVE-2026-54088/2.63.5` | File Browserの脆弱版 | 既存Labのイメージ定義 |
| `CVE-2026-54088/2.63.6` | File Browserの修正版 | 既存Labのイメージ定義 |
| `CVE-2026-71557/5.19.1` | go-gitの脆弱版 | Lab候補 |
| `CVE-2026-71557/5.19.2` | go-gitの修正版 | Lab候補 |

各定義は[Saku0512/vulhubの固定コミット `00dff09cf33abbc0099cc16c8b4298c4921d93aa`](https://github.com/Saku0512/vulhub/tree/00dff09cf33abbc0099cc16c8b4298c4921d93aa/base)の`base/`から取得した。原著作権表示とライセンスは[LICENSE.vulhub](LICENSE.vulhub)に記載する。

現在のRunnerは引き続きRegistryで固定したVulhub forkのComposeと公開済みイメージdigestを使用する。このディレクトリへの追加はRunnerの参照先や公開済みイメージを変更せず、go-git Labを利用可能にするものでもない。ビルド定義を更新した場合は出典・依存関係・安全性をレビューし、イメージを昇格してdigestを確認した上で、RegistryとLessonを別途更新する。

各Dockerfileのビルドコンテキストは対応するバージョンディレクトリである。go-gitのDockerfileはビルド時に`go mod tidy`で依存関係を解決するため、現在の定義だけでは依存関係や生成イメージの完全な再現性は保証されない。公開済みdigestとの一致も未検証である。これらの定義は脆弱な環境を含むため、レビュー前の自動実行や通常の検証には使わない。
