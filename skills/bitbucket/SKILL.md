---
name: bitbucket
description: "Operate Bitbucket Cloud from Claude Code through the `bb` CLI (GitHub CLI-like): repositories, pull requests, pipelines, branches, workspaces, projects, and raw API calls. Use when the user mentions Bitbucket, a bitbucket.org URL, a Bitbucket pull request / pipeline, or asks to do with Bitbucket what `gh` does for GitHub."
user_invocable: true
trigger: "/bitbucket"
---

# Bitbucket Cloud via `bb`

`bb` は GitHub CLI（`gh`）と同じ操作感で Bitbucket Cloud を扱う CLI です。
このスキルは、Claude Code が `bb` を安全かつ効率的に使うための手順書です。
リポジトリ: https://github.com/uehatsu/bb（日本語 README: `README_JA.md`）

## 0. 起動時の確認（毎回最初に行う）

```sh
bb version            # 未導入なら → brew install uehatsu/tap/bb（または go install github.com/uehatsu/bb/cmd/bb@latest）
bb auth status        # exit 0 以外なら未ログイン → ユーザーに `bb auth login` を案内（下記 §5）
```

- `bb` が無い / 未ログインのときは、**代わりに curl や独自実装で API を叩かない**。導入・ログインをユーザーに依頼する。
- 対象リポジトリは、カレントディレクトリの git remote（`upstream` > `origin`）から自動判定される。Bitbucket リポジトリの外で実行するときは **必ず `-R WORKSPACE/REPO`** を付ける。

## 1. 基本ルール

1. **機械可読出力を使う**: 情報取得は `--json <fields>` と `--jq` を使い、表出力をパースしない。
   利用可能なフィールドは `bb <cmd> --json x`（存在しない名前）を実行すると一覧が出る。
2. **非対話で動かす**: Claude Code の Bash は TTY ではない。プロンプトが出るコマンドには明示フラグを付ける
   （`pr create --title/--body`、`pr merge --squash|--merge|--rebase`、`repo delete --yes`、`branch delete --yes`、`repo create --private|--public`）。
3. **破壊的操作は事前確認**: `pr merge`、`pr decline`、`branch delete`、`repo delete`、`repo edit --visibility`、`pipeline stop` は、実行前に対象（リポジトリ・番号・ブランチ名）をユーザーに提示して確認を取る。`--yes` はユーザーが明示的に承認したときのみ付ける。
4. **秘密情報を出力しない**: `bb auth token` の出力、`BB_TOKEN`、`hosts.yml` の内容を会話に貼らない。`BB_DEBUG=2` はレスポンス本文（個人情報を含みうる）を出すので使わない。
5. **PR は URL で指定してよい**: `bb pr view https://bitbucket.org/ws/repo/pull-requests/42` のように URL を渡すと、URL 中のリポジトリが対象になる（カレントリポジトリは無視される）。番号だけを渡すときはカレント / `-R` のリポジトリが対象。
6. **終了コード**: 0 成功 / 1 エラー / 2 キャンセル / 4 認証が必要 / 8 進行中（`pr checks`）。4 のときはユーザーにログインを依頼する。

## 2. よく使うコマンド

### リポジトリ
```sh
bb repo list <workspace> --json fullName,description,updatedAt   # -L で件数、--role contributor|admin|owner
bb repo view [ws/repo] --json fullName,mainBranch,url,isPrivate
bb repo clone ws/repo [dir] [-- --depth 1]
bb repo create ws/name --private --project KEY -d "説明"
bb repo fork ws/repo --workspace mine --clone
bb repo edit --default-branch main --description "…"
bb repo delete ws/repo --yes          # 要ユーザー承認
```

### プルリクエスト
```sh
bb pr list --state open|merged|declined|all --author @me --base main -L 20 --json id,title,state,headRefName,author,url
bb pr view 42 --json id,title,body,state,headRefName,baseRefName,reviewers,participants,url
bb pr view 42 --comments                         # レビューコメントを含めて表示
bb pr diff 42 [--stat | --name-only]             # --color never で色なし
bb pr checks 42                                   # ビルド状態。exit 8 = 実行中、1 = 失敗あり
bb pr create --title "…" --body "…" [--base develop] [--head feat/x] [--reviewer alice,bob] [--draft] [--close-source-branch]
bb pr create --fill                               # コミットからタイトル/本文を生成
bb pr checkout 42                                 # ブランチを取得してチェックアウト（フォークも可）
bb pr review 42 --approve | --request-changes -b "理由" | --comment -b "…"
bb pr comment 42 -b "…" [--path src/x.go --line 10]
bb pr edit 42 --title "…" --body "…" --add-reviewer carol --remove-reviewer bob --base main
bb pr ready 42 [--undo]                           # draft ⇄ ready
bb pr merge 42 --squash|--merge|--rebase [--delete-branch] [-b "コミットメッセージ"] --yes   # 要ユーザー承認
bb pr decline 42 [--delete-branch]                # 要ユーザー承認（close は alias。reopen は不可）
bb pr status                                      # 自分の PR / レビュー待ち
```
- `--body` に複数行を渡すときは `--body-file -` で stdin から渡す（heredoc）。
- merge 戦略は Bitbucket の 6 種類を `--strategy merge_commit|squash|fast_forward|squash_fast_forward|rebase_fast_forward|rebase_merge` で指定可。

### パイプライン（Bitbucket Pipelines）
```sh
bb pipeline list -L 10 --json buildNumber,status,result,refName,createdAt,url
bb pipeline view 128 [--json …]                   # ステップ一覧
bb pipeline run --branch main [--custom deploy --var ENV=prod] [--watch]
bb pipeline watch 128 [--exit-status=false]       # 完了まで待機。失敗時 exit 1
bb pipeline log 128 [--step 2] [--follow]
bb pipeline stop 128                              # 要ユーザー承認
```

### ブランチ / ワークスペース / プロジェクト
```sh
bb branch list -L 20 --json name,target
bb branch create feat/x --from main
bb branch delete feat/x --yes                     # 要ユーザー承認
bb workspace list --json slug,name
bb workspace members <ws>
bb project list -w <ws> --json key,name
bb project create KEY --name "…" -w <ws> --private
```

### 生 API（上記で足りないとき）
```sh
bb api /user
bb api repositories/{workspace}/{repo_slug}/commits --paginate --jq '.[].hash'      # {workspace}/{repo_slug} は自動置換
bb api -X POST repositories/{workspace}/{repo_slug}/refs/tags -f name=v1.0 -F 'target[hash]=abc123'
bb api -X PUT repositories/ws/repo -f description="…"
```
- `--paginate` は `next` を辿って `values` を 1 つの JSON 配列に連結する。
- `-f` は文字列、`-F` は型付き（`true`/`false`/`null`/整数、`@file`）。GET でフィールドを付けると query になる。

### ブラウザで開く
```sh
bb browse [42 | path/to/file[:line]] [--branch x] [--pull-requests | --pipelines] [-n]   # -n で URL のみ出力
```

## 3. 典型的な作業フロー

**PR を作る**
1. `git status` / `git log origin/<base>..HEAD` で差分を確認
2. push 済みか確認（未 push なら `git push -u origin HEAD`）
3. `bb pr create --title "…" --body-file - --base <base> [--reviewer …] <<'EOF' … EOF`
4. 出力された URL をユーザーに提示

**PR をレビューする**
1. `bb pr view <n> --json title,body,headRefName,baseRefName,author,participants`
2. `bb pr diff <n>`（大きければ `--stat` → 必要なファイルだけ `bb api …/diff` や `bb pr checkout`）
3. `bb pr checks <n>`
4. 所見は `bb pr comment` / `bb pr review --request-changes -b` で投稿（投稿前にユーザーへ内容確認）

**CI の失敗を調べる**
1. `bb pr checks <n>` または `bb pipeline list --branch <br> -L 3 --json buildNumber,result,url`
2. `bb pipeline view <build#>` でステップと結果を確認
3. `bb pipeline log <build#> --step <k>` でログを読む（長い場合は `| tail -200`）

**マージする**
1. `bb pr checks <n>` が成功、承認状況を `--json participants` で確認
2. 戦略・ブランチ削除の有無をユーザーに確認
3. `bb pr merge <n> --squash --delete-branch --yes`

## 4. Bitbucket 固有の注意（gh との違い）

- **Issue は無い**（Bitbucket Issues は 2026-08 に廃止）。課題管理は Jira 等を使う。`bb issue` は存在しない。
- **却下した PR は再オープンできない**。同じブランチから作り直す。
- `pr decline --delete-branch` はフォーク側のブランチを消さない（警告のみ）。
- リポジトリは `workspace/slug` で指定する（GitHub の `owner/repo` 相当）。`bb config set workspace <ws>` を設定すると `slug` だけでも可。
- SSH の接続先は `ssh.bitbucket.org`（`bitbucket.org` は 2026-11 以降 SSH 不可）。`bb` は両方を認識する。
- 認証は **App Password ではなく API Token**（2026-07 廃止済み）。

## 5. ログインをユーザーに案内するとき

```
1. https://id.atlassian.com/manage-profile/security/api-tokens を開く
2. 「Create API token with scopes」→ アプリ: Bitbucket → 必要スコープ（read/write の repository, pullrequest, pipeline、read の user, workspace, project）→ 有効期限を設定
3. ターミナルで `bb auth login` を実行し、Atlassian のメールアドレスとトークンを入力（`--expires-in 1y` で期限警告が有効）
4. `bb auth setup-git` で git push/fetch にも同じトークンを使用
```
CI 等の非対話環境: `BB_EMAIL` と `BB_TOKEN` を環境変数で渡す。

## 6. トラブルシューティング

| 症状 | 対処 |
|---|---|
| exit 4 / `not logged in` | §5 のログインを案内 |
| `HTTP 401` | トークン期限切れの可能性。`bb auth status` で期限を確認し再作成 |
| `HTTP 403` | スコープ不足。必要スコープを付けたトークンを作り直す（既存トークンのスコープ変更は不可） |
| `no Bitbucket remote found` | `-R ws/repo` を付ける、または Bitbucket リポジトリ内で実行 |
| `HTTP 429` | 自動リトライ済み。連続実行を減らす（`-L` で件数を絞る、`--json` で fields を絞る） |
| `--yes required` | 非対話環境の破壊的操作。ユーザー承認後に `--yes` を付ける |
| 出力が長い | `--json` + `--jq` で必要な値だけ取る。`-L` で件数制限 |
