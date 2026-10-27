# 初期教材とLab提供方針

> **決定：2026-10-16。** 教育MVPの初期教材をCVE-2021-41773とCVE-2026-54088の2本に固定する。Labは[`Saku0512/reprokit`](https://github.com/Saku0512/Reprokit)を単一のカタログとして参照し、upstreamから継承した環境とforkへ追加した独自環境を同じ方法で扱う。

## 用語と実行方式

- **upstream由来Lab**：forkが`reprokit/reprokit`から継承した環境ディレクトリ。
- **fork追加Lab**：Reprokit Labに未収録のCVEについて、同じCVE単位レイアウトで`Saku0512/reprokit`へ追加する環境。
- **互換アダプター**：ComposeがReprokitの許可リストに適合しない場合、出典と差分を記録したうえで、安全な事前ビルド済みイメージと制限済みComposeへ変換する定義。

Reprokit Labは検証環境のリポジトリであり、実行用CLIではない。ReprokitのGo CoreがRegistry/Manifestから`Saku0512/reprokit`の`master`を一度コミットSHAへ解決し、同じSHAから対象ディレクトリを取得して、安全性検査後にDocker Composeを実行する。

upstream由来Labとfork追加Labは同じforkのスナップショット内にあり、Manifest、Lockfile、Runtime、安全性検査、明示起動、停止・削除の規則を共有する。所有するforkだからといって独自Labを無条件に信頼する例外経路は作らない。

## 教材1：CVE-2021-41773

| 項目 | 内容 |
| --- | --- |
| 製品 | Apache HTTP Server 2.4.49 |
| 分類 | パストラバーサル / 情報漏えい。条件によりRCE |
| 難易度 | 入門 |
| 提供元 | `Saku0512/reprokit`の`httpd/CVE-2021-41773`（upstream由来） |
| 初期演習 | コンテナ内に置いた無害なマーカーファイルの読み取り |
| 発展 | CGI有効時に影響が拡大する理由の説明。RCE操作は初期必須にしない |

### 説明する処理経路

1. HTTPリクエストのURLパスを受け取る。
2. エンコードされたパス要素をApacheが正規化する。
3. 脆弱な正規化により親ディレクトリ相当の要素が残る。
4. Alias配下のパスが想定ディレクトリの外へ解決される。
5. ディレクトリアクセス制御の条件を確認する。
6. コンテナ内の検証用ファイルがHTTPレスポンスへ返る。
7. 修正版の正規化と拒否動作を比較する。

fork内の元Composeはupstreamと同じく`build: .`を使用するため、v0.1のRuntimeでは直接実行しない。固定したDockerfileからレビュー済みイメージを事前ビルドし、digest固定した互換アダプターを使用する。教材画面にはforkコミット、upstream由来であること、元ディレクトリ、アダプターの差分を表示する。

## 教材2：CVE-2026-54088

| 項目 | 内容 |
| --- | --- |
| 製品 | File Browser 2.63.5 |
| 分類 | Hook Authenticationの未認証OSコマンドインジェクション |
| 難易度 | 初級〜中級 |
| 提供元 | `Saku0512/reprokit`へ追加済みの`filebrowser/CVE-2026-54088` |
| 比較対象 | 脆弱版2.63.5 / 修正版2.63.6 |
| 初期演習 | 隔離コンテナ内の検証用マーカーで、入力が危険な実行経路へ到達したことだけを確認 |

### 説明する処理経路

1. 未認証のログインリクエストからユーザー名・パスワードを受け取る。
2. Hook Authenticationが資格情報を保持する。
3. 脆弱版が`os.Expand`で資格情報をフックコマンドの引数へ展開する。
4. 管理者設定がシェル等のインタープリターを呼ぶ場合、展開後の値がコードとして解釈され得る。
5. 認証結果を確定する前に外部コマンドが実行される。
6. 実行権限がFile Browserプロセスの権限になることを確認する。
7. 修正版ではコマンド引数への展開が削除され、資格情報が環境変数だけで渡されることを比較する。

このLabは`Saku0512/reprokit`の`filebrowser/CVE-2026-54088`へ追加済みで、脆弱版2.63.5、修正版2.63.6、loopback限定ポート、非破壊的なマーカーPoCを含む。上流File Browserの脆弱版ソース・公式アドバイザリ・修正差分を出典として記録する。

ただしDocker Hubの`reprokit/*`名前空間はupstream管理であり、fork所有者が独自イメージを公開できる場所ではない。Reprokit用の脆弱版2.63.5と修正版2.63.6は、forkの固定SHAから一度だけビルドして`ghcr.io/saku0512/reprokit-filebrowser`へ公開済み。利用時はタグでなく公開済みdigestを固定する。公開用workflowは成功確認後にforkから削除した。詳細なdigestと工程は[Lab昇格設計](lab-catalog-runtime-v0.1.md)を参照する。upstreamへは[PR #805](https://github.com/Saku0512/Reprokit/pull/805)を提出済みだが、マージや公式イメージ公開をMVPの前提にしない。

## Web UIの共通構成

各教材は、脆弱性の経路を一画面に詰め込まず、次のステップを1つずつ進める。

1. **前提**：製品、影響バージョン、必要な基礎知識。
2. **入力**：攻撃者が制御できる値と入口。
3. **変換**：デコード、正規化、展開など値が変化する処理。
4. **信頼境界**：外部入力が内部のファイルパスやコマンドへ移る地点。
5. **危険な処理**：ファイル読み取り、式評価、OSコマンド実行などのsink。
6. **観察**：無害なマーカーやレスポンスで結果を確認。
7. **修正比較**：脆弱版と修正版のコード・挙動の差。
8. **対策と片付け**：根本対策、緩和策、Labの停止・削除。

各ステップでは「現在の入力値」「現在のコンポーネント」「信頼レベル」「次に渡る値」を表示する。PoC全文だけを先に見せず、まず予想させ、必要に応じてヒントを段階表示する。

## MVPの完了条件

- 2教材がWeb UIから選択できる。
- 両教材で説明、環境取得、明示起動、安全な観察、原因、修正比較、停止・削除まで完走できる。
- 1つのforkスナップショットからupstream由来Labとfork追加Labを同じGo Coreの処理で扱える。
- 取得元のGit SHA、ディレクトリ、Manifest、Compose、イメージdigestをLockfileへ記録できる。
- 初学者テストで、利用者が処理経路と対策を自分の言葉で説明できる。

## 参照元

- [Reprokit用Reprokit Lab fork](https://github.com/Saku0512/Reprokit)
- [upstream Reprokit Lab: CVE-2021-41773](https://github.com/Saku0512/Reprokit/tree/master/httpd/CVE-2021-41773)
- [Apache HTTP Server security information](https://httpd.apache.org/security/vulnerabilities_24.html)
- [File Browser advisory GHSA-m93h-4hw7-5qcm](https://github.com/filebrowser/filebrowser/security/advisories/GHSA-m93h-4hw7-5qcm)
