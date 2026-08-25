# bb — Bitbucket Cloud CLI（GitHub CLI 互換の操作感）実装プラン

作成日: 2026-08-25 / 改訂: v2（MAGI 審議1回目の指摘を反映） / 対象: https://bitbucket.org (Bitbucket Cloud REST API 2.0)

## 0. 調査結果サマリ（プランの前提）

### 0.1 認証（2026-08 時点の事実）
| 方式 | 状態 | 使い方 |
|---|---|---|
| App Password | **廃止済み**（2025-09-09 新規作成停止 → 2026-06-09〜07-27 ブラウンアウト → **2026-07-28 完全削除**） | 使用不可。サポートしない |
| **API Token（スコープ付き）** | **現行の標準** | REST: `Authorization: Basic base64(<Atlassianアカウントのメール>:<token>)`。Bearer も可（メール不要）。git over HTTPS: ユーザー名は Bitbucket username または固定値 **`x-bitbucket-api-token-auth`** |
| Repository / Project / Workspace Access Token | 現行 | REST: `Authorization: Bearer <token>`。git over HTTPS: ユーザー名は固定値 **`x-token-auth`** |
| OAuth 2.0 | 現行 | authorize `https://bitbucket.org/site/oauth2/authorize`、token `https://bitbucket.org/site/oauth2/access_token`。Authorization Code / Client Credentials / JWT 交換。**Device Flow 非対応、PKCE 非対応、client_secret 必須**。Access token 寿命 2h、refresh_token あり。Consumer はワークスペース単位で作成し Callback URL 必須 |

- API Token 作成: `id.atlassian.com/manage-profile/security/api-tokens` → 「Create API token with scopes」→ App=Bitbucket → スコープ選択。**有効期限は必須**（期限切れは 401 で現れる）。
- API Token のスコープ名（新形式）: `read|write|admin|delete:repository:bitbucket`, `read|write:pullrequest:bitbucket`, `read|admin:project:bitbucket`, `read|admin:workspace:bitbucket`, `read|write:user:bitbucket`, `read|write|admin:pipeline:bitbucket`, `read|write|delete:webhook:bitbucket`, `read|write|delete:snippet:bitbucket`, `read|write|delete:ssh-key:bitbucket` ほか。
- OpenAPI 上の OAuth スコープは旧形式（`repository`, `repository:write`, `pullrequest:write`, `pipeline`, `account` …）。両者の対応表を README と `bb auth login` の案内に載せる。応答ヘッダからのスコープ取得は API Token では保証されないため、`bb auth status` はスコープを推測表示しない。

### 0.2 API 基本仕様
- Base URL: `https://api.bitbucket.org/2.0`（OpenAPI v3 定義: `https://developer.atlassian.com/cloud/bitbucket/swagger.v3.json`、173 paths。**ただし削除済みエンドポイントが残っているので鵜呑みにしない**）
- ページング: `{size, page, pagelen, next, previous, values}`。**`next` の URL をそのまま辿る**（page 番号を自前で組み立てない）。`pagelen` 最大は概ね 100。
- フィルタ/ソート: `q=<BBQL>`（例 `state="OPEN" AND author.uuid="{...}"`; UUID は `{}` 付き）, `sort=-updated_on`（単一フィールドのみ）, `fields=` で部分応答（一覧系では必須で付ける）。
- エラー形式: `{"type":"error","error":{"message":"...","detail":"...","data":{...}}}`
- レート制限: 認証済みで概ね 1,000/h（repo データ）〜、匿名 60/h。超過時は **429**、`Retry-After` ヘッダあり得る → 指数バックオフ。
- 識別子: workspace は slug か `{uuid}`、repo は slug か `{uuid}`。
- **廃止済み（実装しない）**
  - Issue Tracker / Wiki: 2026-08-20 に UI・API とも撤去 → `bb issue` は提供しない。
  - **CHANGE-2770（cross-workspace API 廃止、2026-02〜04 削除）**: `GET /user/permissions/repositories`、`GET /repositories/{ws}?role=member`、`GET /repositories`（全公開リポ一覧）は使えない。「自分のリポジトリ一覧」は `GET /user/workspaces` → 各 `GET /repositories/{ws}` の 2 段で代替する。`--role` は `contributor|admin|owner` のみ。
- **SSH ホスト移行**: `bitbucket.org` → **`ssh.bitbucket.org`**（2026-11-12 以降 `bitbucket.org` への SSH は拒否）。remote 解析は両ホストを受け付け、SSH URL の生成は `ssh.bitbucket.org` を使う。

### 0.3 主要エンドポイント（実装対象）
- User/Workspace: `GET /user`, `GET /user/workspaces`, `GET /workspaces/{ws}`, `GET /workspaces/{ws}/members`, `GET /workspaces/{ws}/projects`, `POST /workspaces/{ws}/projects`, `GET /workspaces/{ws}/projects/{key}`
- Repositories: `GET /repositories/{ws}?role=contributor|admin|owner&q=&sort=&fields=`, `GET|POST|PUT|DELETE /repositories/{ws}/{slug}`（POST body: `scm:"git"`, `is_private`, `project.key`, `description`, `fork_policy`）, `POST /repositories/{ws}/{slug}/forks`, `GET .../forks`, `GET .../src/{commit}/{path}`, `GET .../commits`, `GET|POST|DELETE .../refs/branches[/{name}]`, `GET|POST|DELETE .../refs/tags[/{name}]`
- Pull requests: `GET /repositories/{ws}/{slug}/pullrequests?state=OPEN|MERGED|DECLINED|SUPERSEDED（複数指定可）&q=&sort=&fields=`, `POST`（必須 `title`, `source.branch.name`; 任意 `destination.branch.name`, `description`, `reviewers[{uuid}]`, `close_source_branch`, `draft`）, `GET|PUT .../{id}`, `POST|DELETE .../{id}/approve`, `POST|DELETE .../{id}/request-changes`, `POST .../{id}/decline`, `POST .../{id}/merge`（body: `merge_strategy` = `merge_commit|squash|fast_forward|squash_fast_forward|rebase_fast_forward|rebase_merge`, `close_source_branch`, `message`; `?async=true` で 202 + `Location` の task-status をポーリング。同期完了時は 200 で PR を返す）, `GET|POST .../{id}/comments`（body `content.raw`、inline は `inline.path/to`）, `GET .../{id}/diff|diffstat|commits|activity|statuses|conflicts`, `GET /workspaces/{ws}/pullrequests/{user}`
- Pipelines: `GET /repositories/{ws}/{slug}/pipelines?sort=-created_on`, `GET .../pipelines/{uuid}`, `POST .../pipelines`（body `target: {type:"pipeline_ref_target", ref_type:"branch", ref_name:"main", selector?:{type:"custom",pattern:"..."}}`, `variables[]`）, `POST .../pipelines/{uuid}/stopPipeline`, `GET .../steps`, `GET .../steps/{step}/log`（Range 対応）
- Default reviewers: `GET .../effective-default-reviewers`
- 汎用: `bb api <path>` で残りのパスをカバー

## 1. 設計方針

- 言語/ツールチェーン: **Go 1.22+**（現環境は Go 未導入 → `brew install go` がステップ0）。モジュール名 `github.com/ueno/bb`（暫定）。
- CLI フレームワーク: `spf13/cobra` + `spf13/pflag`。gh と同じ `bb <noun> <verb>` 体系。
- API クライアントは**自前実装**（`net/http` + `encoding/json`）。理由: 認証方式の細かい制御、`next` 追従型ページング、429 リトライ、`fields` 指定を薄い層で扱いたい。サードパーティ SDK（ktrysmt/go-bitbucket）は App Password 前提・削除済み API 依存があり採用しない。
- 型定義 `internal/bitbucket`（api と対等の階層）: **使うフィールドだけ定義する寛容な struct**（未知フィールドは無視）で API 変更に強くする。`--json` の許可フィールドは型ごとの「表示名 → JSON path」マップとして同パッケージで保守する（gh の `exportable` 方式）。
- 出力: TTY なら表形式+色、非 TTY ならタブ区切りプレーン。全コマンドに `--json [fields]`, `--jq <expr>`（`itchyny/gojq`）, `--template`（Go template。`timeago`/`color`/`tablerow`/`truncate` 等は gh (MIT) から移植）を実装。
- 対話 UI は `charmbracelet/huh` に統一し、色付けも同系の `lipgloss` を使う（`fatih/color` は採用しない）。
- リポジトリ解決: `cmdutil.Factory.BaseRepo()` が `-R/--repo <workspace>/<slug>` → `BB_REPO` → git remote の順に解決。`internal/gitctx` は remote URL の純粋パーサ（read-only）に限定し、`https://bitbucket.org/{ws}/{slug}(.git)`, `git@bitbucket.org:{ws}/{slug}`, `git@ssh.bitbucket.org:{ws}/{slug}`, `ssh://git@(ssh.)bitbucket.org/{ws}/{slug}` を受理。git への書き込み（fetch/checkout/remote add/credential 設定）は `internal/git`（`exec.Command` の引数配列渡し、シェル不使用、位置引数の前に `--`）に分離。
- 設定: `$XDG_CONFIG_HOME/bb/config.yml`（既定 workspace, editor, pager, git_protocol, merge_strategy 等）、`hosts.yml`（認証情報）。ディレクトリ 0700、ファイル 0600、書き込みは temp（`O_EXCL`）+ rename で原子的に。Windows では権限ビットが無意味なため README で keyring を推奨。
  - `config.CredentialStore` インターフェース: `Get(host) (Credential, error)` / `Set(host, Credential)` / `Delete(host)`。`Credential{Method: api_token|bearer, Email, Token, ExpiresAt?}`。file 実装を v0.1、keyring 実装（`zalando/go-keyring`）を後続。
  - `api.Authenticator` インターフェース（`Apply(*http.Request)`）を Credential から生成し、Client は認証方式を知らない（SRP）。
- 環境変数: `BB_TOKEN`（API Token / Access Token）, `BB_EMAIL`, `BB_AUTH_METHOD=api_token|bearer`, `BB_REPO`, `BB_WORKSPACE`, `NO_COLOR`, `BB_PAGER`, `BB_DEBUG`。**トークンをフラグで渡す手段は提供しない**（ps 露出防止）。
- エラー: Bitbucket の `error.message`/`detail` を人間向けに表示、終了コードは gh 準拠（1: 一般, 2: キャンセル/Ctrl-C, 4: 認証必要）。401 → `bb auth login` 案内（期限切れの可能性も併記）、403 → README のスコープ表へのリンク（本文からスコープを推測しない）。
- セキュリティ:
  - `--verbose`/`BB_DEBUG` のログでは `Authorization` ヘッダ全体、`bb api -H` で渡されたヘッダ、リダイレクト先 URL、エラー応答 Raw 中のトークン様文字列をマスク。`bb auth token` は TTY 時に警告を出す。
  - BBQL は `api.BBQLQuote(s)`（`\` と `"` をエスケープ）経由でのみ値を埋め込み、ユーザー指定 `--search` と内部生成分を混ぜるときは `( … )` で括る。
  - `bb browse` の ws/slug は `^[A-Za-z0-9_.{}-]+$` で検証し `url.PathEscape` して `https://bitbucket.org/` に連結。任意スキームは渡さない。
  - `Retry-After` は上限 60s でクリップ。リトライは冪等メソッド（GET/HEAD/PUT/DELETE）と 429 のみ。POST の 5xx はリトライしない（merge/pipeline run/pr create の二重実行防止）。
- ポーリング抽象 `internal/api/poll.go`: `Poll(ctx, fn, PollOptions{Initial: 2s, Max: 30s, Factor: 1.5, Timeout})`。`pr merge --async` の task-status と `pipeline watch` が共有。429 はリトライ層に任せ、ポーリング側は間隔を伸ばすだけ。Ctrl-C は exit 2。
- ページング `Paginate(ctx, path, query, fields, limit, fn)`: `--limit` 既定 30、`pagelen` = min(limit, 50)。一覧系コマンドは必ず `fields=values.<必要項目>,next` を渡す。
- テスト: `httptest.Server` によるユニットテスト（**各ステップの必須完了条件**）+ `testdata/` のゴールデン（非 TTY 出力と JSON のみ、`-update` フラグで更新）。実 API 統合テストは `BB_INTEGRATION=1` で nightly CI 必須実行（手動 smoke はチェックリストとして任意）。

## 2. コマンド体系（gh 対応表）

| gh | bb | 備考 |
|---|---|---|
| `gh auth login/logout/status/token/refresh/setup-git` | `bb auth login/logout/status/token/setup-git/git-credential` | login: 対話（メール+API Token+任意で有効期限）/ `--with-token` (stdin) / `--bearer`（Access Token）。`refresh` は OAuth 実装時 |
| `gh repo list/view/create/clone/fork/delete/edit` | `bb repo list/view/create/clone/fork/delete/edit` | `list` は `--workspace`（省略時は全ワークスペース走査）, `--role contributor|admin|owner`, `--limit` |
| `gh pr list/view/create/checkout/merge/close/reopen/review/comment/diff/status/edit/ready/checks` | `bb pr list/view/create/checkout/merge/decline(close alias)/approve/unapprove/review/comment/diff/status/edit/ready/checks` | `reopen` は API 非対応 → コマンドは存在させ「Bitbucket では declined PR を再オープンできません。新規 PR を作成してください」と案内して exit 1 |
| `gh run list/view/watch/rerun/cancel` | `bb pipeline list/view/watch/run/stop/log`（`bb run` を alias） | `run` = POST /pipelines |
| `gh browse` | `bb browse` | `bitbucket.org/{ws}/{slug}[/pull-requests/{id}]` |
| `gh api` | `bb api <path> [-X] [-f/-F] [--paginate] [--input] [-H] [-i] [--silent]` | `--paginate` は `next` 追従、`values` を連結 |
| `gh issue` | **非対応** | Issue tracker 撤去のため |
| `gh org` | `bb workspace list/view/members` | |
| — | `bb project list/view/create` | Bitbucket 固有 |
| — | `bb branch list/create/delete` | refs API |
| `gh config get/set/list` | `bb config get/set/list` | `merge_strategy` の既定を上書き可 |
| `gh completion` / `gh version` | 同名 | |

`bb pr merge` のフラグ: `--merge`(=merge_commit, 既定) / `--squash` / `--rebase`(=rebase_merge) を gh 同名で提供し、Bitbucket 固有の 6 種は `--strategy <name>` に逃がす。

## 3. ディレクトリ構成

```
bb/
├── cmd/bb/main.go
├── internal/
│   ├── api/            # client.go, auth.go(Authenticator), pagination.go, retry.go, poll.go, errors.go, bbql.go
│   ├── bitbucket/      # 型定義 + --json フィールドマップ (workspace, repository, pullrequest, pipeline, user, ref)
│   ├── config/         # config.yml / hosts.yml / CredentialStore(file, keyring)
│   ├── gitctx/         # remote URL パーサ（read-only）
│   ├── git/            # git exec ラッパ（fetch/checkout/remote/credential）
│   ├── iostreams/      # TTY 判定・色・pager
│   ├── output/         # table, --json/--jq/--template
│   ├── cmdutil/        # Factory{IOStreams, Config, HTTPClient, BaseRepo, Git}, 共通フラグ, exit codes
│   └── browser/
├── pkg/cmd/            # root/ auth/ repo/ pr/ pipeline/ workspace/ project/ branch/ api/ browse/ config/ version/
├── testdata/
├── docs/
├── .goreleaser.yml, .github/workflows/{ci,nightly,release}.yml
├── Makefile, go.mod, README.md
```

## 4. ステップバイステップ実装計画

各ステップの完了条件は「自動テスト（必須）」+「手動確認（任意チェックリスト）」。見積もりは 1.5 倍の余裕込み。
**マイルストーン**: v0.1 = Step 0〜8（auth / api / repo 基本 / pr 主要 / CI・リリース）。v0.2 = Step 9〜11。v0.3 = Step 12。

### Step 0: 環境準備（0.5日）
- `brew install go`（1.22+）, `golangci-lint`, `goreleaser`。`git init`、`.gitignore`, `go mod init`。
- 完了条件: `go version` が通る、空の `main.go` がビルドできる。

### Step 1: スケルトン & 共通基盤 & CI（1日）
- cobra ルート、`bb version`, `bb completion`、グローバルフラグ定義。
- `internal/iostreams`, `internal/cmdutil`（Factory: IOStreams / Config / HTTPClient / BaseRepo / Git を遅延生成）。
- GitHub Actions `ci.yml`（lint/test/build、macOS/Linux。Windows は best effort で build のみ）を**この時点で**用意。
- 完了条件: `bb --help` が gh 風に表示、`go test ./...`・golangci-lint が CI で緑。

### Step 2: 設定と認証ストア（1日）
- `internal/config`: config.yml / hosts.yml（`gopkg.in/yaml.v3`）、0700/0600、原子的書き込み、`CredentialStore`（file）、`Credential{Method, Email, Token, ExpiresAt}`。環境変数優先順位 `BB_TOKEN`(+`BB_EMAIL`/`BB_AUTH_METHOD`) > hosts.yml。
- 完了条件: 保存/読込/権限/原子性/環境変数優先のユニットテスト。

### Step 3: API クライアント（1.5日）
- `api.Client{Do, Paginate}`、`Authenticator`（Basic email:token / Bearer）、`User-Agent: bb/<ver>`、`Accept: application/json`。
- ページング（`next` 追従、`fields`、`limit`/`pagelen` 丸め）、リトライ（429: Retry-After 秒/HTTP-date 尊重・60s クリップ・指数バックオフ最大 3 回、冪等メソッドの 5xx は 1 回、POST は無し）、`HTTPError{Status, Message, Detail, Raw}`、`BBQLQuote`、`Poll`。
- `--verbose`/`BB_DEBUG=1` の HTTP ログ（マスク仕様は §1）。
- 完了条件: httptest でページング・fields・429/Retry-After・POST 非リトライ・エラー整形・BBQL エスケープ・Poll（タイムアウト/キャンセル）をテスト。

### Step 4: 出力レイヤ（1日）
- `internal/output`: TablePrinter（TTY: 整列+色, 非 TTY: TSV）、`--json fields` の許可フィールド検証（`internal/bitbucket` のマップと連動）、`--jq`（gojq）、`--template`（gh 互換関数の移植）。
- 完了条件: 非 TTY ゴールデンテスト（`-update` 付き）。

### Step 5: `bb auth`（1.5日）
- `login`: 対話（メール → API Token を非エコー入力 → 任意で有効期限）→ `GET /user` で検証 → 保存。`--with-token`（stdin: `email:token` または token 単体 + `--email`）、`--bearer`。作成 URL と推奨スコープ表を表示。
- `status`: `GET /user`, `GET /user/workspaces`、トークン種別、保存済み期限（7 日前から警告）。`logout`, `token`（TTY 警告付き）。
- `setup-git`: `git config --global credential.https://bitbucket.org.helper '!bb auth git-credential'` のようにホスト限定で登録。`git-credential`: credential helper プロトコル（`get`/`store`/`erase`、stdin key=value）を実装し、`protocol=https` かつ `host=bitbucket.org` の場合のみ応答。ユーザー名は Method に応じて `x-bitbucket-api-token-auth`（API Token）/ `x-token-auth`（Access Token）。
- 完了条件: httptest で login/status/helper プロトコル（ホスト不一致で無応答）をテスト。手動: 実アカウントで login→status→`git ls-remote https://…`。

### Step 6: リポジトリ解決 & `bb api` & `bb browse`（1日）
- `gitctx` パーサ（https / git@ / ssh:// × bitbucket.org / ssh.bitbucket.org）、`Factory.BaseRepo()`。
- `bb api`: gh 互換フラグ、`--paginate`、`--jq`。`bb browse [path|pr#] [--branch]`（入力検証は §1）。
- 完了条件: パーサのテーブルテスト、`bb api` の httptest テスト（paginate 連結、-i、-f/-F、--input）。

### Step 7: `bb repo`（1.5日）
- `list [--workspace] [--role contributor|admin|owner] [--limit] [--language] [--visibility] [--sort]`（workspace 省略時は `/user/workspaces` を走査）, `view [--web] [--branch]`（README は `src/{mainbranch}/README.md`）, `create <name> [--workspace] [--project KEY] [--private/--public] [--description] [--clone]`（project 未指定時は最古 project に自動割当される旨を表示）, `clone <ws/slug|slug>`（`git_protocol=ssh` は `git@ssh.bitbucket.org:` を生成）, `fork [--workspace] [--name] [--clone]`, `delete [--yes]`, `edit`。
- 完了条件: 各サブコマンドの httptest テスト + 非 TTY ゴールデン。

### Step 8: `bb pr` + v0.1 リリース（3.5日）
- `list [--state] [--author @me] [--search BBQL] [--limit]`（`@me` は `/user` の uuid → `author.uuid="{…}"`）、`view [id|branch] [--web] [--comments]`（省略時は現在ブランチで `q=source.branch.name=<quoted>`）、`create`（`-t/-b/--body-file/--base/--head/--reviewer/--draft/--close-source-branch/--fill/--web`、対話は huh、reviewer はニックネーム→uuid 解決 + effective-default-reviewers 任意付与）、`checkout <id>`（fork 元は `internal/git` で remote 追加）、`merge [id] [--merge|--squash|--rebase|--strategy] [--delete-branch] [--message]`（200 は即完了、202 は `Location` を `Poll` で追跡。タイムアウト既定 5 分）、`decline`(`close`)、`reopen`（案内のみ）、`approve/unapprove`、`review`、`comment`、`diff [--name-only]`、`status`、`checks`、`edit`、`ready`。
- `release.yml` + goreleaser（brew tap、`-ldflags -X version`）、README（API Token 作成手順、スコープ表、gh との差分）。**v0.1 タグ**。
- 完了条件: 全サブコマンドの httptest テスト（merge の 200/202/タイムアウト/Ctrl-C を含む）+ ゴールデン。手動: create→approve→merge。

### Step 9: `bb pipeline`, `bb branch`（1.5日）
- `pipeline list/view/run/stop/watch/log`（`watch` は `Poll` 共有、終了コードを result に連動、`log` は Range 追記）、`branch list/create/delete`。
- 完了条件: httptest テスト（watch の状態遷移、log の Range）。

### Step 10: `bb workspace`, `bb project`, `bb config`（1日）
- 完了条件: httptest テスト。

### Step 11: nightly 統合テスト・ドキュメント（1日）
- `nightly.yml`（`BB_INTEGRATION=1`、専用テスト用ワークスペースとシークレット）を**必須**とし、削除済み API や仕様変更を検知。`cobra/doc` で `docs/` 生成。**v0.2 タグ**。

### Step 12（v0.3）: OAuth 2.0 ログイン、keyring
- `bb auth login --web`: **ユーザー自身が作成した OAuth Consumer**（key/secret を config か環境変数で指定、Callback `http://127.0.0.1:<port>/callback`）で Authorization Code フロー。`state`（crypto/rand）必須、`127.0.0.1` に bind、ワンショット + タイムアウト、code 受領後即クローズ。refresh_token は access token と同等に秘匿、期限前に自動 refresh。
- `CredentialStore` の keyring 実装、`BB_CREDENTIAL_STORE=file|keyring`。

## 5. リスクと対策
- API Token を Basic のユーザー名にメールで使う点の混乱（git は username / 固定値） → login の説明文と credential helper で吸収。
- **API の継続的な廃止**（App Password, Issues/Wiki, cross-workspace API, SSH ホスト）→ 寛容な struct、nightly 統合テスト必須、Atlassian changelog の定期確認を README の保守手順に記載。
- レート制限 → 自動リトライ、`--limit` 既定 30、`fields` 必須、ポーリング上限。
- gh との機能差（issue なし、reopen 不可、device flow なし）→ README とコマンド内エラーメッセージの両方で案内。
- Windows → v0.1 は best effort（build のみ）、hosts.yml 権限は keyring 推奨で補う。

## 6. 依存ライブラリ（最小限）
`spf13/cobra`, `spf13/pflag`, `gopkg.in/yaml.v3`, `itchyny/gojq`, `mattn/go-isatty`, `charmbracelet/huh` + `charmbracelet/lipgloss`, `golang.org/x/term`, `pkg/browser`。後続: `zalando/go-keyring`。テスト: 標準 `testing` + `google/go-cmp`。

## 7. 参考
- API Token: https://support.atlassian.com/bitbucket-cloud/docs/using-api-tokens/ , https://support.atlassian.com/bitbucket-cloud/docs/api-token-permissions/
- App Password 廃止: https://www.atlassian.com/blog/bitbucket/bitbucket-cloud-transitions-to-api-tokens-enhancing-security-with-app-password-deprecation , https://community.atlassian.com/forums/Bitbucket-articles/Deprecation-notice-Bitbucket-Cloud-app-password-brownout/ba-p/3237429
- CHANGE-2770（cross-workspace API 廃止）: https://community.atlassian.com/forums/Bitbucket-questions/replacements-for-deprecated-user-scoped-permission-endpoints/qaq-p/3166685
- SSH ホスト移行: https://community.atlassian.com/forums/Bitbucket-articles/Upcoming-change-to-Bitbucket-Cloud-SSH-access-move-from/ba-p/3234032
- OAuth 2.0: https://developer.atlassian.com/cloud/bitbucket/oauth-2/ , https://support.atlassian.com/bitbucket-cloud/docs/use-oauth-on-bitbucket-cloud/
- REST intro / OpenAPI: https://developer.atlassian.com/cloud/bitbucket/rest/intro/ , https://developer.atlassian.com/cloud/bitbucket/swagger.v3.json
- Rate limits: https://support.atlassian.com/bitbucket-cloud/docs/api-request-limits/
- Issues/Wiki 廃止: https://community.atlassian.com/forums/Bitbucket-articles/Announcing-sunset-of-Bitbucket-Issues-and-Wikis/ba-p/3193882
- 類似 OSS（参考）: https://github.com/avivsinai/bitbucket-cli (MIT)

## 8. 実装時の注意事項（MAGI 第2回審議の WARNINGS — 全 APPROVE 時の残件）
- サーバー由来 URL（ページング `next`、merge 202 の `Location`、`bb api --paginate`）は host が `https://api.bitbucket.org` のときのみ `Authenticator.Apply` を適用。異なる host は拒否。
- `Poll` は HTTPError(429) なら間隔を伸ばして継続、それ以外のエラーは打ち切り。
- `git-credential` の `store`/`erase` は no-op（helper 経由で hosts.yml を書き換えない）。`host=bitbucket.org:443` 形式も受理。`setup-git` は `helper=` 空エントリで既存 helper をリセットしてから登録（gh と同様）。
- `BB_TOKEN` のみで `BB_AUTH_METHOD` 未指定の場合: `BB_EMAIL` があれば api_token、無ければ bearer とみなす（git-credential のユーザー名選択と整合）。
- `--verbose` はレスポンス本文を出さない。本文出力は `BB_DEBUG=2` の明示オプトインのみ。
- `Retry-After` の HTTP-date が過去なら 0 秒。`BB_NO_RETRY=1` で CI 用途の即時失敗を提供（任意）。
- OAuth consumer secret は config.yml ではなく hosts.yml/keyring に保存。
- `repo list` の workspace 省略時は `config.workspace` があればそれを既定、無ければ走査（`--limit` は横断合計件数で打ち切り、`fields` 最小化）。
- `pr view <arg>`: 数字のみ → id、それ以外 → branch 名。
- `Credential.ExpiresAt` は自己申告値。未設定なら `auth status` に「期限未登録」と表示。
- `internal/bitbucket` の `--json` フィールドマップは「キーが struct に実在するか」を検証するテーブルテストを持つ。
- Step 8 はコミット単位を 8a(list/view/create/checkout) / 8b(merge/decline/approve/review/comment) / 8c(残り+リリース) に分割。
- nightly 失敗時は main をブロックせず issue 起票（運用）。

## 9. 進捗（2026-08-25）
- Step 0〜11 実装済み（v0.1 + v0.2 相当）。全コマンドに httptest ベースのユニットテストあり、golangci-lint 0 件。
- Step 12 実装済み: `bb auth login --web`（Authorization Code、state 検証、127.0.0.1 固定ポート、ワンショット）、`bb auth refresh`、Factory での自動 refresh、`credential_store=keyring`（zalando/go-keyring）。
- 実 API に対する smoke（`bb auth login` → `pr create` → `merge`）は API Token を用意した上で手動確認が必要。

## 10. MAGI コードレビュー対応（2026-08-25）
実装コード全体の MAGI 審議（MELCHIOR=APPROVE / BALTHASAR=CONDITIONAL / CASPER=CONDITIONAL）で挙がった 9 分類すべてに対応済み:
`pr checkout` の修正と git Runner インターフェース化（回帰テスト追加）、OAuth refresh の `config.ResolveFreshCredential` への集約（API クライアント・git credential helper・`auth token` で共有）、OAuth callback の偽装要求を無視して待機継続、`pipeline watch --exit-status` 実装、`pr diff --color {always|never|auto}`、`signal.NotifyContext` による Ctrl-C=exit 2、`pr checks --watch` の Poll 化、remote/サーバー由来 URL・名前の検証、`pr ready/edit` の title 同梱、握りつぶしていたエラーの表面化、`--commit` の target 形式、`OpenBrowser`/`OptionalArg`/`MainBranch`/`gitctx.CloneURL` への集約、一覧の `fields` 指定、冗長な再取得の削減、gh と衝突する短縮フラグの整理、`bb api` の `-H --paginate`/`?` 対応、`credential_store` 切替時の資格情報移行、テスト環境変数の遮断。

### MAGI 再審議（2026-08-25、3 回で完了）
- 第 2 回: MELCHIOR=APPROVE / BALTHASAR=CONDITIONAL（fork PR の `decline --delete-branch` 誤削除）/ CASPER=APPROVE → `b2b7836` で修正（fork 拒否、`pr merge -b/--body`、review 順序、refreshingAuth、checks 再描画/--timeout、log 最終 fetch、秘密マスク、ブラウザ allowlist）
- 第 3 回: **三体 APPROVE**（確信度 9/9/9）。残警告は UX/保守性の提案のみ（URL リテラル集約、fetchPR の fields、refresh 失敗警告の抑制、fork decline の文言）。

### 改善サイクル（2026-08-25、MAGI 残警告の全件対応）
グループ 1〜5（UX 11 件、保守性 8 件、テスト 6 件、ドキュメント 2 件）をすべて実施。Data Center は非対応と README に明記（B5）。実 API での検証（C1・A5 の `q=` フィルタ可否）は統合テストとして追加済みだが、API Token とテスト用リポジトリが必要なため未実行。
