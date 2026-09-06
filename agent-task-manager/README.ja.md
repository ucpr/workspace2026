# atama

コーディングエージェントに作業を委譲するためのタスク管理ツール。

atama はタスク・依存関係・状態をリポジトリ内のプレーンな YAML で管理する。タスクの実行そのものは行わず、エージェントが必要とする唯一の問い —— **次に何をやればいいか** —— にだけ答える。

English: [README.md](README.md)

## なぜ

エージェント（Claude Code, Codex など）はタスクの実行は得意だが、どのタスクが着手可能かの判断は苦手だ。Issue やチャットの指示には機械可読な順序が含まれておらず、結局は人間が手で順番を決めることになる。

atama はその順序を明示する。

- タスクは `depends_on` で DAG を構成する。循環は拒否される。
- `atama next` は依存がすべて満たされたタスクを依存順に JSON で返す。
- エージェントは実行し、`atama complete <id>` を呼び、次を取得する。このループがプロトコルのすべて。
- 1タスク1ファイルの YAML を `.atama/` に置くため、タスクの状態変化をコードと同じように git でレビューできる。

## インストール

```sh
go install github.com/ucpr/atama/cmd/atama@latest
```

Go 1.25 以上が必要。GitHub 連携には `gh`（または `GITHUB_TOKEN`）を使う。

## クイックスタート

```sh
cd your-repo
atama init

goal=$(atama goal add "ユーザー検索APIの追加" --output json | jq -r .id)

t1=$(atama task add "検索インデックスのスキーマ設計" --goal "$goal" --priority high --output json | jq -r .id)
t2=$(atama task add "/search エンドポイントの実装" --goal "$goal" --depends-on "$t1" --output json | jq -r .id)
atama task add "結合テストの作成" --goal "$goal" --depends-on "$t2"
```

```console
$ atama task list
* atm-01a0757a-e6a5-822b-870a-98ad71644403  backlog      high    検索インデックスのスキーマ設計
  atm-01a0757a-e6b6-870d-8630-b34aa3fddb38  blocked      medium  /search エンドポイントの実装
  atm-01a0757a-e6c7-8311-9cd6-24094a33deff  blocked      medium  結合テストの作成
```

`*` は Ready なタスク。他の2件の `blocked` は誰かが設定した値ではなく、未解決の依存から**導出された**状態。

```console
$ atama next --goal "$goal"
Next task: atm-01a0757a-e6a5-822b-870a-98ad71644403  検索インデックスのスキーマ設計

$ atama complete atm-01a0757a-e6a5
atm-01a0757a-e6a5-822b-870a-98ad71644403  [done/high]  検索インデックスのスキーマ設計

$ atama next --goal "$goal"
Next task: atm-01a0757a-e6b6-870d-8630-b34aa3fddb38  /search エンドポイントの実装
```

ID は一意に定まる限り前方一致で指定できるので、`atm-01a0757a-e6a5` で十分。

## ユースケース

### 1. エージェントがゴールを最後まで進める

全コマンドが `--output json` に対応しているため、ループは数行のシェルで書ける。

```sh
while :; do
  task=$(atama next --goal "$goal" --output json)
  id=$(echo "$task" | jq -r '.[0].id // empty')
  [ -z "$id" ] && break

  claude -p "$(echo "$task" | jq -r '.[0] | "Task: \(.title)\n\n\(.body)"')"
  atama complete "$id"
done
```

`next` は本文・ラベル・依存関係・紐づく Issue・worktree など、実行に必要な情報をすべて返す。

```json
[
  {
    "id": "atm-01a0757a-e6a5-822b-870a-98ad71644403",
    "title": "検索インデックスのスキーマ設計",
    "status": "backlog",
    "priority": "high",
    "goal_ids": ["atm-g-01a0757a-e691-83c2-84de-6c5094837583"],
    "effective_status": "backlog",
    "ready": true
  }
]
```

### 2. 複数エージェントの並列実行（1タスク1 worktree）

同じチェックアウトで2つのエージェントを動かすと互いの変更を壊す。`--worktree` を付けると、次のタスクに専用のブランチとディレクトリを同じ呼び出しで割り当てる。

```console
$ atama next --worktree
Next task: atm-01a0757a-e6b6-870d-8630-b34aa3fddb38  /search エンドポイントの実装
worktree: /path/to/your-repo-worktrees/atm-01a0757a-e6b6-…  (branch atama/atm-01a0757a-e6b6-…)
```

```sh
dir=$(atama next --worktree --output json | jq -r '.[0].execution.worktree.path')
(cd "$dir" && claude -p "...")

atama worktree list          # 実行中のものを確認
atama worktree rm "$id"      # 後片付け（--force で未コミットの変更を破棄）
```

パスやブランチは指定可能: `atama worktree add <id> --branch feat/search --path ../wt --start-point main`

### 3. 人間によるボード上での整理

```sh
atama board
```

vim ライクなキーバインドのカンバンボード。カラムがステータスで、`space` でタスクを進め、`D` で依存関係を編集、`g` で依存グラフを表示、`S` で GitHub sync を実行する。CLI とファイルを共有しているため、エージェントの `complete` は `r`（リロード）で反映される。

### 4. GitHub Issues との双方向連携

```sh
atama init --github-repo owner/repo     # または: atama github set-repo owner/repo

atama github import                     # 既存 Issue -> タスク
atama github export "$t2"               # タスク -> 新規 Issue
atama github sync --dry-run             # 差分のプレビュー
atama github sync                       # 適用
```

sync は双方向で、Issue 本体だけでなくコメントスレッドも対象にする。コンフリクトは `updated_at` による後勝ち、削除は編集より優先される。`depends_on` と `goal_ids` は GitHub 側に対応する項目がないため、Issue 本文末尾の不可視な `<!-- atama:meta -->` ブロックに埋め込まれる。認証は `gh auth token`（`GITHUB_TOKEN` があればそちら優先）に委譲し、トークンをリポジトリに書き出すことはない。

## コマンド

| コマンド | 説明 |
|---|---|
| `init [--github-repo owner/repo]` | カレントディレクトリに `.atama` ストアを作成 |
| `task add <title>` | タスク作成（`--body --priority --status --label --assignee --depends-on --goal`） |
| `task list` | 一覧（`--status --label --goal`） |
| `task show <id>` | 詳細表示 |
| `task edit <id>` | タイトル/本文/優先度/ラベル/担当者の更新 |
| `task rm <id>` | 削除 |
| `task status <id> <status>` | ステータスを直接変更 |
| `dep add <id> --on <id>` | 依存関係を追加（循環は拒否） |
| `dep rm <id> --on <id>` | 依存関係を削除 |
| `goal add <name>` / `goal list` / `goal show <id>` | ゴールの管理 |
| `next [--goal <id>] [--worktree]` | Ready なタスクを依存順に取得 |
| `complete <id>` | タスクを完了として報告 |
| `worktree add/rm/list` | タスクごとの git worktree を管理 |
| `github import/export/sync/set-repo` | GitHub Issues 連携 |
| `board` | TUI を起動 |

グローバルフラグ: `--output text|json`、`--dir <path>`

ステータス: `backlog` `ready` `in_progress` `blocked` `in_review` `done` `cancelled`。遷移に制約はなく、`done` から `in_progress` に戻すこともできる。

優先度: `low` `medium` `high` `urgent`

## エージェント向けの仕様

エラーは構造化され、終了コードで判別できる。テキストをパースする必要はない。

```console
$ atama dep add atm-01a0757a-e6a5 --on atm-01a0757a-e6c7 --output json
{"error":"adding dependency atm-01a0757a-e6a5-… -> atm-01a0757a-e6c7-… would create a cycle"}
$ echo $?
2
```

| コード | 意味 |
|---|---|
| 0 | 成功 |
| 1 | 想定外 / I/O エラー |
| 2 | バリデーションエラー（不正な入力、循環依存） |
| 3 | タスク・ゴール・ストアが見つからない |

変更系コマンドはすべて非対話的に実行できる。複数プロセス（複数エージェント + TUI）からの同時アクセスはファイルロックで保護される。

## データの保存形式

`atama init` が `.atama/` を作成し、以降は git が `.git` を探すのと同じようにカレントディレクトリから上へ辿って発見する。1タスク1ファイルなので diff が意味を持つ。

```
.atama/
├── config.yaml
├── goals/atm-g-01a0757a-….yaml
└── tasks/atm-01a0757a-….yaml
```

```yaml
id: atm-01a0757a-e6b6-870d-8630-b34aa3fddb38
title: /search エンドポイントの実装
status: backlog
priority: medium
depends_on:
    - atm-01a0757a-e6a5-822b-870a-98ad71644403
goal_ids:
    - atm-g-01a0757a-e691-83c2-84de-6c5094837583
created_at: 2026-09-06T06:49:43.86269Z
updated_at: 2026-09-06T06:49:59.040434Z
execution:
    worktree:
        path: /path/to/your-repo-worktrees/atm-01a0757a-e6b6-…
        branch: atama/atm-01a0757a-e6b6-…
```

ID は `atm-`（タスク）/ `atm-g-`（ゴール）プレフィックス + UUIDv8。先頭48ビットが生成時刻のため、文字列ソートがそのまま時刻順になる。`config.yaml` は将来のマイグレーション用に `schema_version` を持つ。

## TUI キーバインド

```
h/← l/→   カラム移動         j/↓ k/↑    タスク移動
enter     詳細を開く         space      ステータスを進める
H / L     タスクを左右へ移動  n          新規タスク
e         編集               d          削除
D         依存関係の編集      g          依存グラフ
W         worktree 作成      G          ゴールで絞り込み
f         ラベル/優先度で絞り込み
s         ソート切替          /          検索
S         github sync        i          github import
c         コメント追加        tab        詳細タブ切替
r         リロード            ?          ヘルプ
esc       閉じる/解除         q / ctrl+c 終了
```

## スコープ

v0.1 は個人・単一マシン・単一リポジトリを前提とする。複数人での共有、複数リポジトリ横断、実行フックのランナー本体（`execution.executor` は保持・受け渡しのみで起動はしない）は対象外だが、拡張できる設計になっている。詳細な要件は [specs/requirements.md](specs/requirements.md) を参照。
