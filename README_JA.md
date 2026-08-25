# bb — Bitbucket Cloud CLI

[English](README.md) | 日本語

`bb` は [GitHub CLI](https://cli.github.com/) の操作感を
[Bitbucket Cloud](https://bitbucket.org) に持ち込むコマンドラインツールです。
Go 製の単一バイナリで、`--json` / `--jq` / `--template` によるスクリプト連携に対応しています。

```
$ bb auth login
$ bb repo clone acme/widgets
$ bb pr create --title "Fix login" --fill
$ bb pr checkout 42
$ bb pr merge 42 --squash --delete-branch
$ bb pipeline watch 128
```

> **対象範囲:** `bb` は **Bitbucket Cloud（bitbucket.org）専用**です。Bitbucket
> Data Center / Server は REST API が異なるため対応していません。API のベース URL、
> Web URL、ブラウザ起動の許可リストは bitbucket.org に固定されています。

## インストール

```sh
# Homebrew（macOS / Linux）
brew install uehatsu/tap/bb

# ソースから（Go 1.22 以上）
go install github.com/uehatsu/bb/cmd/bb@latest
```

macOS / Linux / Windows 向けのビルド済みバイナリは各
[リリース](https://github.com/uehatsu/bb/releases)に添付されています。

## 認証

Bitbucket Cloud の **App Password** は 2026 年 7 月に廃止されました。`bb` は
**スコープ付き Atlassian API トークン**で認証します。

1. <https://id.atlassian.com/manage-profile/security/api-tokens> を開く
2. **Create API token with scopes** を選び、アプリに **Bitbucket** を指定して、
   下表のスコープを付与する。有効期限は必須（Atlassian の仕様）
3. `bb auth login` を実行し、Atlassian アカウントのメールアドレスとトークンを入力する

| スコープ | 用途 |
|---|---|
| `read:user:bitbucket` | `bb auth status`、`@me` フィルタ |
| `read:workspace:bitbucket` | ワークスペース / メンバー一覧（レビュアー解決） |
| `read:project:bitbucket` / `admin:project:bitbucket` | `bb project` |
| `read:repository:bitbucket` / `write:repository:bitbucket` | リポジトリ、ブランチ、ソース、clone |
| `admin:repository:bitbucket` / `delete:repository:bitbucket` | `bb repo create/edit/delete` |
| `read:pullrequest:bitbucket` / `write:pullrequest:bitbucket` | `bb pr *` |
| `read:pipeline:bitbucket` / `write:pipeline:bitbucket` | `bb pipeline *` |

非対話 / CI での利用:

```sh
echo "you@example.com:ATATT3x..." | bb auth login --with-token
# または
export BB_EMAIL=you@example.com BB_TOKEN=ATATT3x...
```

リポジトリ / プロジェクト / ワークスペース単位の **Access Token** にも対応しています。
Bearer トークンとして送信され、メールアドレスは不要です。

```sh
bb auth login --bearer --with-token < token.txt
# または: export BB_TOKEN=...（BB_EMAIL は設定しない）
```

### OAuth 2.0（`--web`）

Bitbucket には Device Flow が無く、クライアントシークレットが必須のため、
`bb auth login --web` は**自分で登録した OAuth consumer** を使います。
ワークスペース設定 → *OAuth consumers* → *Add consumer* で、コールバック URL を
`http://127.0.0.1:8976/callback`（ポートは `--port` / `bb config set oauth_port` で変更可）、
必要なスコープ（最低限 `account`）を設定してください。その後:

```sh
export BB_OAUTH_CLIENT_ID=... BB_OAUTH_CLIENT_SECRET=...
bb auth login --web            # ブラウザを開き、127.0.0.1 でコールバックを受け取る
bb auth refresh                # 手動でリフレッシュ（通常は自動）
```

アクセストークンの有効期間は 2 時間です。`bb` は保存済みのリフレッシュトークンで
自動更新します。`bb pipeline watch` のような長時間コマンドの途中でも、
git credential helper 経由でも更新されます。更新に失敗した場合は警告を 1 回だけ表示し、
30 秒ごとに再試行します。consumer のシークレットは資格情報と一緒に
（hosts.yml または keyring に）保存され、`config.yml` には書かれません。

### 資格情報の保存先

既定では `$XDG_CONFIG_HOME/bb/hosts.yml`（パーミッション 0600）に保存されます。
OS のキーチェーン（macOS Keychain、Windows Credential Manager、Linux の Secret Service）
を使う場合:

```sh
bb config set credential_store keyring   # または BB_CREDENTIAL_STORE=keyring
```

保存先を切り替えると、保存済みの資格情報は新しい保存先に自動で移行されるため、
再ログインは不要です。

### HTTPS での git 操作

`bb auth setup-git` は `https://bitbucket.org` に限定して `bb` を git の
credential helper として登録し、`git clone` / `push` でも同じトークンを使えるようにします。
（API トークンは固定ユーザー名 `x-bitbucket-api-token-auth`、Access Token は
`x-token-auth` を使いますが、`bb` が自動で使い分けます。）

Windows ではファイルのパーミッションが強制されないため、keyring か環境変数の利用を推奨します。

### SSH

Bitbucket は SSH の接続先を `bitbucket.org` から `ssh.bitbucket.org` へ移行中です
（期限 2026-11-12）。`bb` はリモート URL として両方のホストを受け付け、`git_protocol` が
`ssh` の場合は `ssh.bitbucket.org` の clone URL を生成します。

## コマンド

| bb | gh 相当 | 備考 |
|---|---|---|
| `bb auth login/logout/status/token/refresh/setup-git` | `gh auth …` | API トークン。Access Token は `--bearer`、OAuth は `--web` |
| `bb repo list/view/create/clone/fork/delete/edit` | `gh repo …` | `list` はワークスペース指定（省略時は所属ワークスペースを順に走査）。`--role` は `contributor\|admin\|owner` |
| `bb pr list/view/create/checkout/merge/decline/approve/unapprove/review/comment/diff/status/checks/edit/ready` | `gh pr …` | `close` は `decline` の alias。`reopen` は Bitbucket では不可 |
| `bb pipeline list/view/run/stop/watch/log`（`bb run` alias） | `gh run …` | |
| `bb branch list/create/delete` | — | |
| `bb workspace list/view/members` | `gh org …` | |
| `bb project list/view/create` | — | |
| `bb api <path>` | `gh api` | `{workspace}` / `{repo_slug}` プレースホルダ、`--paginate`、`-f` / `-F`、`--jq` |
| `bb browse` | `gh browse` | |
| `bb config get/set/list` | `gh config …` | `workspace`、`git_protocol`、`merge_strategy`、`credential_store`、`oauth_client_id`、`oauth_port`、`editor`、`pager`、`browser` |
| `bb issue` | `gh issue` | **利用不可** — Bitbucket の Issue トラッカーは 2026-08-20 に廃止 |

一覧 / 表示系のコマンドはすべて `gh` と同じく `--json <fields>`、`--jq <expr>`、
`--template <go-template>` に対応しています。利用可能なフィールドは、
`bb <cmd> --json` に存在しないフィールド名を渡すと一覧が表示されます。

### 対象リポジトリの決定

コマンドはカレントディレクトリの git リモート（`upstream` > `origin` > 最初の
Bitbucket リモート）のリポジトリを対象にします。`-R WORKSPACE/REPO` または
`BB_REPO=WORKSPACE/REPO` で上書きできます。`REPO` のみを指定した場合は
`workspace` 設定値が補われます。

### 環境変数

| 変数 | 用途 |
|---|---|
| `BB_TOKEN`、`BB_EMAIL`、`BB_AUTH_METHOD` | 資格情報（`api_token` \| `bearer`。未指定時は `BB_EMAIL` の有無で判定） |
| `BB_OAUTH_CLIENT_ID`、`BB_OAUTH_CLIENT_SECRET` | `bb auth login --web` 用の OAuth consumer |
| `BB_CREDENTIAL_STORE` | `file`（既定）\| `keyring` |
| `BB_REPO` | 対象リポジトリの上書き |
| `BB_CONFIG_DIR` | 設定ディレクトリ（既定 `$XDG_CONFIG_HOME/bb`） |
| `BB_PAGER`、`PAGER` | 長い出力に使うページャ |
| `BROWSER` | `--web` で使うブラウザコマンド |
| `BB_DEBUG=1` | HTTP リクエストをログ出力（秘密情報はマスク）。`=2` でレスポンス本文も出力（メールアドレス等の個人情報を含む場合あり） |
| `BB_NO_RETRY=1` | 429 / 5xx で再試行せず即座に失敗 |
| `NO_COLOR` | 色を無効化 |

## GitHub CLI との違い

- `bb issue` はありません（Bitbucket Issues は 2026 年 8 月に Atlassian により廃止）。
- `bb pr reopen` は「却下済み PR は再オープンできない」旨を案内します。
- `bb pr merge` は Bitbucket の 6 種類の戦略を `--strategy` で指定できます。
  `--merge` / `--squash` / `--rebase` はそれぞれ `merge_commit` / `squash` /
  `rebase_merge` に対応します。既定値は `bb config set merge_strategy` で設定でき、
  対話モードではその既定値が選択肢の先頭に表示されます。
- `bb pr decline --delete-branch` はフォーク側のブランチを削除しません。
  却下後に警告を表示し、終了コード 0 で終わります。
- `bb pr checks --watch` は TTY で表を再描画し、`--timeout` を受け付けます。
  `bb pipeline watch --exit-status=false` はパイプラインが失敗しても 0 を返します。
- PR の指定には番号、`#番号`、ブランチ名、bitbucket.org の PR URL が使えます。
  URL を指定した場合は常に URL 中のリポジトリが対象になります（gh と同じ）。
  他ホストの URL は拒否されます。
- 破壊的コマンド（`repo delete`、`branch delete`）は、非対話環境では `--yes` が必須です。
- レビュアー（`--reviewer`）はニックネームからサーバー側フィルタで解決し、
  使えない場合はワークスペースメンバーの走査にフォールバックします。
  `{uuid}` 形式はそのまま受け付けます。
- OAuth ログインには自前の OAuth consumer が必要です（Bitbucket は Device Flow と
  公開クライアント / PKCE に非対応）。設定不要で使えるのは API トークンです。

## 開発

```sh
make build      # bin/bb
make test       # ユニットテスト（httptest ベース。git があれば実 git も使用）
make lint       # golangci-lint が必要
make docs       # docs/reference を再生成（コミット対象。古いままだと CI が失敗）
```

実 API に対する統合テストは `BB_INTEGRATION=1` を指定したときのみ実行されます
（`BB_EMAIL` / `BB_TOKEN`、`BB_INTEGRATION_REPO=WORKSPACE/REPO` が必要）。
書き込み系のテストはさらに `BB_INTEGRATION_WRITE=1` と `BB_INTEGRATION_PR` および/または
`BB_INTEGRATION_COMMIT` が必要で、部分 PUT と `pipeline_commit_target` の前提、
ワークスペースメンバー API が `q=` フィルタを受け付けるかを検証します。

Windows ビルドはベストエフォートです。CI では Windows 上でビルドとユニットテストを
実行しますが、失敗してもリリースはブロックされません。Windows で `bb` を使う前に
手動で確認しておくとよい項目: `credential_store=keyring` での `bb auth login`、
`bb auth setup-git` 後の `git fetch`、スペースを含むパスへの `bb repo clone`。

## ライセンス

MIT
