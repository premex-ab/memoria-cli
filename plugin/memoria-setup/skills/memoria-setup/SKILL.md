---
name: memoria-setup
description: Use when the user wants to set up or configure Memoria (https://memoria.premex.se) in this Claude Code session. Activates on prompts like "set up memoria", "configure memoria mcp", "install memoria", or when the user references brains/episodes/playbooks in a project that doesn't have the memoria binary installed.
version: 0.1.0
---

# memoria-setup skill

This skill bootstraps the `memoria` CLI and MCP server in the current Claude Code session. It checks whether the CLI is installed, installs it if needed, and walks the user through `memoria init <token>` so subsequent sessions have the MCP server available automatically.

## What this skill does

1. Detects whether `memoria` is on PATH.
2. If not, runs the one-line installer.
3. Asks the user for their Memoria API key.
4. Runs `memoria init <token>` to wire up `~/.claude.json` and deposit the in-session skill.
5. Instructs the user to `/restart` Claude Code so the new MCP entry is picked up.

Do NOT bake any tokens into this file. The skill is a script of instructions, not a token store.

---

## Step 1: check whether the CLI is installed

Run this in the bash tool:

```bash
command -v memoria || echo not-installed
```

- If a path is printed → the CLI is installed. Skip to **Step 3**.
- If `not-installed` is printed → proceed to **Step 2**.

---

## Step 2: install the CLI

Run the installer:

```bash
curl -fsSL https://api.memoria.premex.se/install.sh | sh
```

Print the output verbatim so the user can see what was installed and where.

After the installer completes, confirm `~/.local/bin` is on the user's PATH:

```bash
echo "$PATH" | tr ':' '\n' | grep -q "$HOME/.local/bin" && echo "PATH ok" || echo "PATH missing"
```

If `PATH missing` is printed, instruct the user to add the following line to their shell rc file (`~/.zshrc`, `~/.bashrc`, etc.) and then reload the shell:

```sh
export PATH="$HOME/.local/bin:$PATH"
```

After updating the PATH, verify:

```bash
command -v memoria && memoria --version
```

---

## Step 3: ask the user for an API key

Tell the user:

> To connect this Claude Code session to a Memoria brain, you need an API key.
>
> 1. Open https://memoria.premex.se/dashboard in your browser.
> 2. Pick the brain you want this session to use (or create one).
> 3. Go to **Keys** for that brain and mint a new key — give it scopes `memory:read` and `memory:write` for normal agent use.
> 4. Copy the key (it starts with `mem_live_`) — it is shown only once.
>
> Paste the key here when you're ready. (Do not put the key in any file — paste it directly in the chat.)

Wait for the user to provide the key. Treat the key as sensitive: do not echo it back, do not log it, and do not include it in any file content you write.

---

## Step 4: run `memoria init <token>`

Once the user has pasted the key, run — replacing `<token>` with the value they provided:

```bash
memoria init <token>
```

On success, `memoria init` will:
- Validate the token against `https://api.memoria.premex.se`
- Print the bound brain name and tenant ID
- Store the token in your OS keychain (macOS) or a permission-restricted file (Linux)
- Write the MCP entry into `~/.claude.json` using the `headersHelper` mechanism — the token is **never** stored in that file in plaintext
- Deposit the `memoria` skill at `~/.claude/skills/memoria/SKILL.md`

Confirm success:

```bash
memoria status
```

Expected output includes the brain name, token storage location, and a confirmation that `~/.claude.json` has the memoria MCP entry.

---

## Step 5: restart Claude Code

Tell the user:

> `memoria init` is complete. To activate the MCP server and the memoria skill in this session, run `/restart` now.
>
> After restart, the `mcp__memoria__*` tools will be available and the memoria skill will load automatically.

---

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| `"token rejected"` or `401` from `memoria init` | Token was revoked, is for a different environment, or has a typo | Re-mint the key at the dashboard and try again |
| `command not found: memoria` after install | `~/.local/bin` is not in `$PATH` | Add `export PATH="$HOME/.local/bin:$PATH"` to your shell rc and reload |
| `~/.claude.json malformed` error | An earlier edit left the JSON invalid | Run `cat ~/.claude.json` and fix the syntax, or back it up and re-run `memoria init` |
| MCP not showing after `/restart` | `headersHelper` command failed | Run `memoria headers` manually — if it errors, run `memoria status` to diagnose |
| `secret-tool: command not found` on Linux | `libsecret-tools` not installed | `sudo apt install libsecret-tools` or accept the file-based fallback (Memoria will warn you) |
