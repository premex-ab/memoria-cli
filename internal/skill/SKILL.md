---
name: memoria
description: Use when working with Memoria (https://memoria.premex.se) — bi-temporal AI agent memory served over MCP. Activates on any session that has the memoria MCP server configured, or when the user mentions brains/episodes/entities/edges/playbooks/recall/remember.
version: 0.1.0
---

# Memoria — bi-temporal AI agent memory

## The four-concept mental model

| Plain word | Technical name | What it is | Like... |
|---|---|---|---|
| Container | brain | Memory isolation unit. Each agent gets its own brain. | A separate notebook, not a tag on a shared one. |
| Note | episode | Something that happened, in narrative form. The raw input. | A diary entry or wiki-page edit. |
| Thing | entity | A discrete thing the agent wants to remember and refer back to. | A row in a tracker — a library, a bug, a PR, a person. |
| Fact | edge | A relationship between two things, with timestamps. | A cell linking two rows in a table. |
| Briefing | playbook | A synthesized summary of what's known in this brain. | The doc an agent reads at session start to catch up. |

The API and dashboard use the technical names. The plain words are how to think about them.

## The brain as the container

A brain is the memory boundary for one agent. Every API key is bound to exactly one brain. Queries against brain A structurally cannot return data from brain B — this is path-based isolation (separate Firestore paths), not convention-based isolation.

**One tenant, many brains.** The tenant is the billing unit. Under it, create one brain per agent, project, or cron — whatever needs separate memory. A second brain needs a second API key.

**Default brain.** Every account gets a `default` brain. Single-agent setups never think about brains explicitly — the key handles everything.

**Multi-brain.** When separation is needed (two crons, two projects, personal vs. work), create a second brain in the dashboard, mint a key for it, and hand that key to the second agent. Both share one bill; neither can read the other's data.

## When to use each MCP tool

**`remember(content, source, metadata?)`**
Runs a 7-stage extraction pipeline server-side: entity extraction → entity resolution → fact extraction → dedup → temporal → contradiction → commit. Cost: ~$0.10–0.30 per call. Use for findings worth keeping — PR opened, upstream change observed, bug confirmed. Do not use for mid-run trivia.

**`recall(query, asOf?)`**
Semantic + sparse + graph hybrid search across the brain. Cheap. Use when the agent thinks it already knows something about a topic.

**`get_playbook(branch?, file?, sessionId?)`**
Load the synthesized briefing at session start instead of re-reading raw history. Cheap. Use at the beginning of every session.

**`regenerate_playbook(branch?, file?, sessionId?)`**
Refresh the briefing from current facts so the next session starts fresh. A few cents per call. Use at the end of every session after saving notes.

**`recall_history(entityId)`**
Bi-temporal timeline: how has knowledge about one thing changed over time. Cheap. Use for "what was the state of this bug last week?"

**`forget(edgeId)`**
Mark a specific fact invalid — soft delete, history preserved. Cheap. Use to correct stale or wrong facts. Do not hand-edit playbooks; fix the underlying facts instead.

**`relate(fromEntityId, relationType, toEntityId, factText)`**
Hand-curated relation between two known entities — skips the extraction pipeline. Cheap. Use for curation or stitching two things the agent already knows about.

## Sub-scopes within a brain

`branch`, `file`, and `sessionId` are optional refinements passed to `get_playbook` and `regenerate_playbook`. They narrow which playbook is loaded or regenerated.

`recall()` always searches the **entire brain** regardless of sub-scope arguments. Sub-scopes affect briefings only.

A briefing scope is exact-match: `{branch: "main"}` does not match a stored `{branch: "main", file: "index.ts"}`. Pick a consistent scope shape per use case and stick to it.

Example — a coding-agent brain with per-branch briefings:

```
get_playbook({ branch: "feature-x" })
regenerate_playbook({ branch: "feature-x" })
```

## The two clocks

Each fact carries two time pairs:

- **`tValid`** — when the fact became true in the world (e.g. "compose 1.7.2 shipped on 2026-05-27").
- **`tIngested`** — when Memoria learned it (e.g. "agent saved this on 2026-05-28").

The `asOf` parameter on `recall` lets the agent ask "what did I think was true last week?" and get an honest answer, even after contradicting information has been ingested since.

```
recall({ query: "compose version", asOf: "2026-05-20T00:00:00Z" })
```

## Cost expectations

| Operation | Cost |
|---|---|
| `remember` | ~$0.10–0.30 per call (7-stage LLM pipeline) |
| `regenerate_playbook` | A few cents (synthesis LLM call) |
| `recall`, `get_playbook`, `recall_history`, `forget`, `relate` | Cheap |

`remember` is the only expensive operation. A daily cron saving 5–10 findings per run costs roughly $1–3/day.

## Anti-patterns

**Do not bulk-import historical data into a new brain.** Running all past notes through `remember` causes a cost spike and typically produces worse synthesis than letting fresh runs accumulate naturally. Let the next few runs fill in context organically.

**Do not hand-edit playbooks.** The briefing is synthesized from facts. If the briefing is wrong, the fix is to save better notes (`remember`) or invalidate stale facts (`forget`) — not to edit the playbook text directly.

**Do not use `recall` as a substitute for `get_playbook` at session start.** `get_playbook` returns a synthesized, human-readable briefing. `recall` is for targeted lookups during a session.

## Multi-brain setups

Run one brain per agent, cron, or project. Examples:

- Glyph maintenance cron → brain "glyph-maintenance" → key A
- Memoria self-maintenance cron → brain "memoria-self-maintenance" → key B
- Personal coding assistant → brain "personal" → key C

Each brain's `remember`, `recall`, `get_playbook`, and `regenerate_playbook` calls operate exclusively within that brain. No `project:` argument needed — the brain is implicit from the key. All brains share one tenant and one bill.

To add a brain: dashboard → `/dashboard/brains` → New Brain → mint a key → hand the key to the agent.
