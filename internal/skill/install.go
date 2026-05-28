// Package skill embeds the Memoria Claude Code skill and deposits it into the
// user's skills directory at ~/.claude/skills/memoria/SKILL.md.
package skill

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

//go:embed SKILL.md
var EmbeddedSKILL []byte

// ErrMalformedFrontmatter is returned when SKILL.md does not start with a
// valid YAML frontmatter block (---\n…\n---\n).
var ErrMalformedFrontmatter = errors.New("malformed frontmatter: expected --- delimiters")

// Outcome describes what Install did.
type Outcome int

const (
	// OutcomeInstalled means the skill file was written for the first time.
	OutcomeInstalled Outcome = iota
	// OutcomeUpdated means the embedded version was strictly newer than the
	// existing file; the file was overwritten.
	OutcomeUpdated
	// OutcomeSkipped means the existing file version was >= the embedded
	// version; the file was left unchanged.
	OutcomeSkipped
	// OutcomeOverwrittenUnparseable means the existing file had no valid
	// frontmatter; it was overwritten with a warning printed to stderr.
	OutcomeOverwrittenUnparseable
)

// String returns a human-readable name for the outcome.
func (o Outcome) String() string {
	switch o {
	case OutcomeInstalled:
		return "installed"
	case OutcomeUpdated:
		return "updated"
	case OutcomeSkipped:
		return "skipped"
	case OutcomeOverwrittenUnparseable:
		return "overwritten-unparseable"
	default:
		return fmt.Sprintf("Outcome(%d)", int(o))
	}
}

// EmbeddedVersion returns the version declared in the embedded SKILL.md
// frontmatter. Returns ("", ErrMalformedFrontmatter) if the frontmatter is
// missing or malformed.
func EmbeddedVersion() (string, error) {
	return parseFrontmatterVersion(EmbeddedSKILL)
}

// Install deposits the embedded skill at homeDir/.claude/skills/memoria/SKILL.md.
//
// Behaviour:
//   - If the file does not exist, write it (OutcomeInstalled).
//   - If the file exists, parse its frontmatter version; if the embedded
//     version is strictly newer (semver compare), overwrite (OutcomeUpdated).
//   - If the existing version is >= embedded, leave it alone (OutcomeSkipped).
//   - If the existing file is unparseable, log a warning to stderr and
//     overwrite (OutcomeOverwrittenUnparseable).
//
// The write is atomic: tempfile → chmod 0600 → rename. Parent dirs are
// created with 0700 permissions.
func Install(homeDir string) (Outcome, error) {
	destDir := filepath.Join(homeDir, ".claude", "skills", "memoria")
	destPath := filepath.Join(destDir, "SKILL.md")

	embeddedVer, err := EmbeddedVersion()
	if err != nil {
		return OutcomeInstalled, fmt.Errorf("skill: embedded SKILL.md is malformed: %w", err)
	}

	// Check whether the file already exists.
	existing, err := os.ReadFile(destPath)
	if errors.Is(err, os.ErrNotExist) {
		// First install — just write it.
		if err := atomicWrite(destDir, destPath, EmbeddedSKILL); err != nil {
			return OutcomeInstalled, err
		}
		return OutcomeInstalled, nil
	}
	if err != nil {
		return OutcomeInstalled, fmt.Errorf("skill: read existing skill file: %w", err)
	}

	// File exists — check the version.
	existingVer, err := parseFrontmatterVersion(existing)
	if err != nil {
		// Existing file is unparseable — warn and overwrite.
		fmt.Fprintf(os.Stderr, "memoria: warning: existing skill file has no valid frontmatter; replacing it\n")
		if err := atomicWrite(destDir, destPath, EmbeddedSKILL); err != nil {
			return OutcomeOverwrittenUnparseable, err
		}
		return OutcomeOverwrittenUnparseable, nil
	}

	cmp := compareSemver(embeddedVer, existingVer)
	if cmp <= 0 {
		// Existing is same or newer — leave it alone.
		return OutcomeSkipped, nil
	}

	// Embedded is newer — overwrite.
	if err := atomicWrite(destDir, destPath, EmbeddedSKILL); err != nil {
		return OutcomeUpdated, err
	}
	return OutcomeUpdated, nil
}

// atomicWrite writes data to path via a temp file + rename. Creates parent
// dirs with 0700. The destination file gets 0600 permissions.
func atomicWrite(dir, path string, data []byte) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("skill: create skill dir: %w", err)
	}

	tmp, err := os.CreateTemp(dir, "SKILL.md.*.tmp")
	if err != nil {
		return fmt.Errorf("skill: create temp file: %w", err)
	}
	tmpName := tmp.Name()
	committed := false
	defer func() {
		if !committed {
			os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("skill: write temp file: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("skill: chmod temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("skill: close temp file: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("skill: rename skill file: %w", err)
	}
	committed = true
	return nil
}

// parseFrontmatterVersion parses a SKILL.md-style YAML frontmatter block and
// returns the value of the "version" key.
//
// Expected format:
//
//	---\n
//	key: value\n
//	…\n
//	---\n
//	…body…
//
// Returns ErrMalformedFrontmatter if the block is absent or malformed.
func parseFrontmatterVersion(data []byte) (string, error) {
	s := string(data)

	const open = "---\n"
	if !strings.HasPrefix(s, open) {
		return "", ErrMalformedFrontmatter
	}
	rest := s[len(open):]

	const close = "\n---\n"
	idx := strings.Index(rest, close)
	if idx < 0 {
		return "", ErrMalformedFrontmatter
	}
	block := rest[:idx]

	fields := parseFrontmatterBlock(block)
	ver, ok := fields["version"]
	if !ok || ver == "" {
		return "", fmt.Errorf("%w: no version field", ErrMalformedFrontmatter)
	}
	return ver, nil
}

// parseFrontmatterBlock parses a YAML block (without the --- delimiters) into
// a map[string]string. Only handles simple "key: value" lines; multi-line
// values and nested structures are not supported (and not needed here).
func parseFrontmatterBlock(block string) map[string]string {
	out := make(map[string]string)
	for _, line := range strings.Split(block, "\n") {
		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		if key != "" {
			out[key] = val
		}
	}
	return out
}

// compareSemver compares two MAJOR.MINOR.PATCH version strings.
// Returns -1 if a < b, 0 if a == b, +1 if a > b.
// Treats any parse error as version 0.0.0.
func compareSemver(a, b string) int {
	ma, mia, pa := parseSemver(a)
	mb, mib, pb := parseSemver(b)

	if ma != mb {
		return cmpInt(ma, mb)
	}
	if mia != mib {
		return cmpInt(mia, mib)
	}
	return cmpInt(pa, pb)
}

// parseSemver splits "MAJOR.MINOR.PATCH" into three ints. Returns (0,0,0) on
// any parse error.
func parseSemver(v string) (major, minor, patch int) {
	parts := strings.SplitN(v, ".", 3)
	if len(parts) != 3 {
		return 0, 0, 0
	}
	major, _ = strconv.Atoi(parts[0])
	minor, _ = strconv.Atoi(parts[1])
	patch, _ = strconv.Atoi(parts[2])
	return major, minor, patch
}

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}
