package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/premex-ab/memoria-cli/internal/api"
	"github.com/premex-ab/memoria-cli/internal/auth"
	"github.com/premex-ab/memoria-cli/internal/config"
	"github.com/premex-ab/memoria-cli/internal/skill"
	"github.com/premex-ab/memoria-cli/internal/version"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	var apiURL string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show the current Memoria configuration and connection status",
		Long: `memoria status prints a human-readable summary of the active configuration:
which brain the configured token belongs to, where the token is stored,
whether ~/.claude.json has the MCP entry, and whether the skill is installed.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			stdout := cmd.OutOrStdout()

			// Resolve the token. On ErrNoToken we still print CLI info but skip
			// the remote checks.
			token, src, err := auth.Resolve()
			if errors.Is(err, auth.ErrNoToken) {
				fmt.Fprintln(stdout, "Token:    not configured — run `memoria init <token>`")
				printCLILine(cmd)
				// ErrNoToken is informational — the command itself succeeded.
				return nil
			}
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "memoria: resolve token: %v\n", err)
				return err
			}

			// Token is present — print its source.
			fmt.Fprintf(stdout, "Token:    %s\n", src.String())

			// Call /v1/whoami to get brain + tenant + scopes.
			client := api.New(apiURL)
			whoami, whoamiErr := client.Whoami(ctx, token)
			if whoamiErr != nil {
				if errors.Is(whoamiErr, api.ErrInvalidToken) {
					fmt.Fprintln(stdout, "Brain:    unable to resolve — token may be revoked.")
				} else {
					fmt.Fprintf(stdout, "Brain:    unable to resolve — %v\n", whoamiErr)
				}
			} else {
				scopes := strings.Join(whoami.Scopes, ",")
				fmt.Fprintf(stdout, "Brain:    %s/%s  (scopes: %s)\n",
					whoami.TenantID, whoami.BrainID, scopes)
			}

			// MCP: confirm entry in ~/.claude.json, and display the stored URL.
			storedMCPURL, mcpErr := resolveMCPURL()
			if mcpErr != nil {
				fmt.Fprintln(stdout, "MCP:      NOT configured in ~/.claude.json — re-run `memoria init`")
			} else {
				fmt.Fprintf(stdout, "MCP:      configured at %s\n", storedMCPURL)
			}

			// Skill: read the installed SKILL.md frontmatter version.
			home, homeErr := os.UserHomeDir()
			if homeErr == nil {
				skillPath := filepath.Join(home, ".claude", "skills", "memoria", "SKILL.md")
				installedVer, skillErr := skill.InstalledVersion(home)
				if errors.Is(skillErr, os.ErrNotExist) {
					fmt.Fprintln(stdout, "Skill:    NOT installed — re-run `memoria init`")
				} else if skillErr != nil {
					fmt.Fprintf(stdout, "Skill:    %s (unable to read version: %v)\n", skillPath, skillErr)
				} else {
					embVer, _ := skill.EmbeddedVersion()
					if embVer != "" && embVer != installedVer {
						fmt.Fprintf(stdout, "Skill:    %s (installed %s, binary embeds %s — run `memoria init` to refresh)\n",
							skillPath, installedVer, embVer)
					} else {
						fmt.Fprintf(stdout, "Skill:    %s (version %s)\n", skillPath, installedVer)
					}
				}
			}

			// CLI version + last checked.
			printCLILine(cmd)
			return nil
		},
	}

	cmd.Flags().StringVar(&apiURL, "api-url", "https://api.memoria.premex.se",
		"Memoria API base URL (override for testing)")

	return cmd
}

// printCLILine writes the "CLI: <version> — last checked…" line to cmd's stdout.
func printCLILine(cmd *cobra.Command) {
	stdout := cmd.OutOrStdout()
	state, _ := config.Read()
	lastChecked := "never"
	if !state.LastChecked.IsZero() {
		lastChecked = relativeTime(state.LastChecked)
	}
	fmt.Fprintf(stdout, "CLI:      %s — last checked for updates %s\n",
		version.Version, lastChecked)
}

// relativeTime returns a human-friendly description of how long ago t was.
func relativeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		mins := int(d.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	case d < 24*time.Hour:
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
}

// resolveMCPURL reads ~/.claude.json and returns the configured memoria MCP URL,
// or an error if the entry is absent.
func resolveMCPURL() (string, error) {
	claudePath, err := config.ClaudeJSONPath()
	if err != nil {
		return "", err
	}

	raw, err := os.ReadFile(claudePath)
	if err != nil {
		return "", fmt.Errorf("~/.claude.json: %w", err)
	}

	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return "", fmt.Errorf("~/.claude.json malformed: %w", err)
	}

	servers, ok := doc["mcpServers"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("mcpServers not found")
	}
	mem, ok := servers["memoria"]
	if !ok {
		return "", fmt.Errorf("memoria server not found")
	}
	memMap, ok := mem.(map[string]any)
	if !ok {
		return "", fmt.Errorf("memoria entry malformed")
	}
	u, _ := memMap["url"].(string)
	return u, nil
}
