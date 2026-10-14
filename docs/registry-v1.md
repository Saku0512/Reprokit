# Registry v1 仕様

## 教育方針との関係

このRegistryは**Lab（検証環境）を検索する技術的索引**であり、学習教材の有無や難易度をまだ含まない。将来のLessonカタログは別に定義する。Labが登録されただけで学習教材が完成した扱いにはしない（[教材設計方針](learning-content.md)）。

## 方針

- v0.1は**GitHub上の静的Registry**を利用し、独自サーバーを作らない。
- 各Labは独立したManifestを持つ。CVE ↔ Labは多対多。
- `index.json`はManifestの`metadata.id`と`spec.cves`から**決定的に生成**する。逆方向の手動管理を正本にはしない。
- 登録・変更はPull Requestでレビューし、JSON Schemaと意味検証を行う。
- Registry更新を特定のGitコミットSHAに結び付ける。ブランチ名だけを永続的な識別子にしない。
- v0.1は公式Registry 1つだけを対象。外部Registry追加・優先順位はv0.2以降。

## レイアウト

```text
registry/
├── index.json
└── manifests/
    └── vulhub/
        └── log4j-solr.yaml
```

`index.json`例（初期データを同形式でリポジトリにも格納）：

```json
{
  "registryVersion": 1,
  "cves": {
    "CVE-2021-44228": ["vulhub/log4j-solr"]
  },
  "labs": {
    "vulhub/log4j-solr": {
      "manifest": "manifests/vulhub/log4j-solr.yaml"
    }
  }
}
```

- `labs`はLab ID → Registry直下からの相対Manifestパス。
- `cves`はCVE ID → Lab IDの**重複なし**の配列。
- JSONのキーと配列は辞書順で整列させ、同じManifest集合なら同じ出力にする。
- Manifestパスに絶対パス・親ディレクトリ参照・シンボリックリンク経由の領域外参照を許可しない。
- インデックスを利用する前にJSON Schema Draft 2020-12と整合性検証を行い、参照した各ManifestのID・CVEとの一致を確認する。
- CLIはManifestを**信頼しない入力**として扱う。パス検証・Git URL検証は別に行う。

## 更新・キャッシュ

| 設定 | 採用値 |
| --- | --- |
| 公開方式 | GitHubの静的ファイル |
| キャッシュTTL | 24時間 |
| スナップショット | GitコミットSHA |
| 登録手順 | PR → Schema/意味チェック → レビュー → マージ |
| 手動更新 | `reprokit registry update` |
| 複数Registry | v0.2以降 |

通常の検索では、キャッシュが24時間未満ならそのスナップショットを使用する。TTL経過後は新しいコミットを取得し直す。更新に失敗した場合は古いキャッシュを黙って最新版として扱わず、警告し、明示的なオフライン操作のときに限り使う方針とする。取得したManifestとRegistry SHAはローカル環境に記録する。

**現在の `registry/index.json` は初期サンプルです。** Manifestから生成するCLI/CIは今後実装する。

## インデックス生成手順（実装予定）

1. `registry/manifests/**/*.yaml`を収集し、Manifest JSON Schemaと意味検証を実行。
2. 重複Lab IDを拒否し、各`metadata.id`と`spec.cves`から双方向索引を生成。
3. JSONをキー・配列ともに安定順序で書き出す。
4. PRのCIでは生成結果とコミット済み`index.json`が一致するか検証する。

## 公式Registryの取得プロトコル（追加確定）

デフォルトの公開元は`jphacks/sd_2601`の`main`。取得時に`https://api.github.com/repos/jphacks/sd_2601/git/ref/heads/main`の`object.sha`を一度だけ解決し、以降は同じコミットに固定して次を取得する。

1. `https://raw.githubusercontent.com/jphacks/sd_2601/<commit>/registry/index.json`
2. `https://raw.githubusercontent.com/jphacks/sd_2601/<commit>/registry/<manifest-path>`

JSON SchemaとManifestのCVE/Lab ID照合、パス・URL・応答サイズ・タイムアウトの検証後、SHAで指定したスナップショットを原子的にキャッシュする。`main`を取得途中で再解決しない。Git SHAはバージョン識別子であって署名ではない。

キャッシュTTLは既定の**24時間**。`reprokit registry update`はオンラインで更新。更新失敗時に古いキャッシュを最新と偽らない。`search`/`create`の明示的`--offline`でのみ期限切れのキャッシュを利用可能とし、その時点とSHAを警告表示する。初回でキャッシュがなければエラーとする。
