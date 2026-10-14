# Reprokit

**CVE IDから脆弱性の検証環境を取得・管理する、再現性重視のパッケージマネージャー。**

Reprokitは、公開された脆弱性をローカルで検証するための環境を、統一されたCLIで検索・取得・起動・停止・削除できるようにするOSSプロジェクトです。

例えば `reprokit create CVE-2021-44228` のようにCVE IDを指定すると、対応する検証環境を探してローカルに取得します。脆弱なソフトウェアのバージョンや依存関係を調べ、環境を手作業で組み立てる負担を減らすことが目標です。

> [!IMPORTANT]
> **現在は設計・開発初期段階です。** このREADMEに記載したCLI、ファイル形式、検証機能は設計案であり、**まだ実装済みではありません**。仕様は開発に伴って変更される可能性があります。

## Reprokitが目指すもの

- **CVE-first:** 検証環境の提供元を知らなくても、CVE IDから探せる。
- **複数の提供元を統一:** [Vulhub](https://github.com/vulhub/vulhub)など既存の検証環境と、将来のコミュニティ提供環境を同じ方法で扱う。
- **環境を固定して記録:** ソースコードのコミット、コンテナイメージのdigest、構成ファイルのハッシュなどを残す。
- **取得と実行を分離:** 環境を取得するだけでは脆弱なサービスを起動しない。
- **ライフサイクル管理:** 一度取得した環境を確認・起動・停止・削除できる。

Reprokitは**脆弱性スキャナーでも、自動的に攻撃コードを生成するツールでもありません**。また、指定したCVEに必ず検証環境が存在するわけではありません。

## 想定する使い方

以下のコマンドは**すべて未実装の利用イメージ**です。

```bash
# CVEに対応する環境を検索する
reprokit search CVE-2021-44228

# 環境をローカルへ取得する（自動起動しない）
reprokit create CVE-2021-44228

# 取得元・必要な権限・公開ポートなどを確認する
reprokit info CVE-2021-44228

# 確認後、明示的に起動する
reprokit start CVE-2021-44228

# 管理している環境を一覧表示する
reprokit list

# 停止・削除する
reprokit stop CVE-2021-44228
reprokit delete CVE-2021-44228
```

同じCVEに複数の検証環境が見つかった場合は候補を表示して選択できる設計とします。また、将来的にはCVE IDの代わりに検証環境の固有IDを指定できるようにします。

## アーキテクチャ

Reprokitの中心は **Registry → Manifest → Lockfile → Runtime** の4段階です。

```text
ユーザー
  |
  | reprokit create CVE-2021-44228
  v
+-----------------------+
| Reprokit CLI (Go)     |
+-----------------------+
  |
  v
+-----------------------+
| Registry / Resolver   |  CVE ID → 検証環境の候補
+-----------------------+
  |
  v
+-----------------------+
| Manifest              |  環境の定義・取得元・実行条件
+-----------------------+
  |
  v
+-----------------------+
| Lockfile              |  コミット・digest・ハッシュを固定
+-----------------------+
  |
  v
+-----------------------+
| Local Manager         |  取得・検査・保存・起動・停止・削除
+-----------------------+
  |
  v
Docker Compose（初期対応）
```

### 1. Registry — CVEと環境を紐付ける

Registryは、CVE IDから利用可能な検証環境を探すための**索引**です。検証環境の実体をReprokitのサーバーに再配布する必要はなく、Vulhubなどの元リポジトリを参照します。初期段階では、GitHub上で管理する静的なインデックスを想定しています。

**CVEと検証環境は多対多で管理します。**

- 1つのCVEを再現する環境が複数存在する場合がある。
- 1つの環境で複数のCVEを検証できる場合がある。
- CVE IDは検索キーであり、環境自体の一意なIDではない。

例えば、次のようなインデックスを**Manifestから生成**します（構造は仮案）。

```json
{
  "schemaVersion": 1,
  "cve": "CVE-2021-44228",
  "labs": [
    {
      "id": "vulhub/log4j-solr-2021-44228",
      "manifest": "labs/vulhub-log4j-solr.yaml"
    }
  ]
}
```

ファイル名やREADMEからCVE IDを抽出する仕組みは、登録候補の発見には利用できます。ただし、**機械的な名前の一致だけで「検証可能」とは判定しません**。正式な紐付けはManifestで明示します。

将来的には公式Registryに加え、コミュニティや組織が管理するRegistryを追加可能にする方針です。

### 2. Manifest — 検証環境を定義する

Manifestは、環境固有のID、対応CVE、取得元、実行方式、対象ソフトウェア、確認項目などを宣言します。

以下は[VulhubのLog4Shell検証環境](https://github.com/vulhub/vulhub/tree/master/log4j/CVE-2021-44228)を題材にした**設計例**です。実在するファイルの内容をそのまま示したものではありません。

```yaml
schemaVersion: 1

id: vulhub/log4j-solr-2021-44228
name: Log4Shell on Apache Solr

cves:
  - CVE-2021-44228

source:
  type: git
  url: https://github.com/vulhub/vulhub.git
  commit: "<検証した40桁のコミットSHA>"
  path: log4j/CVE-2021-44228

runtime:
  type: docker-compose
  file: docker-compose.yml
  platform: linux/amd64

target:
  product: Apache Solr
  version: "8.11.0"
  vulnerableComponent:
    name: Log4j
    version: "2.14.1"

checks:
  - type: tcp
    port: 8983
```

`source.commit` にはブランチ名ではなく、実際に確認したコミットSHAを記録します。`checks` は起動・到達性を確認するためのものであり、**TCPポートに接続できても脆弱性の再現成功を意味しません**。

初期実装はDocker Composeに絞り、他のRuntimeや独自形式の環境定義は拡張対象とします。

### 3. Lockfile — 依存関係と構成を固定する

Gitのコミットを固定しても、Composeが参照するDockerイメージのタグが更新される可能性があります。

そこで、環境を取得した時点で解決した情報を`reprokit.lock`に保存します。

```yaml
lockVersion: 1
lab: vulhub/log4j-solr-2021-44228

source:
  commit: "<実際のコミットSHA>"

files:
  composeSha256: "<実際の構成ファイルSHA256>"

images:
  solr:
    digest: "<sha256:で始まる実際のdigest>"
    platform: linux/amd64

runtime:
  type: docker-compose
```

上記のSHAとdigestはプレースホルダーです。実際のlockfileには解決済みの値を記録し、起動時は**タグではなく固定したdigest**を使う方針です。

さらに、環境変数の展開、Composeの実効設定、初期データなど、動作に影響する情報も必要に応じて記録します。

ただし、lockfileだけで完全な再現性が保証されるわけではありません。ホストOS・CPUアーキテクチャ・Docker/Composeのバージョン・外部ネットワークやサービス・時刻・外部データなども結果に影響します。Reprokitは、**何を固定し、どの条件で動いたかを追跡可能にすること**を重視します。

### 4. Runtime — ローカル環境を管理する

初期Runtimeは**Docker Compose**を採用します。

`create` ではManifestの取得、ソースの固定、依存関係の解決、lockfileの生成、構成の事前検査とローカル保存までを行い、コンテナは起動しません。

`start` ではlockfileとの整合性や安全上の問題を確認してから起動します。`stop` は停止、`delete` は管理対象の環境を削除します。

## 検証状態の区別

「環境を見つけた」「同じ構成で起動できた」「脆弱性を再現できた」はそれぞれ別の意味です。

| 状態 | 意味 |
| --- | --- |
| `Indexed` | CVEと検証環境の対応が登録されている |
| `Resolved` | 取得元やイメージなどを具体的なリビジョン・digestに固定できた |
| `Bootable` | 指定条件下で環境を起動でき、基本チェックを通過した |
| `Verified` | 対象CVEについて、定義された検証方法で期待結果を確認した |

MVPでは`Bootable`までの基本的な確認を目標とします。`Verified`は将来的に、検証手順、期待結果、実施日時、使用lockfile、結果の証跡を伴って扱う構想です。状態は環境一般に対する永続的な保証ではなく、**どの条件で確認したか**と一緒に記録します。

## セキュリティ上の注意

> [!WARNING]
> Reprokitが扱うのは**意図的に脆弱なソフトウェア**です。第三者が提供するComposeファイル、イメージ、Dockerfile、セットアップスクリプトは信頼できないコードとして扱う必要があります。

設計上、次の対策を重視します。

- `create` で脆弱なサービスやスクリプトを自動実行しない。
- 起動前に公開ポート・特権モード・ホストネットワーク・危険なホストマウント（例: Dockerソケット）などを検査する。
- 原則として公開ポートを`127.0.0.1`に制限し、外部公開が必要な場合は明示的に確認する。
- 取得ファイルのパストラバーサルやシンボリックリンクによる不正な書き込みを防ぐ。
- ソースの改変や構成の差異を検出できるよう、リビジョンとハッシュを記録する。
- コンテナを完全な隔離境界とみなさず、高リスクな検証はネットワークを制限した専用VM内で行う。

**これらは実装予定の安全策であり、現時点で実装・検証済みという意味ではありません。** Reprokitの利用は、自分が所有する環境や許可を得た検証環境に限定してください。

## 開発ロードマップ

### v0.1.0 — ローカルMVP

- [ ] Go製CLIの基本構成
- [ ] Vulhubの1つ以上の環境をCVE IDから解決
- [ ] Manifestの最小スキーマ
- [ ] GitコミットとDockerイメージdigestの固定
- [ ] `reprokit.lock`の生成・整合性確認
- [ ] 環境の取得・情報表示・起動・停止・一覧・削除
- [ ] Docker Composeの基本的な安全性検査
- [ ] 起動状態・到達性の基本チェック

### v0.2.0 — RegistryとProviderの拡張

- [ ] ManifestからCVEインデックスを自動生成
- [ ] 複数の検証環境候補の選択
- [ ] Providerの追加を容易にするインターフェース
- [ ] 公式の静的Registryと外部Registryの追加機能
- [ ] Manifestのスキーマ検証と登録ワークフロー

### v1.0.0 — 再現性・信頼性の向上

- [ ] 依存関係・初期データを含む固定範囲の拡張
- [ ] 完全性・取得元の検証
- [ ] 実行条件と再現結果の記録
- [ ] `Verified`向けの検証方式と証跡管理
- [ ] クロスプラットフォーム対応とドキュメント整備

バージョン番号と実装範囲は暫定です。

## 技術スタック（予定）

- **CLI:** Go
- **Runtime:** Docker Compose（初期対応）
- **Registry:** GitHub上の静的インデックス（初期案）
- **Manifest / Lockfile:** YAML（初期案）

## コントリビューション

現在は設計・開発初期段階です。仕様に関する議論、不具合報告、改善提案は[GitHub Issues](https://github.com/Saku0512/Reprokit/issues)で受け付けます。

## ライセンス

[MIT License](LICENSE)
