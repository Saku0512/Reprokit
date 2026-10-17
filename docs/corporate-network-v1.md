# 企業ネットワーク接続 v1

## 目的

承認済みかつ版固定済みのLabだけを、企業プロキシやプライベートコンテナレジストリを使う環境でも取得・実行できるようにする。接続設定は再現対象ではないため、Lockfile・Session・Lesson・ログへ保存しない。

## 接続設定

Coreを組み立てる側は`internal/network.Config`を明示して、`lab.NewMaterializerWithNetwork`および`runtime.NewDockerComposeProviderWithNetwork`へ渡す。設定できる値は次の通り。

- `HTTPProxy` と `HTTPSProxy`: `http://`または`https://`のプロキシURL
- `NoProxy`: プロキシを使わないホスト名・ドメイン接尾辞
- `RegistryCredentials`: レジストリのホスト名、ユーザー名、パスワードまたはトークン

継承した`HTTP_PROXY`、`HTTPS_PROXY`、`NO_PROXY`、`ALL_PROXY`は子プロセスへ渡さない。設定値だけをGitとDocker CLIへ適用する。これは実行環境が意図しないプロキシを使わないための規則であり、プロキシURLに認証情報を含める場合も永続化してよいという意味ではない。

Dockerイメージの取得はDocker **daemon**が行う。Docker daemonが企業プロキシを必要とする場合は、ホスト管理者がdaemonのプロキシ設定も行う必要がある。ReprokitはComposeサービスの環境変数へプロキシを注入しない。注入すると固定済みComposeの内容を変えてしまうためである。

## プライベートレジストリ認証

Registry認証情報は、Dockerコマンドごとに作成する0700の一時ディレクトリ内の`config.json`（0600）へだけ書き込む。Dockerには`--config`でそのディレクトリを渡し、終了後は削除する。

- 認証情報をコマンド引数、Lockfile、Session、教材、エラーメッセージへ含めない。
- 同じレジストリへ複数の認証情報を与えることは拒否する。
- Registryホスト以外のURL・パス・クエリ・ユーザー情報は拒否する。
- Git取得の認証はこの版の対象外である。教材Sourceは既存のHTTPS・固定コミット制約を維持する。

## 接続失敗

Git取得またはイメージ取得の通信失敗は、秘密情報を表示せず、対象ホストと「プロキシ設定、DNS、レジストリ認証、またはネットワーク接続を確認する」旨を返す。TLS・プロキシ・名前解決・タイムアウト・認証失敗の詳細な診断は、認証情報を含まないホスト側のDocker/Gitログで管理する。

## オフライン配布

`runtime.Manager.OfflineArtifacts`は、Prepare済みSessionの検証済みLockfileから以下を返す。

- 固定Git URLとコミットSHA
- サービスごとのコンテナイメージ参照、digest、platform

外部通信できない端末へ配布する運用では、接続可能な管理環境で同じ固定コミットのGitスナップショットとdigest固定OCIイメージを取得して検証し、信頼できる媒体または社内アーティファクト保管庫を通じて配布する。受け側では配布物のSHA-256とLockfileを照合する。

v1では次を行わない。

- 任意のイメージ名・タグ・レジストリを受講者が入力して取得すること
- Lockfileのdigestをタグへ戻して再解決すること
- オフライン媒体の自動import、未検証tarの自動実行、プロキシ設定のComposeサービスへの注入
- 認証情報を含む設定ファイルの配布

完全なオフライン起動には、固定したGitソース、全イメージdigest、Docker Engine/Composeの要件をあらかじめ満たす必要がある。Dockerを隔離境界として扱わないという既存の安全性要件は変わらない。
