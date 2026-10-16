# Provider / 取得方式 v0.1.0

## 採用方針（2026-10-16更新）

- **標準のLabカタログはVulhub**とする。
- Vulhubに未収録のCVEは、Reprokitが管理する**Vulhub互換Lab**で補完する。
- v0.1ではVulhub専用CLIや専用ネットワークプロトコルを作らず、Manifestで指定されたHTTPS Gitリポジトリから取得する共通処理を使う。
- `spec.source.type`は`git`のみ。`url`、固定`revision`、`subdirectory`を共通インターフェースとして扱う。
- カスタムProviderプラグイン、独自取得プロトコル、外部Registryの動的追加はv0.2以降に延期する。

Vulhubは検証環境を収録するGitリポジトリであり、Labを起動するCLIではない。「内部的にVulhubを使う」とは、ReprokitがVulhubの固定コミットと対象ディレクトリを取得し、自身の安全性検査とDocker Compose Runtimeで実行することを指す。

## Labの2つの提供形態

### Vulhub由来Lab

- `https://github.com/vulhub/vulhub.git`の固定コミットを参照する。
- `subdirectory`は例として`httpd/CVE-2021-41773`のようなCVE環境ディレクトリ。
- 上流README、Dockerfile、Compose、ライセンスを出典とともに表示する。
- 上流ComposeがRuntime許可リストに適合しない場合は、元ファイルを直接実行せず互換アダプターを使用する。

### Vulhub互換Lab

- Reprokitが管理するGitリポジトリ内に、`<product>/<CVE-ID>/`単位で配置する。
- 最低限、`docker-compose.yml`、環境説明、上流製品・アドバイザリ・修正差分への参照を持つ。
- Vulhub由来Labと同じManifest、Lockfile、安全性検査、Runtimeを使用する。
- 将来Vulhubへ提案できる構造を保つが、採用やマージをMVPの前提にしない。

## 初期教材の割り当て

| CVE | 形態 | 取得元 |
| --- | --- | --- |
| CVE-2021-41773 | Vulhub由来Lab + 互換アダプター | Vulhub `httpd/CVE-2021-41773`の固定コミット |
| CVE-2026-54088 | Vulhub互換Lab | Reprokit管理のFile Browser環境 |

## 取得フロー

1. Registryから`metadata.id`でManifestを特定する。
2. Manifestのスキーマ、CVE・Lab IDの一致、HTTPS Git URL、固定`revision`を検証する。
3. Git URLとコミットSHAをキーにキャッシュを参照し、存在しなければ取得する。
4. 指定コミットを検証し、`subdirectory`配下を独立したステージング領域へ複製する。
5. 上流由来情報、互換アダプターの差分、参照先の範囲外リンクを検査する。
6. Composeの許可リスト検査、イメージdigest解決、Lockfile生成を行う。
7. すべて成功した場合のみインスタンスを原子的に登録する。`create`では起動しない。

## ProviderとLab IDの違い

`vulhub/httpd-cve-2021-41773`の`vulhub`や`reprokit/filebrowser-cve-2026-54088`の`reprokit`はLab IDの名前空間であり、実行時に同名のプラグインが必要という意味ではない。取得元の種類は表示・監査用メタデータとして扱い、実際の取得は共通Git処理へ収束させる。

## キャッシュと障害時の責任分担

ソースは`Git URL + revision`で共有キャッシュを利用できるが、作成中のインスタンスは独立したステージング領域で検証する。取得・検証・digest解決・Lockfile生成がすべて成功した時だけ登録し、失敗時は未完成のインスタンスを残さない。
