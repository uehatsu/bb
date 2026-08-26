# MAGI 指摘（v0.1.2..HEAD 審議）対応プラン — v0.1.4

作成日: 2026-08-26 / 対象: https://github.com/uehatsu/bb（HEAD `b275fed`、v0.1.3 リリース済み）

## 0. 対応する指摘

| # | 指摘 | 出所 | 優先 |
|---|---|---|---|
| 1 | SKILL.md の「承認後に `--yes`」が一般化されすぎ。`--yes` を持つのは `pr merge` / `repo delete` / `branch delete` のみで、`pr decline` / `pipeline stop` / `repo edit --visibility` に付けると `unknown flag` | CASPER | 中 |
| 2 | 3 つの SKILL.md（本文共通・frontmatter 差分）の同期が人力頼みで、CI に drift 検出が無い | BALTHASAR / CASPER | 中 |
| 3 | 回帰テストに「グループコマンドを引数無しで実行 → ヘルプ・exit 0」「`--help` → exit 0」のケースが無い | BALTHASAR | 低 |
| 4 | Claude Code 版 frontmatter の `user_invocable` / `trigger` は Claude Code が解釈するキーではない | CASPER | 低 |
| 5 | Makefile: `cp` が既存 SKILL.md を無確認で上書き、`$(HOME)` 未クォート、`install-skill` が未使用エージェントの `~/.codex` `~/.copilot` も作る | MELCHIOR | 低 |
| 6 | root（`bb bogus`）と group（`bb pr bogus`）で不明コマンド時の出力が不揃い（root は Usage 無し） | BALTHASAR | 低 |
| 7 | Go 1.26 要件引き上げ・過去の変更がリリースノートに無い。同一修正の 3 連続コミット | BALTHASAR | 低 |

## 1. 方針と非対象

- **履歴の書き換えはしない**（#7 の squash）。`v0.1.3` タグ・`main` は公開済みで、rewrite は利用者の clone を壊す。代わりに **CHANGELOG.md** を導入して以後のリリースノートの正本とする。
- SKILL.md は **生成物**にする（#2）。ソースは 1 本の本文 + エージェント別 frontmatter。生成物はコミットし続ける（`make install-*` と、リポジトリから直接コピーする利用者のため）。CI は「生成し直して差分が無いこと」を検査する（`docs/reference` と同じ方式）。
- `pr decline` / `pipeline stop` に `--yes` を**追加しない**（MELCHIOR 警告 3 は認識した上で据え置き）。理由: どちらも取り消し可能（decline → 新 PR、stop → 再実行）で、gh の `pr close` / `run cancel` にも確認は無く、互換性を優先する。SKILL.md 側で「承認は必要だがフラグは無い」と正確に書く（#1）。

## 2. 変更内容

### Step 1: SKILL.md の生成方式（#1, #2, #4）
- `skills/bitbucket.body.md` — 共通本文（現在の 3 ファイルの本文。`AGENT_NAME` プレースホルダ）
- `skills/<agent>/bitbucket/frontmatter.md` — エージェント別 frontmatter（`claude_code` は `name` / `description` のみに整理し `user_invocable` / `trigger` を削除、`codex` は `metadata.short-description`、`copilot` は `license`）
- `scripts/gen-skills.sh` — `frontmatter.md` + 本文（`AGENT_NAME` 置換）→ `skills/<agent>/bitbucket/SKILL.md`。POSIX sh のみ（Go 不要）。`AGENT_NAME` は frontmatter.md 先頭のコメント行 `<!-- agent: Claude Code -->` から取る
- `make skills` で生成、`make check-skills` で「生成 → `git diff --exit-code -- skills/`」。CI（`ci.yml` の docs チェックと同じジョブ）に `make check-skills` を追加
- 本文の修正（#1）: Ground rule 3 と Destructive-operation workflow を次のとおり正確化
  - `--yes` を持つコマンドは **`pr merge` / `repo delete` / `branch delete` の 3 つだけ**。非対話環境では `repo delete` / `branch delete` は `--yes` が**必須**、`pr merge` は省略可（TTY でなければ確認プロンプトは出ない）
  - `pr decline` / `pipeline stop` / `repo edit --visibility` は**承認を得てから、フラグ無しで**実行する（`--yes` を付けると `unknown flag` で失敗する）
  - 「Destructive-operation workflow」の step 3 を「run the command (add `--yes` only where the command has it)」に変更
- README / README_JA の Agent skills 節を「`skills/bitbucket.body.md` を編集して `make skills`」に更新

### Step 2: root と group の統一 + テスト（#3, #6）
- `pkg/cmd/root/root.go`: root にも `Args: cobra.ArbitraryArgs` + `RunE: cmdutil.GroupRunE` を設定し、`bb bogus` も group と同じ `unknown command "bogus" for "bb"` + Usage + exit 1 にする。`bb`（引数無し）はヘルプ exit 0、`bb --version` / `bb --help` は従来どおり
- `pkg/cmd/root/root_test.go` に追加:
  - `TestGroupWithoutArgsShowsHelp`: 8 グループ + root を引数無しで実行 → `err == nil`、出力に各コマンドの `Short` を含む
  - `TestGroupHelpFlag`: `bb pr --help` → `err == nil`
  - `TestUnknownSubcommandFails` に root ケース `{"bogus"}` を追加
- `cmd/bb/main_test.go`: `execute(ctx, f, []string{"pr", "bogus"})` → exit 1、`[]string{"pr"}` → exit 0

### Step 3: Makefile の安全化（#5）
- 全ターゲットで `"$(HOME)"` をクォート
- `install-<agent>-skill`: 既存の `SKILL.md` がリポジトリ版と異なる場合は `SKILL.md.bak` に退避してから上書きし、その旨を表示（`cmp -s` で判定）
- `install-skill` は従来どおり 3 つすべて（README に「使わないエージェントのディレクトリも作られる。個別ターゲットを使えば作られない」と注記）

### Step 4: CHANGELOG（#7）
- `CHANGELOG.md`（Keep a Changelog 形式）を追加。`0.1.0`〜`0.1.3` を遡って記載（0.1.1: PR URL / branch delete `--yes` / store 移行、0.1.2: pipeline 番号解決、0.1.3: 不明サブコマンド exit 1、`go install` に Go 1.26 が必要になった旨）。`Unreleased` に本プランの内容
- `.goreleaser.yml` の `changelog` は既存のコミットベース生成のまま（CHANGELOG.md は人間向け正本）。README の Development 節に「リリース前に CHANGELOG.md を更新」を追記

### Step 5: 検証・リリース
- `go test ./...`、`golangci-lint run`、`make check-skills`、`make docs` の差分無しを**ローカルで確認してから**コミット
- `make install-skill` で 3 箇所を更新し、Claude Code のスキル一覧に `bitbucket` が引き続き現れることを確認（#4 の frontmatter 変更の検証）
- コミットは Step ごと（4 コミット）。`v0.1.4` タグは全 CI 緑を確認後に 1 回だけ打つ

## 3. 完了条件
- `bb pr decline --yes` のような誤用を SKILL.md が誘発しない（本文に対応コマンドが明記）
- `make check-skills` が CI で動き、3 ファイルを個別に編集すると CI が落ちる
- `bb`, `bb <group>`, `bb <group> --help` → exit 0 / `bb bogus`, `bb <group> bogus` → exit 1 がテストで固定
- Makefile が既存のローカル編集を失わない
- CHANGELOG.md に 0.1.0〜0.1.4 の履歴

## 4. リスク
- root に `RunE` を持たせると cobra の `--version` 処理順が変わる可能性 → テストで `bb --version` の出力を固定
- frontmatter から `user_invocable` / `trigger` を外すと `/bitbucket` が出なくなる可能性 → Step 5 で確認し、出なければ `name` だけで足りない証拠として戻す（Claude Code 公式のキーは `name` / `description` / `allowed-tools` / `disable-model-invocation`）
