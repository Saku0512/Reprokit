# Reprokit v0.1.0 採用設計

> 2026-10-14 時点の**採用仕様**。設計を採用したものであり、各機能が実装済みであることを意味しない。既存のルート `README.md` は変更しない。

## スコープ

ReprokitはCVEをキーとして検証環境を検索・取得・固定・実行管理するGo製CLI。脆弱性スキャナーやExploit生成ツールではない。

**データの流れ：** `Registry → Manifest → Lockfile → Docker Compose`

| 項目 | v0.1.0 採用仕様 | 文書 |
| --- | --- | --- |
| Manifest | YAML / `reprokit.dev/v1alpha1` / Gitコミット固定 / Compose専用 | [Manifest](manifest-v1alpha1.md) |
| Registry | GitHubの静的JSON索引、Manifestから生成、キャッシュ24h、Git SHAでスナップショット識別 | [Registry](registry-v1.md) |
| Lockfile | `reprokit.lock`（YAML）、Manifest/ソース/Compose/イメージを固定、変更時は起動拒否 | [Lockfile](lockfile-v1.md) |
| CLI | `search/create/info/start/stop/delete/list/registry update`、複数候補は対話選択 | [CLI](cli-v1.md) |
| ローカル保存 | LinuxのXDG規約に従い設定・環境・キャッシュ・ログを分離 | [Storage](local-storage-v1.md) |
| 実行 | Docker Compose projectごとに隔離、デタッチ、公開ポートは`127.0.0.1`に制限 | [Runtime](runtime-v1.md) |
| 安全対策 | 未対応/危険な設定を拒否し、Dockerは完全な隔離境界とみなさない | [Security](security-v1.md) |
| Provider | v0.1は共通Git取得処理。専用Provider拡張はv0.2以降 | [Provider](provider-v1.md) |
| 検証 | ライフサイクルと検証レベルを分離。起動/TCPの基本チェックまで | [Verification](verification-v1.md) |

## リポジトリ上の予定構成

```text
cmd/reprokit/          # Go CLI（今後実装）
internal/{cli,registry,manifest,resolver,lockfile,runtime,security,storage}/
registry/index.json
registry/manifests/vulhub/
schemas/manifest-v1alpha1.schema.json
schemas/registry-v1.schema.json
schemas/lock-v1.schema.json
examples/
docs/
tests/                # 今後実装
```

## v0.1.0の完了条件

- CVE IDで検索し、複数候補があれば環境を選択できる。
- Manifestの厳格なバリデーションとGit固定コミットでのソース取得ができる。
- 取得した構成から実効Composeを決定し、利用する全イメージをdigest固定できる。
- Manifest・ソース・Compose・イメージの整合性を検査して起動できる。
- Docker Composeの危険な構成を拒否でき、公開ポートをループバックへ限定できる。
- インスタンスを複数管理し、起動・停止・削除・Volume削除を明示的に操作できる。
- TCP到達性でBootable判定でき、Verifiedと混同しない。

## 仕様の優先順位

- Manifestの形状は[ManifestのJSON Schema](../schemas/manifest-v1alpha1.schema.json)を正本とする。
- Registryの形状は[RegistryのJSON Schema](../schemas/registry-v1.schema.json)を正本とする。
- Lockfileの形状は[LockfileのJSON Schema](../schemas/lock-v1.schema.json)を正本とする。
- CLI動作やComposeに対する意味検証は、このドキュメント群の規則を参照する。
- [概要資料](reprokit-overview.md)は元のREADMEの保存コピーであり、古いManifest例等は本仕様に優先しない。

## 追加確定：実装前の7項目（2026-10-14）

1. Composeの**許可リスト**を採用。image・固定TCP ports・固定値environment・短形式depends_on・named volumes・project専用Bridgeのみサポート。詳細は[Runtime](runtime-v1.md)。
2. GitHub APIで公式RegistryのmainコミットSHAを一度だけ解決。index/Manifestを同じSHAから取得し、キャッシュ24時間。明示的`--offline`対応。[Registry](registry-v1.md)
3. TCPチェックは短命な専用コンテナで内部Bridge経由に実行。`alpine:3`のdigestも固定。[Verification](verification-v1.md)
4. Lockfileに参照ファイル`source.files`のSHA-256、チェック用イメージ`checker`を追加。[Lockfile](lockfile-v1.md)
5. Volumeは既定保持し、削除後の所有関係を`volumes.json`に保存。[Local Storage](local-storage-v1.md)
6. `create`はステージングと原子的rename。`start`失敗は対象Projectだけロールバック。[Local Storage](local-storage-v1.md)
7. Go 1.27.2、Linux、Docker Engine 28以上、Compose v2.36.1以上、GitHub Actionsのテスト方針を採用。[開発・CI](development-testing-v1.md)

これらは**仕様の採用**であり実装完了ではない。
