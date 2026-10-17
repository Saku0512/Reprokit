# Lesson v1alpha1

Issue #7 の Lesson パッケージ形式。JSON Schema の正本は [`schemas/lesson-v1alpha1.schema.json`](../../schemas/lesson-v1alpha1.schema.json)。教材は UTF-8 JSON として保存する。

## 内容と識別子

- `metadata.id` は製品内で安定した `namespace/name`。`metadata.version` は 3 要素の数値版で、両者の組が Lesson Version を表す。公開後に内容または Lab 参照を変える場合は版を上げる。
- `metadata.title` は学習者に示す現象名。CVE ID だけをタイトルにしない。`spec.evidence.cves` は省略でき、CVE のない実在事例にも使える。
- `audience`、`prerequisites`、`durationMinutes`、`objectives` は学習前の案内に使う。`case` は実在製品、対象版、背景を記録する。
- `steps` は表示順に並び、操作説明、期待結果、観察点、段階的ヒントを持つ。`analysis`、`comparison`、`prevention` が原因、影響、修正、再発防止を扱う。`assessment` は設問、回答例、評価基準を持つ。回答例の閲覧権限は配信側で制御する。

## 出典と権利

- `evidence.references` に少なくとも 1 件の HTTPS 原典を記載し、`type`、`title`、`url` を埋める。出典が原因・影響・修正の記述を支えるかは人がレビューする。
- `license.name`、`license.url`、`license.attribution` を必須にする。これは教材に使う資料の権利情報であり、Lab や依存素材のライセンス確認を代替しない。

## Lab 参照と検証

- `labs[].ref` は Lesson 内の論理名、`labs[].labId` は Registry に登録された Lab Manifest の `metadata.id` と同じ形式。Lesson に取得先 Git URL、Compose パス、image tag、実行コマンドを持たせない。
- `steps[].labRef` は宣言済みの論理名を指す。`comparison.fixedLabRef` を使う場合は `purpose: compare-fix` の Lab を指す。
- JSON Schema は型、必須項目、未知の項目、文字列形式を検証する。`lessons/validate.js` はさらに参照整合性と `ref`、step ID、設問 ID の一意性を検証する。
- Registry に Lab ID が実在し、レビュー済みであることは公開時に Registry と照合する。Schema だけで公開可能と判断しない。Lab の取得と実行は Runner の安全性検査に従う。

ローカル検証:

```sh
npm ci --prefix lessons
npm test --prefix lessons
node lessons/validate.js lessons/fixtures/valid.json
```

`lessons/fixtures/valid.json` は形式の例であり、Lab 登録や教材の公開承認を示さない。
