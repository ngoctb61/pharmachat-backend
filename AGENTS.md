# AGENTS.md

Instructions for AI coding agents other than Claude Code (Codex, Cursor, Copilot, Gemini CLI and similar). Claude Code reads `CLAUDE.md` and `.claude/` on its own and does not need this file.

**Read [`CLAUDE.md`](CLAUDE.md) first.** It is the single source of truth for what this repo is, the commands, the architecture and tenant isolation. This file only adds what Claude Code loads automatically and other agents do not.

## Rules you must load yourself

Claude Code applies these files automatically. Read them before working in the matching area:

| File | When |
|---|---|
| `.claude/rules/workflow.md` | Always: Jira issue per change, scope, branch and commit naming, secrets |
| `.claude/rules/go-code.md` | Editing any `*.go` file |
| `.claude/rules/migrations.md` | Editing `migrations/`, `internal/store/queries/` or `sqlc.yaml` |

`.claude/skills/*/SKILL.md` describe step-by-step procedures (`start-task`, `review-task`, `fix-review`, `new-migration`, `verify-stack`). Follow the matching one by hand when doing that kind of work. The personas they use are in `.claude/agents/` (`aarch`, `aeng`, `asec`).

Task work keeps its plan, decisions, summary and reviews in `ticketed/PHA-n/`, committed with the branch. Do not change code for a task until the user has approved `ticketed/PHA-n/plan.md`; in Claude Code a hook enforces this, for you it is a rule.

## Guards that only exist for Claude Code

`.claude/settings.json` and its hooks enforce these for Claude sessions only. Nothing stops you, so follow them yourself:

- Never read, print or commit `deploy/.env`. Sample values live in `deploy/.env.example`.
- Never edit `internal/store/*.go` by hand; run `make gen`.
- Run `gofmt` on Go files you change.
- Never push to `master` and never force-push. Push a `feat/PHA-n-<scope>`, `fix/PHA-n-<scope>` or `chore/<scope>` branch and open a PR. GitHub Free cannot protect `master` on this private repo.

## Business docs and tasks

Specs, the constitution, business decisions B-1..B-6 and tasks live on Atlassian (`pharmachat.atlassian.net`): Confluence space `SD` and Jira project `PHA`. See `CLAUDE.md`. `.mcp.json` registers the Atlassian MCP server (`https://mcp.atlassian.com/v2/mcp`) for Claude Code; configure the same server in your own agent to read them.
