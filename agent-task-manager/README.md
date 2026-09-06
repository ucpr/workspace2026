# atama

A task manager for delegating work to coding agents.

atama tracks tasks, their dependencies, and their state in plain YAML files inside your repository. It never runs your tasks — it answers the one question an agent needs answered: **what should I work on next?**

日本語版: [README.ja.md](README.ja.md)

## Why

Agents (Claude Code, Codex, …) are good at executing a task and bad at deciding which task is executable. Issues and chat instructions carry no machine-readable ordering, so a human ends up sequencing the work by hand.

atama makes that ordering explicit:

- Tasks form a DAG via `depends_on`. Cycles are rejected.
- `atama next` returns the tasks whose dependencies are all satisfied, in dependency order, as JSON.
- The agent executes, calls `atama complete <id>`, and asks for the next one. That loop is the whole protocol.
- A `.atama/` directory of one-task-per-file YAML lives in git, so task state reviews like code.

## Install

```sh
go install github.com/ucpr/atama/cmd/atama@latest
```

Requires Go 1.25+. GitHub integration additionally uses `gh` (or `GITHUB_TOKEN`).

## Quick start

```sh
cd your-repo
atama init

goal=$(atama goal add "Add user search API" --output json | jq -r .id)

t1=$(atama task add "Design search index schema" --goal "$goal" --priority high --output json | jq -r .id)
t2=$(atama task add "Implement /search endpoint" --goal "$goal" --depends-on "$t1" --output json | jq -r .id)
atama task add "Write integration tests" --goal "$goal" --depends-on "$t2"
```

```console
$ atama task list
* atm-01a0757a-e6a5-822b-870a-98ad71644403  backlog      high    Design search index schema
  atm-01a0757a-e6b6-870d-8630-b34aa3fddb38  blocked      medium  Implement /search endpoint
  atm-01a0757a-e6c7-8311-9cd6-24094a33deff  blocked      medium  Write integration tests
```

`*` marks Ready tasks. The other two show `blocked` — a *derived* status, computed from unmet dependencies, not something anyone typed.

```console
$ atama next --goal "$goal"
Next task: atm-01a0757a-e6a5-822b-870a-98ad71644403  Design search index schema

$ atama complete atm-01a0757a-e6a5
atm-01a0757a-e6a5-822b-870a-98ad71644403  [done/high]  Design search index schema

$ atama next --goal "$goal"
Next task: atm-01a0757a-e6b6-870d-8630-b34aa3fddb38  Implement /search endpoint
```

Any unambiguous ID prefix works, so `atm-01a0757a-e6a5` is enough.

## How to develop with it

The cycle is: **plan a graph → review it → let an agent drain it → commit the result.**

**1. Plan.** Ask an agent to decompose the request, or write the tasks yourself. Two rules make the graph worth having: one task = one reviewable unit of work, and `--body` carries the done-condition, because the agent that picks the task up later sees only what is stored.

```sh
atama task add "Implement /search endpoint" --goal "$goal" --depends-on "$t1" \
  --body "Add GET /search backed by the index from the previous task. Done when it returns ranked results and the handler is unit-tested."
```

Add `--depends-on` only for real ordering constraints — `next` returns *every* Ready task, so a loose graph fans out to parallel agents while an over-linked one runs single-file.

**2. Review before executing.** `atama task list --goal "$goal"` shows what is Ready (`*`) and what is `blocked` by dependencies; `atama board` is the same view with editing. This is the cheap moment to fix ordering — after the agent starts, it is a rebase.

**3. Execute.** Each iteration is: `next` → claim → work → report.

```sh
id=$(atama next --goal "$goal" --output json | jq -r '.[0].id // empty')
atama task status "$id" in_progress    # claim: this removes it from `next`
# ... do the work ...
atama complete "$id"
```

Claiming matters: a task left in `backlog` while you work on it will be handed to the next agent too. And a task left in `in_progress` disappears from `next` forever — if you stop mid-task, set it back to `backlog`, or to `blocked` and record why in `--body`.

**4. Adjust as you learn.** Work uncovers work. Put it in the graph instead of a scratchpad, so the ordering stays true:

```sh
new=$(atama task add "Backfill existing rows" --goal "$goal" --output json | jq -r .id)
atama dep add "$id" --on "$new"
```

**5. Commit `.atama/` with the code.** One file per task means the diff shows exactly which task moved to `done` and which dependencies changed — the plan is reviewed in the same PR as the implementation. If the work also lives on GitHub, close the loop with `atama github sync --dry-run` then `atama github sync`.

An empty `atama next` means either "finished" or "everything is stuck", and they look identical. Check with `atama task list` before declaring victory.

> Using Claude Code? [`.claude/skills/atama/SKILL.md`](.claude/skills/atama/SKILL.md) teaches this workflow to the agent, including the JSON shapes and exit codes it should branch on.

## Use cases

### 1. An agent working a goal to completion

Every command takes `--output json`, so the loop is a few lines of shell:

```sh
while :; do
  task=$(atama next --goal "$goal" --output json)
  id=$(echo "$task" | jq -r '.[0].id // empty')
  [ -z "$id" ] && break

  claude -p "$(echo "$task" | jq -r '.[0] | "Task: \(.title)\n\n\(.body)"')"
  atama complete "$id"
done
```

`next` returns everything needed to act — body, labels, dependencies, linked issue, worktree:

```json
[
  {
    "id": "atm-01a0757a-e6a5-822b-870a-98ad71644403",
    "title": "Design search index schema",
    "status": "backlog",
    "priority": "high",
    "goal_ids": ["atm-g-01a0757a-e691-83c2-84de-6c5094837583"],
    "effective_status": "backlog",
    "ready": true
  }
]
```

### 2. Parallel agents, one git worktree each

Two agents on the same checkout overwrite each other. `--worktree` hands the next task its own branch and directory in the same call:

```console
$ atama next --worktree
Next task: atm-01a0757a-e6b6-870d-8630-b34aa3fddb38  Implement /search endpoint
worktree: /path/to/your-repo-worktrees/atm-01a0757a-e6b6-...  (branch atama/atm-01a0757a-e6b6-...)
```

```sh
dir=$(atama next --worktree --output json | jq -r '.[0].execution.worktree.path')
(cd "$dir" && claude -p "...")

atama worktree list          # what's currently in flight
atama worktree rm "$id"      # clean up (--force discards uncommitted changes)
```

Paths and branches are overridable: `atama worktree add <id> --branch feat/search --path ../wt --start-point main`.

### 3. Human triage on the board

```sh
atama board
```

A vim-keyed kanban board: columns are statuses, `space` advances a task, `D` edits dependencies, `g` draws the dependency graph, `S` runs a GitHub sync. It shares the same files as the CLI, so an agent's `complete` shows up on `r` (reload).

### 4. GitHub Issues in both directions

```sh
atama init --github-repo owner/repo     # or: atama github set-repo owner/repo

atama github import                     # existing issues -> tasks
atama github export "$t2"               # task -> new issue
atama github sync --dry-run             # preview the diff
atama github sync                       # apply it
```

Sync is bidirectional and covers comment threads, not just the issue body. Conflicts resolve last-write-wins by `updated_at`; deletions beat edits. `depends_on` and `goal_ids` have no GitHub equivalent, so they ride along in an invisible `<!-- atama:meta -->` block at the end of the issue body. Auth comes from `gh auth token`, or `GITHUB_TOKEN` if set — nothing is written to the repo.

## Commands

| Command | Description |
|---|---|
| `init [--github-repo owner/repo]` | Create a `.atama` store in the current directory |
| `task add <title>` | Create a task (`--body --priority --status --label --assignee --depends-on --goal`) |
| `task list` | List tasks (`--status --label --goal`) |
| `task show <id>` | Show one task |
| `task edit <id>` | Update title/body/priority/labels/assignees |
| `task rm <id>` | Delete a task |
| `task status <id> <status>` | Set status directly |
| `dep add <id> --on <id>` | Add a dependency (rejects cycles) |
| `dep rm <id> --on <id>` | Remove a dependency |
| `goal add <name>` / `goal list` / `goal show <id>` | Manage goals |
| `next [--goal <id>] [--worktree]` | Ready tasks, in dependency order |
| `complete <id>` | Mark a task done |
| `worktree add/rm/list` | Manage per-task git worktrees |
| `github import/export/sync/set-repo` | GitHub Issues interop |
| `board` | Launch the TUI |

Global flags: `--output text|json`, `--dir <path>`.

Statuses: `backlog` `ready` `in_progress` `blocked` `in_review` `done` `cancelled`. Transitions are unconstrained — moving `done` back to `in_progress` is allowed.

Priorities: `low` `medium` `high` `urgent`.

## Agent-facing details

Errors are structured and exit codes are meaningful, so callers branch without parsing prose:

```console
$ atama dep add atm-01a0757a-e6a5 --on atm-01a0757a-e6c7 --output json
{"error":"adding dependency atm-01a0757a-e6a5-... -> atm-01a0757a-e6c7-... would create a cycle"}
$ echo $?
2
```

| Code | Meaning |
|---|---|
| 0 | success |
| 1 | unexpected / I/O error |
| 2 | validation error (bad input, dependency cycle) |
| 3 | task, goal, or store not found |

Every mutating command is non-interactive. Concurrent processes (several agents, plus an open TUI) are guarded by a file lock.

## Storage

`atama init` creates `.atama/` and `atama` finds it by walking up from the working directory, the way git finds `.git`. One YAML file per task keeps diffs meaningful:

```
.atama/
├── config.yaml
├── goals/atm-g-01a0757a-….yaml
└── tasks/atm-01a0757a-….yaml
```

```yaml
id: atm-01a0757a-e6b6-870d-8630-b34aa3fddb38
title: Implement /search endpoint
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

IDs are `atm-` (tasks) or `atm-g-` (goals) plus a UUIDv8 whose leading 48 bits are the creation timestamp — string sort equals chronological sort. `config.yaml` carries a `schema_version` for future migrations.

## TUI keys

```
h/← l/→   focus column      j/↓ k/↑    focus task
enter     open detail       space      advance status
H / L     move task         n          new task
e         edit task         d          delete task
D         edit dependencies g          dependency graph
W         create worktree   G          filter by goal
f         filter label/priority
s         cycle sort        /          search
S         github sync       i          github import
c         add comment       tab        switch detail tab
r         reload            ?          toggle help
esc       close/clear       q / ctrl+c quit
```

## Scope

v0.1 is single-user, single-machine, single-repository. Multi-user sharing, cross-repo tasks, and the executor-hook runner (`execution.executor` is stored and passed through, never launched) are out of scope but designed for. Full requirements: [specs/requirements.md](specs/requirements.md).
