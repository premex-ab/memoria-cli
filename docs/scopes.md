# API Key Scopes

Every Memoria API key carries one or more scopes. Scopes are enforced server-side on both the REST and MCP surfaces, which share the same scope vocabulary.

The scopes are:

| Scope            | Grants                                                                                          |
|------------------|-------------------------------------------------------------------------------------------------|
| `memory:read`    | Read episodes, entities, edges, playbooks; run recall.                                          |
| `memory:write`   | Write episodes (triggers extraction → entities + edges), create entities, create edges directly, regenerate / edit playbooks. |
| `graph:write`    | Mutate the bi-temporal graph after the fact: invalidate edges (`forget`), manually create relations (`relate`). |
| `admin`          | Bypasses all scope checks. Use for human-operated keys, never for autonomous agents.            |
| `offline_access` | **OAuth-only.** When included in an OAuth `/oauth/authorize` scope request, the `/oauth/token` endpoint returns a `refresh_token` alongside the access token. Clients must include this scope to enable long-lived sessions without re-authentication. Not valid as a standalone scope on `/mcp` — it is only meaningful during the OAuth consent flow. |

`offline_access` is only accepted at the OAuth `/oauth/authorize` endpoint (and stored in the resulting token's scope list). It is not enforced by `requireScope` on REST or MCP routes.

A key may hold any subset. `admin` is **not** a superset shorthand — the scope checker explicitly treats `admin` as a wildcard on both REST and MCP; for everything else, the requested scope must be present in the key's scope list.

There is **no `graph:read` scope**. Reading edges, entities, and recall results all gate on `memory:read`.

## Per-endpoint reference

The table below pairs each REST route with its MCP tool for quick comparison.

| Capability                       | REST                                          | Required scope  | MCP tool                  |
|----------------------------------|-----------------------------------------------|-----------------|---------------------------|
| Write an episode                 | `POST /v1/episodes`                           | `memory:write`  | `remember`                |
| Read an episode                  | `GET  /v1/episodes/:id`                       | `memory:read`   | _(not exposed)_           |
| Create an entity                 | `POST /v1/entities`                           | `memory:write`  | _(not exposed)_           |
| Read an entity                   | `GET  /v1/entities/:id`                       | `memory:read`   | _(not exposed)_           |
| Create an edge directly          | `POST /v1/edges`                              | `memory:write`  | _(see `relate` below)_    |
| Read an edge                     | `GET  /v1/edges/:id`                          | `memory:read`   | _(not exposed)_           |
| Invalidate an edge               | `PATCH /v1/edges/:id/invalidate`              | `graph:write`   | `forget`                  |
| Manually create a relation       | _(use `POST /v1/edges`)_                      | `graph:write`   | `relate`                  |
| Run recall                       | `POST /v1/recall`                             | `memory:read`   | `recall`                  |
| Regenerate a playbook            | `POST /v1/playbooks/regenerate`               | `memory:write`  | `regenerate_playbook`     |
| List playbooks                   | `GET  /v1/playbooks`                          | `memory:read`   | _(use `get_playbook`)_    |
| Read one playbook                | `GET  /v1/playbooks/:id`                      | `memory:read`   | `get_playbook`            |
| Edit a playbook                  | `PATCH /v1/playbooks/:id`                     | `memory:write`  | _(not exposed)_           |
| Record decision / convention / gotcha / session outcome | _(use `POST /v1/episodes`)_ | `memory:write`  | `record_decision`, `record_convention`, `record_gotcha`, `record_session_outcome` (all delegate to `remember`) |
| Recall (brain-scoped via key)    | `POST /v1/recall`                             | `memory:read`   | `recall`                  |

Note the asymmetry: `POST /v1/edges` (the REST verb that creates a brand-new edge from explicit entity IDs and a fact) gates on **`memory:write`**, because in the REST surface it is the symmetric "write a memory in graph form" operation. The MCP `relate` tool covers the same job for agents but requires **`graph:write`** because LLM-driven post-hoc graph stitching is treated as a graph-mutation primitive rather than a fresh memory. The edge **invalidation** path (`PATCH /v1/edges/:id/invalidate` and MCP `forget`) is graph-mutation on both sides and always requires `graph:write`.

## Worked examples

### "Agent only writes memories"

Minimum scopes: `memory:write`.

The agent can `POST /v1/episodes` (or call the MCP `remember` / `record_*` tools). Episodes trigger the full extraction pipeline, so entities and edges land in the KG automatically — without needing any `graph:write`. The agent cannot read its own writes back, cannot invalidate edges, and cannot manually stitch relations via `relate`.

Useful for batch ingestors and one-way feeds.

### "Agent reads + writes memories" (typical Claude Code workspace key)

Minimum scopes: `memory:read`, `memory:write`.

Covers `remember`, `recall`, `get_playbook`, `regenerate_playbook`, and all the `record_*` shortcuts. The brain is implicit from the key — no scope arguments needed for isolation. This is the recommended default for an autonomous coding agent.

### "Agent reads + writes graph" (curation agent / sleep-time consolidator)

Minimum scopes: `memory:read`, `memory:write`, `graph:write`.

Adds the ability to call `forget` (invalidate stale edges) and `relate` (manually post a fact between two entities the agent looked up via recall). Use this scope set for higher-trust agents that prune or reshape the graph.

### "Human key for an operator"

Scopes: `admin`.

`admin` bypasses every other scope check, including future ones. Reserve for dashboard maintenance and incident response — never hand to an agent.

## Errors

A missing or unknown bearer token returns `401 missing_auth` / `401 invalid_auth` / `401 invalid_key`. A valid key without the required scope returns `403 forbidden` with the body `{"error": "forbidden", "message": "missing required scope: <scope>"}`. MCP tool calls surface the same condition as an `isError: true` tool result with text `missing required scope: <scope>`.

## Provisioning

Keys are minted via the dashboard at `/dashboard/brains/[brainId]/keys` (Firebase-auth route, dashboard `admin` role required). The selectable scope set is exactly the four listed above; the plain key is returned **once** at create time.

---

## Playbook scopes (separate concept)

The word "scope" is overloaded in Memoria: API-key scopes (above) gate **what** an agent can do, while **playbook scopes** narrow **which subset of the KG** a playbook covers. The two are independent — both apply to a single `regenerate_playbook` call.

A `PlaybookScope` is an object with up to three optional sub-scope dimensions. The brain (isolation unit) is separate from the playbook scope — it comes from the API key, not from scope arguments.

| Dim         | Episode field matched                              |
|-------------|----------------------------------------------------|
| `branch`    | `metadata.branch`                                  |
| `file`      | `metadata.file`                                    |
| `sessionId` | top-level `sessionId` (not under `metadata`)       |

The `repo` and `project` dims were removed in the brains refactor — brain identity replaces them. A key bound to brain `glyph-maintenance` implicitly scopes every recall/remember/playbook call to that brain; there is no need for a `project` arg.

### Semantics

- **At least one** dim must be set on every regenerate call. The pipeline throws (`at least one scope dimension is required`); REST returns `400 invalid_query`; MCP returns the error as a tool result.
- **Conjunctive (AND)** across dims: episodes are included only when **every** set field equals the episode's corresponding value. `{ branch: 'feature-x', file: 'index.ts' }` picks episodes with BOTH `metadata.branch == 'feature-x'` AND `metadata.file == 'index.ts'`.
- **No partial-match** on lookup: `get_playbook` finds the most recent stored playbook whose scope is an **exact match** — every set dim equal AND no extra set dims. `{ branch: b }` does NOT match a stored `{ branch: b, file: f }` and vice versa.

### Surfaces

- REST: `POST /v1/playbooks/regenerate?scope.branch=...&scope.file=...` (any combination of the three `scope.<dim>` query params).
- MCP: `regenerate_playbook` and `get_playbook` tools accept `{ branch?, file?, sessionId? }` directly.

### Indexing

Each dim has a single-field index in `firestore.indexes.json` (`fieldOverrides` for `metadata.branch`, `metadata.file`; `sessionId` is already a top-level field covered by an existing composite index). Multi-dim AND-of-equality queries do not need an additional composite index — they execute as a zigzag merge join over the single-field indexes.
