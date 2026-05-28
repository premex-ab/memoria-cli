# Example: migrating a daily-maintenance cron from a wiki to Memoria

This is a worked example of an autonomous-maintenance cron that uses Memoria for cross-session memory instead of a markdown wiki. The original wiki-based version lived at [premex-ab/remote-compose/wiki](https://github.com/premex-ab/remote-compose/wiki). This is the migrated form.

The shape is unchanged: one daily run, agent loads what's already known, investigates in parallel, opens PRs for findings that pass the bar, saves notes about what happened, refreshes the briefing for next time. Only the memory surface changes.

## Setup: create a brain for this cron

Before updating the prompt, go to the Memoria dashboard at `/dashboard/brains` and create a brain named "Glyph maintenance." Copy the API key that is minted for it. That key is the only credential this cron needs.

The brain is the isolation unit — everything this cron saves is visible only to keys bound to this brain. You can create a second brain later (say, "Memoria self-maintenance") and give that cron its own key, and the two will never see each other's memory. One Memoria account, same bill, fully isolated.

## What changes

The wiki pattern was:

1. **Start of run:** `git clone <wiki-repo>` then `cat Claude-Memory.md`.
2. **End of run:** edit the markdown file by hand, `git commit && git push`.

The Memoria pattern is:

1. **Start of run:** call `get_playbook` — the brain is implicit from the API key.
2. **End of run:** call `remember` for each significant finding, then `regenerate_playbook` to refresh the briefing.

Everything else in the prompt (sub-agents, hard rules, workflow steps, PR requirements, priority order) stays the same. The diff is concentrated in the "Memory" section of the prompt and two of the workflow steps.

## The new Memory section

Replace the wiki block in the cron prompt with this:

> ## Memory — Memoria
>
> Persistent memory is stored in Memoria. The brain for this cron is implicit from the API key — no project or scope argument needed.
>
> At the start of every run, load the briefing:
>
> ```
> mcp call memoria.get_playbook --args '{"branch": "main"}'
> ```
>
> The briefing summarizes what's known so far — current state (versions, bugs, workarounds, competitors), recent activity (past runs and their PRs), open work (PRs still in flight), past attempts (closed-without-merge PRs with reasons).
>
> While working, if you need to dig deeper than the briefing offers, use:
>
> - `recall` — find facts relevant to a specific question.
> - `recall_history` — see how knowledge about one specific thing has changed over time. Useful for "what was the state of bug 508869338 last week?"
>
> At the end of every run, save one note per significant finding:
>
> ```
> mcp call memoria.remember --args '{
>   "content": "Bumped androidx.compose.remote pin from 1.0.0-alpha010 to 1.0.0-alpha011. Workaround for issuetracker 508869338 (TextMerge re-eval) dropped. Opened PR #82.",
>   "source": "agent",
>   "metadata": {"sessionId": "cron-2026-05-27"}
> }'
> ```
>
> Save notes for: each PR opened (URL, branch, one-line rationale), each material finding that was skipped (with reason), each upstream change observed (new alpha, bug status change, competitor move). Don't save mid-run trivia — only what tomorrow's agent would benefit from knowing.
>
> After all notes for the run are saved, regenerate the briefing:
>
> ```
> mcp call memoria.regenerate_playbook --args '{"branch": "main"}'
> ```
>
> If the run produces no PRs and no material findings, still save one short note ("no SHIP-grade findings across X vectors, alpha011 still latest, bugs unchanged") and regenerate — the audit trail matters.

## Workflow step edits

The wiki version's workflow step 1 said:

> 1. Clone the wiki and read `/tmp/wiki/Claude-Memory.md` (Known state + Run log + PR graveyard).

Replace with:

> 1. Load the briefing: `mcp call memoria.get_playbook --args '{"branch": "main"}'`. The briefing has current state, recent activity, open work, and past attempts.

The wiki version's workflow step 6 said:

> 6. **Update the wiki.** After all PRs are opened (or skip is decided), update `/tmp/wiki/Claude-Memory.md` with new Run-log lines and any 'Known state' deltas. Commit + push to the wiki repo.

Replace with:

> 6. **Save notes.** After all PRs are opened (or skip is decided), call `mcp call memoria.remember` for each significant finding (PR opened, upstream change observed, finding that was skipped with reason). Then call `mcp call memoria.regenerate_playbook --args '{"branch": "main"}'` to refresh the briefing for next time.

Also remove this hard rule from the wiki version (no longer applicable):

> Not include any edits to `CRON.md` (it no longer exists) or any commit that touches the wiki (the wiki is its own repo).

## Migration notes

- **Don't bulk-import the existing wiki.** Let the next few cron runs accumulate fresh notes. The 116 KB wiki you have stays as a frozen read-only snapshot for audit. Bulk-imports go through the extraction pipeline and would (a) cost money and (b) probably produce worse synthesis than letting fresh runs build up.
- **The brain handles isolation.** There is no `project: "glyph-maintenance"` metadata argument any more — the brain is implicit from the API key. Sub-scope arguments (`branch`, `file`, `sessionId`) remain available for briefing granularity within the brain.
- **Cost.** Each `remember()` runs the 7-stage extraction pipeline — roughly $0.10–0.30 per call at current Anthropic pricing. A typical run with 5–10 saved findings costs $1–3. For a daily cron, that's fine.
- **First run produces a short briefing.** Expected — there's no prior memory to summarize. Fills in over the next 3–5 runs.
- **Verify the briefing covers what you need before retiring the wiki.** Run both in parallel for a week. If something material is missing from the briefing, adjust what the agent saves at end-of-run; don't change the briefing prompt itself unless you've tried adjusting inputs first.
- **The agent never edits a memory file by hand.** The briefing is synthesized from facts. If the briefing is wrong, the fix is to save better notes (or invalidate stale facts with `forget`) — not to hand-edit anything.
- **Want a separate brain for another agent (e.g. Memoria self-maintenance)?** Create a second brain in the dashboard at `/dashboard/brains`, mint a key for it, and that cron is fully isolated from this one. Same Memoria account, same bill. Each cron's `recall`, `remember`, `get_playbook`, and `regenerate_playbook` calls only ever touch the brain bound to its key.

## Sanity check before flipping

Before pointing the cron at Memoria, run this once manually with a fresh brain:

1. Save a few seed notes via `remember` covering current Known state (latest alpha, open bugs, key workarounds).
2. Call `regenerate_playbook` for the scope.
3. Call `get_playbook` and read the result.
4. Does it look like a brief, useful version of your wiki's Known state? If yes, flip the cron. If it's too short or missing structure, that's an actionable signal — file an issue against the playbook prompt rather than blocking the migration.
