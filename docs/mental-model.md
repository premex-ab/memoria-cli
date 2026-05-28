# Memoria: the mental model

Memoria is memory for AI agents that lasts beyond a single session. An agent runs, learns things, saves them. Next session — even days later, even a different agent operating in the same scope — it starts already knowing what was learned, without having to re-read everything raw.

This doc explains the mental model in plain words.

## Five words

Memoria's surface uses five concepts. The technical names come from knowledge-graph research, but they map cleanly to ideas you already have:

| Plain word | Technical name | What it is | Like... |
| --- | --- | --- | --- |
| **Container** | brain | The memory isolation unit. Each agent gets its own brain. | A separate notebook, not a tag on a shared one. |
| **Note** | episode | Something that happened, in narrative form. The raw input. | A diary entry. A wiki page edit. A daily-cron log entry. |
| **Thing** | entity | A discrete thing your agent wants to remember and refer back to. | A row in a tracker — a library, a bug, a PR, a person. |
| **Fact** | edge | A relationship between two things, with timestamps. | A cell in a table that links two rows. |
| **Briefing** | playbook | A synthesized summary of what's known in this brain. | The doc you read at the start of a session to catch up. |

The technical names (brain, episode, entity, edge, playbook) are what you'll see in the API and dashboard. The plain words are how to think about them.

## The brain: first-class isolation

A **brain** is the memory boundary for one agent. Every API key is bound to exactly one brain. Queries against brain A structurally cannot return data from brain B — it is path-based isolation (separate Firestore paths), not convention-based isolation (a filter you have to remember to add).

**One tenant, many brains.** Your Memoria account (the *tenant*) is the billing unit — one Stripe bill, one set of team members. Under that account you create brains: one per agent, per project, per cron, per whatever makes sense as a separate memory context.

**Default brain.** Every new account comes with a `default` brain. If you only ever run one agent, you will never think about brains explicitly — you use the default, the key is bound to it, and everything just works.

**Multi-brain.** When you want a second isolated memory (a second cron, a different project, a personal vs. work split), you create a second brain in the dashboard (`/dashboard/brains`), mint a key for it, and hand that key to the second agent. The two agents share your account and your bill — but cannot read each other's memory by construction.

Sub-scopes (`branch`, `file`, `sessionId`) remain as optional refinements *within* a brain. A coding-agent brain might want one briefing per branch. A research brain might want one per topic. These are passed as optional arguments to `get_playbook` and `regenerate_playbook`.

## The flow

When an agent does meaningful work, it saves a **note** — `remember()`. Memoria digests the note: it identifies the **things** mentioned, links them with **facts**, and stamps each fact with when it was true in the world (`tValid`) and when Memoria learned it (`tIngested`).

When a new session starts, the agent loads the **briefing** — `get_playbook()`. Memoria assembles the briefing from the current set of things and facts in this brain. The briefing is a couple of pages of Markdown, human-readable, always current.

If the agent needs to dig deeper than the briefing, it can `recall()` for specific facts, or `recall_history()` to see how knowledge about one thing has changed over time.

## A worked example: two crons, one Memoria account

Say you run two daily crons:

1. **Glyph maintenance** — scans androidx release notes and opens PRs.
2. **Memoria self-maintenance** — monitors Memoria's own changelog and opens issues.

These crons should have completely separate memory. What Glyph knows about `androidx.compose` is irrelevant to what Memoria self-maintenance knows about `@hono/zod-openapi`.

**Without brains (old model):** you used a single API key and tagged every note with `metadata.project = "glyph-maintenance"` or `metadata.project = "memoria-maintenance"`. But `recall()` searched the whole tenant — the wrong facts could leak across.

**With brains:**

1. You create two brains in the dashboard: "Glyph maintenance" and "Memoria self-maintenance."
2. Each brain gets its own API key.
3. The Glyph cron uses the Glyph brain's key. `remember`, `recall`, `get_playbook` all operate exclusively within that brain. No `project` argument needed — the brain is implicit from the key.
4. The Memoria self-maintenance cron uses the other brain's key. Same deal.
5. One Stripe bill covers both.

The Glyph cron at the start of a run:

```
mcp call memoria.get_playbook --args '{"branch": "main"}'
```

That's it. No `project: "glyph-maintenance"` — the brain handles the isolation.

At the end of the run:

```
mcp call memoria.remember --args '{
  "content": "Bumped androidx.compose.remote pin from 1.0.0-alpha010 to 1.0.0-alpha011. Opened PR #82.",
  "source": "agent",
  "metadata": {"sessionId": "cron-2026-05-27"}
}'
mcp call memoria.regenerate_playbook --args '{"branch": "main"}'
```

The Memoria self-maintenance cron does the same with its own key. Each cron sees only its own memory.

Want a third agent? Create a third brain, mint a third key. Same account, same bill, fully isolated.

## Two clocks

Most memory systems track one time: when a fact was written. Memoria tracks two:

- **`tValid`** — when the fact became true in the world. (e.g. "compose 1.7.2 shipped on 2026-05-27")
- **`tIngested`** — when Memoria learned it. (e.g. "agent saved this on 2026-05-28")

This is why you can ask "what did I think was true as of last Monday?" and get an honest answer, even after you've learned new things that contradict the old view. For an agent that runs daily, bumps its own version pins, and needs to reason about its own past correctness, this is the load-bearing feature.

## Sub-scopes (optional refinements within a brain)

Sometimes a single brain still benefits from fine-grained scoping for briefings. A coding-agent brain might want one briefing per branch (`branch: "feature-x"`) and another for the whole project (no sub-scope). The available sub-scope dimensions are `branch`, `file`, and `sessionId`.

Sub-scopes affect briefings only — `recall()` always searches the entire brain.

A briefing scope is "exact-match" — asking for `{branch: "main"}` will not return a briefing stored for `{branch: "main", file: "index.ts"}`. Pick a consistent scope shape per use case.

## Connecting an agent: API key vs OAuth

Every agent call to `/mcp` needs a `(tenantId, brainId)` binding so Memoria knows which brain to read from and write to. There are two ways to establish that binding:

**API key (manual):** you mint a `mem_live_*` token in the dashboard (`/dashboard/brains/<id>/keys`), copy it, and paste it into your MCP client config (`--header "Authorization: Bearer mem_live_..."`). Simple, works everywhere, requires no browser.

**OAuth 2.1 (recommended for new setups):** you register Memoria in your MCP client with `claude mcp add --transport http memoria <url>`, then run `/mcp` in Claude Code. A browser window opens; you sign in to `memoria.premex.se` (already authenticated via Google if you're on the dashboard), pick a brain from a consent screen, and click Authorize. Claude Code stores the resulting token in your OS keychain — no token to copy, no env var, no file. The binding between the MCP client and the brain is established once, then renewed automatically via refresh tokens.

Both paths arrive at exactly the same `(tenantId, brainId, scopes)` binding inside the API. Downstream routes and MCP tools are completely unaware of which path was used.

## Where to go next

- [scopes.md](scopes.md) — the permission model for API keys.
- The MCP tool reference exposed by your client — every tool description is written in the agent's language.
- [examples/glyph-cron-on-memoria.md](examples/glyph-cron-on-memoria.md) — a worked migration from a wiki-based daily cron to Memoria, updated to use a brain-bound API key.
