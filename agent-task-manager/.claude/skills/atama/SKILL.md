---
name: atama
description: Use the atama CLI to break work into dependency-ordered tasks and execute them autonomously. Use when a repo has a .atama/ directory, when the user asks to plan/track/decompose work as atama tasks or goals, when running the next→complete agent loop, or when syncing tasks with GitHub Issues.
---

# atama

`atama` is a task manager for delegated agent execution. It stores tasks as YAML under `.atama/`, tracks `depends_on` as a DAG, and answers **which task is executable right now**. It never runs anything itself — you do the work, then report back.

# Workflow

Pick the entry point, then follow the steps in order.

| Situation | Start at |
|---|---|
| No `.atama/` yet, user wants work tracked | Step 0 |
| User described work to do ("add search", "refactor X") | Step 1 |
| User said "continue" / "work the backlog" / a goal exists | Step 2 |
| Work is finished or you must stop early | Step 3 |

## Step 0 — Confirm the store

```sh
atama task list --output json
```

Exit 3 means no store exists. Create one only if the user asked to start tracking work; do not initialize a store as a side effect of some other task.

```sh
atama init                                  # or: atama init --github-repo owner/repo
```

Use `--dir <path>` to operate on a store outside the current directory. The CLI walks up from the working directory to find `.atama/`, like git finds `.git`.

## Step 1 — Plan: turn the request into a graph

Create dependencies **before** the tasks that reference them; `--depends-on` fails if the target does not exist yet.

```sh
goal=$(atama goal add "Add user search API" \
  --description "REST endpoint + index, behind a feature flag" --output json | jq -r .id)

t1=$(atama task add "Design search index schema" \
  --goal "$goal" --priority high \
  --body "Decide fields, analyzer, and migration path. Done when the schema is committed." \
  --output json | jq -r .id)

t2=$(atama task add "Implement /search endpoint" \
  --goal "$goal" --depends-on "$t1" --output json | jq -r .id)

atama task add "Write integration tests" --goal "$goal" --depends-on "$t2"
```

Then check the graph before executing anything:

```sh
atama task list --goal "$goal"     # '*' marks Ready; 'blocked' here is derived from deps
atama next --goal "$goal"          # what the loop will pick first
```

Guidelines that make the graph useful rather than decorative:

- One task = one reviewable unit of work. If you cannot state its done-condition in a sentence, split it.
- Put the done-condition and any context the executing agent will need into `--body`. Whoever picks the task up later sees only what is stored — not this conversation.
- Add `--depends-on` only for real ordering constraints. Over-linking serializes work that could run in parallel; `next` returns *all* Ready tasks, so a loose graph fans out.
- Use a goal whenever there is more than one task, so `next --goal` and `task list --goal` can scope later runs.
- Show the plan to the user before starting a long autonomous run.

Cycles are rejected at creation time (exit 2), so a rejected `dep add` means the graph already implies the reverse edge.

## Step 2 — Run the loop

One iteration, in order. Repeat until `next` returns `[]`.

**2a. Ask for work.**

```sh
task=$(atama next --goal "$goal" --output json)     # drop --goal to consider every task
id=$(echo "$task" | jq -r '.[0].id // empty')
```

`next` returns Ready tasks in dependency order. A task is Ready when it is not `done`/`cancelled`/`in_progress`/`in_review` **and every `depends_on` target is `done`**.

**2b. Empty result?** Go to Step 3 — but first find out *why* it is empty, because "all done" and "everything is stuck" look identical here:

```sh
atama task list --output json | jq -r '.[] | select(.status != "done") | "\(.status)\t\(.id)\t\(.title)"'
```

**2c. Claim the task before doing anything.**

```sh
atama task status "$id" in_progress
```

This removes it from `next`, so a concurrent agent cannot pick up the same task. Never skip this — an unclaimed task is a duplicated task.

**2d. Read the task in full.** The JSON from 2a already carries body, labels, `depends_on`, linked issue, and worktree; re-read with `atama task show "$id" --output json` if you no longer have it. Work from `body`, not from your memory of the plan.

**2e. Optionally take a worktree** (see below) if other agents are running or the task touches wide surface area.

**2f. Do the actual work.**

**2g. Report the outcome — exactly one of:**

```sh
atama complete "$id"                                    # done
atama task status "$id" in_review                       # done, awaiting human review
atama task status "$id" blocked                         # cannot proceed
atama task status "$id" cancelled                       # no longer needed
```

For anything other than `complete`, record why, so the next iteration is not blind:

```sh
atama task edit "$id" --body "$(atama task show "$id" --output json | jq -r .body)

Blocked: the search index migration needs a DB owner's approval."
```

**Never leave a task in `in_progress`.** It is invisible to `next` and stalls the loop silently. If you must stop mid-task, set it back to `backlog` or to `blocked` with a reason.

**2h. Feed discoveries back into the graph** before looping, so the ordering stays true:

```sh
new=$(atama task add "Backfill existing rows" --goal "$goal" --output json | jq -r .id)
atama dep add "$id" --on "$new"      # <id> now waits for the new task
```

**2i. Loop back to 2a.** Guard against spinning: if `next` hands you the same ID twice with no status change in between, stop and report instead of retrying.

Full loop, condensed:

```sh
while :; do
  task=$(atama next --goal "$goal" --output json)
  id=$(echo "$task" | jq -r '.[0].id // empty')
  [ -z "$id" ] && break

  atama task status "$id" in_progress
  # ... do the work described in $(echo "$task" | jq -r '.[0].body') ...
  atama complete "$id"
done
```

## Step 3 — Wrap up

Report to the user with the store as the source of truth, not your own recollection:

```sh
atama task list --goal "$goal"           # final state of every task
atama github sync --dry-run              # if the store is linked to GitHub
```

State plainly what is `done`, what is `blocked` and why, and what was `cancelled`. If a worktree is still checked out, say so and clean it up once the branch is merged or abandoned.

# Parallel execution with git worktrees

Two agents editing one checkout corrupt each other's work. Give each task its own worktree:

```sh
dir=$(atama next --worktree --output json | jq -r '.[0].execution.worktree.path')
cd "$dir"        # branch atama/<task-id>, created from HEAD
```

`--worktree` is a no-op for a task that already has one. Manage them explicitly with:

```sh
atama worktree add <id> --branch feat/search --path ../wt --start-point main
atama worktree list
atama worktree rm <id>        # --force discards uncommitted changes
```

Remove the worktree after the branch is merged or abandoned, not before — `rm` without `--force` refuses to drop uncommitted work.

# GitHub Issues

```sh
atama github import                 # issues -> tasks (--overwrite refreshes linked ones)
atama github export <id>            # task -> new issue
atama github sync --dry-run         # always preview first
atama github sync
```

Sync is bidirectional over issue fields and comment threads; conflicts resolve last-write-wins by `updated_at`, deletions beat edits. `depends_on`/`goal_ids` travel in an `<!-- atama:meta -->` block at the end of the issue body — never hand-edit or strip it. Auth comes from `GITHUB_TOKEN` or `gh auth token`; if both are absent, ask the user to run `gh auth login` rather than working around it.

# Command surface

| Command | Notes |
|---|---|
| `task add <title>` | `--body --priority --status --label --assignee --depends-on --goal` (all repeatable except body/priority/status) |
| `task list` | `--status --label --goal`; `--status` matches the **stored** status, not the derived one |
| `task show <id>` | Full task including comments and execution metadata |
| `task edit <id>` | `--title --body --priority --label --assignee`; **`--label` replaces the whole list**, and there is no `--status` here |
| `task status <id> <status>` | The only way to change status |
| `task rm <id>` | Also clears the task from goals and from other tasks' `depends_on` |
| `dep add\|rm <id> --on <id>` | `<id>` depends on `--on` |
| `goal add <name>` / `goal list` / `goal show <id>` | `goal add --task <id>` links existing tasks |
| `next [--goal <id>] [--worktree]` | Ready tasks, dependency-ordered |
| `complete <id>` | Shorthand for `task status <id> done` |
| `worktree add\|rm\|list` | See above |
| `github import\|export\|sync\|set-repo` | See above |

Statuses: `backlog` `ready` `in_progress` `blocked` `in_review` `done` `cancelled`. Transitions are unconstrained. Priorities: `low` `medium` `high` `urgent`.

Any unambiguous ID prefix works (`atm-01a0757a-e6a5`); an ambiguous prefix is a validation error, not a silent pick.

# Machine-readable output

Pass `--output json` to every command you parse — text output is for humans and is not stable. Errors go to stdout as `{"error": "..."}` in JSON mode, stderr otherwise. Branch on the exit code:

| Code | Meaning | Reaction |
|---|---|---|
| 0 | success | continue |
| 1 | I/O or unexpected error | stop and report |
| 2 | validation error (bad value, dependency cycle) | fix the input and retry |
| 3 | task/goal/store not found | check the ID, or `atama init` |

# Do not

- **Do not run `atama board`.** It is an interactive TUI for humans and will block a non-interactive session.
- Do not edit files under `.atama/` directly — a file lock guards concurrent writers, and the CLI maintains cross-references (goal membership, `depends_on`) that hand edits break.
- Do not expect `execution.executor` to run anything; v0.1 stores it and never launches it.
- Do not try to add comments from the CLI — there is no such command. Comments arrive via `github sync` or the TUI.
