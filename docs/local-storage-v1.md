# ローカル環境管理 v0.1.0

## 対応OSと保存先

初期の正式対応は**Linux**。XDG Base Directoryに準拠し、次の変数を優先する。

| 用途 | 環境変数 | フォールバック |
| --- | --- | --- |
| 設定 | `XDG_CONFIG_HOME` | `~/.config` |
| ユーザーデータ | `XDG_DATA_HOME` | `~/.local/share` |
| キャッシュ | `XDG_CACHE_HOME` | `~/.cache` |
| 状態・ログ | `XDG_STATE_HOME` | `~/.local/state` |

```text
~/.config/reprokit/config.yaml
~/.local/share/reprokit/
  labs/
    log4shell-test/
      reprokit.yaml
      reprokit.lock
      resolved.compose.yaml
      source/
      instance.json
~/.cache/reprokit/
  registry/
  sources/
~/.local/state/reprokit/
  logs/
```

**上記はXDG変数が未設定の場合の配置例。** 実際のパスは環境により異なる。`config.yaml`にはレジストリURLなどの非機密設定だけを保存し、秘密情報は平文保存しない。

## 環境の識別

- CVE ID：検索キー。複数Labを指し得る。
- Lab ID：Manifestの`metadata.id`。環境定義の識別子。
- Instance Name：利用者が指定する一意なローカル名（例`log4shell-test`）。
- Instance ID：`create`ごとに生成するUUID。Compose Project名（例`reprokit_<uuid由来の短い識別子>`）とラベル照合に用いる。

同じCVEやLabから別のInstance Nameで複数インスタンスを作成できる。名前重複は拒否。

## キャッシュと削除

- 取得ソースは**Git URL + commit SHA**を基準に共有キャッシュ化。RegistryはGit SHAでスナップショットを保存し、検索時キャッシュTTLを24時間とする。
- ソースキャッシュをインスタンス内へ配置するときは、各インスタンスが変更しても他に波及しないよう分離する。
- 作成は一時ディレクトリで検証・ロック生成後に原子的に配置する。途中失敗時に壊れたインスタンスを登録しない。
- `delete`はComposeを停止・削除してインスタンス内ファイルを除去するが、共有キャッシュは保持。
- Dockerのnamed volumeは既定では保持。`delete --volumes`でそのインスタンスに属するVolumeを明示的に削除する。
- 他のCompose projectや他のユーザーのコンテナ、Volume、キャッシュは削除しない。所有ラベルとInstance IDを照合して操作する。

## 同時実行と整合性

- インスタンスごとの排他ロックで`start`・`stop`・`delete`の同時実行を防ぐ。
- `list`と`info`は記録だけに依存せずDocker側の実状態を参照し、手動操作によるドリフトを明示する。
- 削除や起動失敗があればエラーとログを残し、途中状態を成功扱いしない。

