# memoria

Single-binary CLI for [Memoria](https://memoria.premex.se) — bi-temporal AI agent memory served over MCP.

The CLI handles install + auth + MCP config in one shot, and deposits a Claude Code skill so every session in a configured project automatically reads Memoria's mental model.

## Install

```sh
curl -fsSL https://api.memoria.premex.se/install.sh | sh
memoria init <your-api-key>
```

Mint an API key at https://memoria.premex.se/dashboard/brains/&lt;brain-id&gt;/keys. The CLI stores it in your OS keychain (macOS) or a permission-restricted file (Linux), wires up `~/.claude.json`, and deposits the `memoria` skill at `~/.claude/skills/memoria/SKILL.md`. Subsequent Claude Code sessions pick it all up automatically.

## What this repo contains

- The Go source for the `memoria` CLI (`cmd/`, `internal/`).
- The in-session Claude Code skill (`skill/SKILL.md`) embedded into the binary at build time.
- The bootstrap Claude Code plugin (`plugin/memoria-setup/`) that installs the CLI on demand.
- The installer shell script (`install.sh`).
- User-facing docs (`docs/`): CLI reference, mental model, scopes reference, and worked examples.

The Memoria service itself (api, web app, dashboard) lives in a separate private repo. This repo is the user-facing client + docs surface.

## Docs

- [docs/cli.md](docs/cli.md) — full CLI reference: all commands, flags, token resolution, `headersHelper`
- [docs/mental-model.md](docs/mental-model.md) — what Memoria stores and why, in plain words
- [docs/scopes.md](docs/scopes.md) — API key scopes and per-endpoint access control
- [docs/examples/glyph-cron-on-memoria.md](docs/examples/glyph-cron-on-memoria.md) — migrating a daily-maintenance cron from a wiki to Memoria

## License

Apache-2.0. See [LICENSE](LICENSE).
