---
name: bitbucket
description: "Operate Bitbucket Cloud from GitHub Copilot through the `bb` CLI (GitHub CLI-like): repositories, pull requests, pipelines, branches, workspaces, projects, and raw API calls. Use when the user mentions Bitbucket, a bitbucket.org URL, a Bitbucket pull request or pipeline, or asks to do with Bitbucket what `gh` does for GitHub."
license: MIT
---

# Bitbucket Cloud via `bb`

`bb` は GitHub CLI (`gh`) と同じ操作感で Bitbucket Cloud を扱う CLI です。
このスキルは、GitHub Copilot（CLI / エージェント）が `bb` を安全かつ効率的に使うための手順書です。
リポジトリ: https://github.com/uehatsu/bb

## 0. 起動時の確認

Bitbucket 操作を始める前に、必要な範囲で状態を確認する。

```sh
bb version            # 未導入なら: brew install uehatsu/tap/bb（または go install github.com/uehatsu/bb/cmd/bb@latest）
bb auth status        # exit 0 以外なら未ログイン → ユーザーに `bb auth login` を案内（§5）
```

- `bb` が無い、または未ログインの場合は、curl や独自実装で API を叩かず、導入または `bb auth login` をユーザーに案内する。
- 対象リポジトリは、カレントディレクトリの git remote (`upstream` > `origin`) から自動判定される。Bitbucket リポジトリの外で実行するときは `-R WORKSPACE/REPO` を付ける。

## 1. 基本ルール

1. **機械可読出力を使う**: 情報取得は `--json <fields>` と `--jq` を優先し、表出力をパースしない。利用可能なフィールドは `bb <cmd> --json x` のように存在しないフィールドを渡すと確認できる。
2. **非対話で動かす**: Copilot のコマンド実行環境は TTY ではない。プロンプトが出るコマンドには明示フラグを付ける。例: `pr create --title/--body`, `pr merge --squash|--merge|--rebase`, `repo delete --yes`, `branch delete --yes`, `repo create --private|--public`。
3. **破壊的操作は事前確認**: `pr merge`, `pr decline`, `branch delete`, `repo delete`, `repo edit --visibility`, `pipeline stop` は、実行前に対象のリポジトリ、番号、ブランチ名をユーザーに提示して確認を取る。`--yes` はユーザーが明示的に承認したときのみ付ける。
4. **秘密情報を出力しない**: `bb auth token` の出力、`BB_TOKEN`、`hosts.yml` の内容を会話に貼らない。`BB_DEBUG=2` はレスポンス本文を出すため使わない。
5. **PR は URL で指定できる**: `bb pr view https://bitbucket.org/ws/repo/pull-requests/42` のように URL を渡すと、URL 中のリポジトリが対象になる。番号だけを渡すときはカレントまたは `-R` のリポジトリが対象。
6. **終了コードを読む**: 0 成功 / 1 エラー / 2 キャンセル / 4 認証が必要 / 8 進行中 (`pr checks`)。4 のときはユーザーにログインを依頼する。

### 破壊的操作のワークフロー（厳格）

1. 対象の特定情報（リポジトリ、PR 番号、ブランチ名、簡単な説明）を取得してユーザーに提示する。
2. ユーザーの明示的な承認（自然言語での同意）を待つ。承認前に実行しない。
3. 承認が得られたら `--yes` を付けて実行し、結果を報告する。

例: ブランチ削除

```text
提案: 「ブランチ feat/x を workspace/repo から削除します。続行しますか?」
承認後: bb branch delete feat/x --yes
```

## 2. よく使うコマンド

### リポジトリ

```sh
bb repo list <workspace> --json fullName,description,updatedAt   # -L で件数、--role contributor|admin|owner
bb repo view [ws/repo] --json fullName,mainBranch,url,isPrivate
bb repo clone ws/repo [dir] [-- --depth 1]
bb repo create ws/name --private --project KEY -d "説明"
bb repo fork ws/repo --workspace mine --clone
bb repo edit --default-branch main --description "..."
bb repo delete ws/repo --yes          # 要ユーザー承認
```

### プルリクエスト

```sh
bb pr list --state open|merged|declined|all --author @me --base main -L 20 --json id,title,state,headRefName,author,url
bb pr view 42 --json id,title,body,state,headRefName,baseRefName,reviewers,participants,url
bb pr view 42 --comments                         # レビューコメントを含めて表示
bb pr diff 42 [--stat | --name-only] --color never   # ANSI なしで読む
bb pr checks 42                                   # exit 8 = 実行中、1 = 失敗あり
bb pr create --title "..." --body "..." [--base develop] [--head feat/x] [--reviewer alice,bob] [--draft] [--close-source-branch]
bb pr create --fill                               # コミットからタイトル/本文を生成
bb pr checkout 42                                 # ブランチを取得してチェックアウト（フォークも可）
bb pr review 42 --approve | --request-changes -b "理由" | --comment -b "..."
bb pr comment 42 -b "..." [--path src/x.go --line 10]
bb pr edit 42 --title "..." --body "..." --add-reviewer carol --remove-reviewer bob --base main
bb pr ready 42 [--undo]                           # draft <-> ready
bb pr merge 42 --squash|--merge|--rebase [--delete-branch] [-b "コミットメッセージ"] --yes   # 要ユーザー承認
bb pr decline 42 [--delete-branch]                # 要ユーザー承認（close は alias。reopen は不可）
bb pr status                                      # 自分の PR / レビュー待ち
```

- 複数行の本文は `--body-file -` で stdin から渡す。
- merge 戦略は `--strategy merge_commit|squash|fast_forward|squash_fast_forward|rebase_fast_forward|rebase_merge` で指定できる。

### パイプライン

```sh
bb pipeline list -L 10 --json buildNumber,status,result,refName,createdAt,url
bb pipeline view 128 [--json buildNumber,status,result,refName]   # ステップ一覧
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
bb project create KEY --name "..." -w <ws> --private
```

### 生 API

`bb` の専用コマンドで足りない場合だけ使う。

```sh
bb api /user
bb api repositories/{workspace}/{repo_slug}/commits --paginate --jq '.[].hash'
bb api -X POST repositories/{workspace}/{repo_slug}/refs/tags -f name=v1.0 -F 'target[hash]=abc123'
bb api -X PUT repositories/ws/repo -f description="..."
```

- `{workspace}` / `{repo_slug}` は対象リポジトリから自動置換される。
- `--paginate` は `next` を辿って `values` を 1 つの JSON 配列に連結する。
- `-f` は文字列、`-F` は型付き (`true`, `false`, `null`, 整数, `@file`)。GET でフィールドを付けると query になる。

### ブラウザで開く

```sh
bb browse [42 | path/to/file[:line]] [--branch x] [--pull-requests | --pipelines] [-n]
```

`-n` は URL のみを出力する。

## 3. 典型的な作業フロー

**PR を作る**

1. `git status` と `git log origin/<base>..HEAD` で差分を確認する。
2. push 済みか確認し、未 push ならユーザーの意図に沿って `git push -u origin HEAD` する。
3. `bb pr create --title "..." --body-file - --base <base> [--reviewer ...]` を使う。
4. 出力された URL をユーザーに提示する。

**PR をレビューする**

1. `bb pr view <n> --json title,body,headRefName,baseRefName,author,participants` で概要を見る。
2. `bb pr diff <n>` を確認する。大きい場合は `--stat` や `--name-only` で絞る。
3. `bb pr checks <n>` で CI 状態を見る。
4. コメントやレビュー投稿は、投稿内容をユーザーに確認してから `bb pr comment` / `bb pr review` を実行する。

**CI の失敗を調べる**

1. `bb pr checks <n>` または `bb pipeline list --branch <br> -L 3 --json buildNumber,result,url` を見る。
2. `bb pipeline view <build#>` でステップと結果を確認する。
3. `bb pipeline log <build#> --step <k>` でログを読む。長い場合は必要な範囲だけに絞る。

**マージする**

1. `bb pr checks <n>` と承認状況を確認する。
2. 戦略とブランチ削除の有無をユーザーに確認する。
3. 承認後に `bb pr merge <n> --squash --delete-branch --yes` などを実行する。

## 4. Bitbucket 固有の注意

- `bb issue` はない。Bitbucket Issues は 2026-08 に廃止済み。
- 却下した PR は再オープンできない。同じブランチから作り直す。
- `pr decline --delete-branch` はフォーク側のブランチを消さない。
- リポジトリは `workspace/slug` で指定する。`bb config set workspace <ws>` を設定すると `slug` だけでも使える。
- SSH の接続先は `ssh.bitbucket.org`。`bb` は旧 `bitbucket.org` 形式の remote も認識する。
- 認証は App Password ではなく Atlassian API Token を使う。

## 5. ログイン案内

ユーザーにログインを案内するときは、次を伝える。

```text
1. https://id.atlassian.com/manage-profile/security/api-tokens を開く
2. Create API token with scopes → アプリ: Bitbucket → 必要スコープを選択 → 有効期限を設定
3. `bb auth login` を実行し、Atlassian のメールアドレスとトークンを入力
4. 必要なら `bb auth setup-git` で git push/fetch にも同じトークンを使う
```

CI などの非対話環境では `BB_EMAIL` と `BB_TOKEN` を環境変数で渡す。

## 6. トラブルシューティング

| 症状 | 対処 |
|---|---|
| exit 4 / `not logged in` | ログインを案内する |
| `HTTP 401` | トークン期限切れの可能性。`bb auth status` で期限を確認し、必要なら再作成 |
| `HTTP 403` | スコープ不足。必要スコープを付けたトークンを作り直す |
| `no Bitbucket remote found` | `-R ws/repo` を付ける、または Bitbucket リポジトリ内で実行 |
| `HTTP 429` | 自動リトライ後の失敗。件数や取得フィールドを絞る |
| `--yes required` | 非対話環境の破壊的操作。ユーザー承認後に `--yes` を付ける |
| 出力が長い | `--json` + `--jq` と `-L` で絞る |
