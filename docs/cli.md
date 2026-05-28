# memoria CLI reference

`memoria` is the command-line interface for [Memoria](https://memoria.premex.se) — bi-temporal AI agent memory served over MCP. It handles install + auth + MCP config in one shot, and deposits a Claude Code skill so every session in a configured project automatically reads Memoria's mental model.

---

## Install

```sh
curl -fsSL https://api.memoria.premex.se/install.sh | sh
```

The installer:

1. Resolves the latest release from GitHub.
2. Downloads the binary for your OS and architecture (`darwin/arm64`, `darwin/amd64`, `linux/arm64`, `linux/amd64`).
3. Writes it to `~/.local/bin/memoria` and marks it executable.

After install, make sure `~/.local/bin` is on your PATH:

```sh
# Add to ~/.zshrc or ~/.bashrc if it's not already there
export PATH="$HOME/.local/bin:$PATH"
```

Verify:

```sh
memoria --version
```

---

## Commands

### `memoria init <token>`

Connects a Memoria brain to your Claude Code sessions.

```sh
memoria init mem_live_<your-key>
```

**What it does:**

1. Validates the token by calling `GET /v1/whoami` against the Memoria API.
2. Prints the bound tenant and brain name.
3. Stores the token securely (see [Token resolution order](#token-resolution-order) below).
4. Writes the `mcpServers.memoria` entry into `~/.claude.json` using the `headersHelper` mechanism — the token is **never** stored in `~/.claude.json` in plaintext.
5. Deposits the `memoria` skill at `~/.claude/skills/memoria/SKILL.md`.

**Flags:**

| Flag | Default | Description |
|---|---|---|
| `--api-url URL` | `https://api.memoria.premex.se` | Override the API base URL (useful for self-hosted or staging) |

**Cloud environment setup:**

When `MEMORIA_API_KEY` is already set in the environment, `memoria init` skips the keychain write and records the install as "env-var mode." The `memoria headers` command reads from the env var at MCP connection time. This is the recommended pattern for CI and cloud development environments:

```sh
# Cloud setup script
curl -fsSL https://api.memoria.premex.se/install.sh | sh
memoria init "$MEMORIA_API_KEY"
```

---

### `memoria headers`

Resolves the active API token and prints an HTTP Authorization header as JSON. This command is invoked automatically by Claude Code's `headersHelper` mechanism at MCP connection time — you do not need to run it manually.

```sh
memoria headers
```

Output:

```json
{"Authorization":"Bearer mem_live_..."}
```

Resolution order: env var → macOS keychain → credentials file. See [Token resolution order](#token-resolution-order) below.

**Note:** This command must complete in under 10 seconds (Claude Code's `headersHelper` timeout). If the keychain is locked (e.g. a non-interactive shell on macOS), the command exits non-zero with a clear error message so the session log shows the cause.

---

### `memoria status`

Shows the current Memoria configuration and connection state.

```sh
memoria status
```

Output includes:

- Which brain the configured token belongs to (one round-trip to `/v1/whoami`)
- Where the token is stored (keychain / env var / credentials file)
- When the token was last rotated (from `~/.config/memoria/state.json`)
- Whether `~/.claude.json` has the `mcpServers.memoria` entry
- Whether `~/.claude/skills/memoria/SKILL.md` exists and matches the binary's embedded version
- CLI version and last update-check time

**Flags:**

| Flag | Default | Description |
|---|---|---|
| `--api-url URL` | `https://api.memoria.premex.se` | Override the API base URL |

---

### `memoria update`

Self-updates the CLI to the latest release.

```sh
memoria update
```

Resolves the latest GitHub Release, downloads the binary for the current OS and architecture, and atomically replaces `~/.local/bin/memoria` via a temp file + rename. The update is refused if the downloaded version is older than the current one.

**Flags:**

| Flag | Default | Description |
|---|---|---|
| `--check-only` | false | Print whether an update is available without downloading |
| `--source URL` | GitHub Releases API | Override the releases endpoint (for testing or mirrors) |

---

### `memoria --version` / `memoria -v`

Prints the CLI version.

```sh
memoria --version
# memoria version v0.1.0
```

---

## Token resolution order

`memoria headers` (and by extension every Claude Code session using the MCP server) resolves the token in this order:

1. **`$MEMORIA_API_KEY` environment variable** — if set, used immediately. No keychain or file access. Intended for CI, cloud environments, and containerized agents.
2. **macOS keychain** (darwin only) — `security find-generic-password -s memoria.premex.se -a $USER -w`. `memoria init` writes here on macOS when `$MEMORIA_API_KEY` is not set.
3. **`~/.config/memoria/credentials` file** — plaintext fallback for Linux and other environments without a system keychain. Written with `chmod 600`. `memoria init` warns when this fallback is used.
4. → **Error.** All three sources failed. Run `memoria init <token>` or set `$MEMORIA_API_KEY`.

---

## `~/.config/memoria/state.json`

A JSON file written by `memoria init` and read by `memoria status`. Records:

```json
{
  "tenantId": "tenant-abc",
  "brainId": "default",
  "brainName": "My brain",
  "installedAt": "2026-05-28T10:00:00Z",
  "lastTokenRotation": "2026-05-28T10:00:00Z",
  "lastUpdateCheck": "2026-05-28T10:00:00Z",
  "tokenSource": "keychain",
  "cliVersion": "v0.1.0"
}
```

**Do not hand-edit this file.** `memoria status` reads it for display; `memoria init` overwrites it. If it becomes corrupted, delete it and re-run `memoria init`.

---

## The `headersHelper` mechanism

Claude Code's `~/.claude.json` supports a `headersHelper` field on MCP server entries. When set, Claude Code runs the specified command at MCP connection time and merges the resulting JSON object into the request headers. This means the actual token never appears in `~/.claude.json` — it is resolved at connection time from whatever secure storage the CLI uses.

The entry written by `memoria init`:

```json
{
  "mcpServers": {
    "memoria": {
      "type": "http",
      "url": "https://api.memoria.premex.se/mcp",
      "headersHelper": "memoria headers"
    }
  }
}
```

If `memoria headers` exits non-zero (e.g. keychain locked, token missing), Claude Code surfaces the error in the session log. The fix is always `memoria init <token>` or ensuring `$MEMORIA_API_KEY` is set.

---

## See also

- [mental-model.md](mental-model.md) — what Memoria stores and why, in plain words
- [scopes.md](scopes.md) — API key scopes and which tools require which scope
- [examples/glyph-cron-on-memoria.md](examples/glyph-cron-on-memoria.md) — migrating a daily-maintenance cron from a wiki to Memoria
