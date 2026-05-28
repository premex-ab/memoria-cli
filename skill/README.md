# `skill/`

The canonical `SKILL.md` lives at `internal/skill/SKILL.md` so that
`//go:embed` (which does not support `..` paths) can pull it into the
binary at build time. This directory is intentionally a stub — kept for
the historical layout documented in the spec.
