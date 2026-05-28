package main

import (
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
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	var apiURL string

	cmd := &cobra.Command{
		Use:   "init <token>",
		Short: "Bind a Memoria API token to your Claude Code sessions",
		Long: `memoria init validates the supplied token against the Memoria API,
stores it securely, writes the MCP entry into ~/.claude.json, and records
the bound tenant and brain in ~/.config/memoria/state.json.

Resolution for token storage:
  1. If $MEMORIA_API_KEY is already set, env-var mode is used (no persistent write).
  2. On macOS, the token is stored in the system keychain.
  3. Otherwise, the token is written to ~/.config/memoria/credentials (0600).

The MCP entry added to ~/.claude.json uses Claude Code's headersHelper
mechanism — the token itself is never stored in ~/.claude.json.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			token := args[0]
			ctx := cmd.Context()

			stdout := cmd.OutOrStdout()
			stderr := cmd.ErrOrStderr()

			// 1. Validate the token by calling GET /v1/whoami.
			client := api.New(apiURL)
			whoami, err := client.Whoami(ctx, token)
			if err != nil {
				if errors.Is(err, api.ErrInvalidToken) {
					fmt.Fprintln(stderr,
						"memoria: token rejected by server — check that you minted it at",
						"https://memoria.premex.se/dashboard and didn't revoke it.",
					)
					return err
				}
				fmt.Fprintf(stderr, "memoria: whoami request failed: %v\n", err)
				return err
			}

			// 2. Print identity so the user can confirm what they bound to.
			scopes := strings.Join(whoami.Scopes, ",")
			fmt.Fprintf(stdout, "Bound to tenant=%s brain=%s scopes=%s\n",
				whoami.TenantID, whoami.BrainID, scopes)

			// 3. Store the token.
			source, err := auth.Store(token)
			if err != nil {
				fmt.Fprintf(stderr, "memoria: failed to store token: %v\n", err)
				return err
			}

			// 4. Write the MCP entry into ~/.claude.json.
			claudePath, err := config.ClaudeJSONPath()
			if err != nil {
				fmt.Fprintf(stderr, "memoria: cannot find claude.json path: %v\n", err)
				return err
			}
			entry := config.McpEntry{
				Type:          "http",
				URL:           apiURL + "/mcp",
				HeadersHelper: "memoria headers",
			}
			if err := config.WriteMemoriaEntry(claudePath, entry); err != nil {
				fmt.Fprintf(stderr, "memoria: failed to write ~/.claude.json: %v\n", err)
				fmt.Fprintln(stderr, "memoria: the token was stored successfully — re-run 'memoria init <token>' to retry the MCP config step.")
				return err
			}

			// 5. Install or update the embedded skill.
			home, homeErr := os.UserHomeDir()
			if homeErr != nil {
				fmt.Fprintf(stderr, "memoria: warning: cannot determine home dir for skill install: %v\n", homeErr)
			} else {
				skillOutcome, skillErr := skill.Install(home)
				if skillErr != nil {
					// Non-fatal — MCP entry is already written.
					fmt.Fprintf(stderr, "memoria: warning: skill install failed: %v\n", skillErr)
				} else {
					skillPath := filepath.Join(home, ".claude", "skills", "memoria", "SKILL.md")
					embVer, _ := skill.EmbeddedVersion()
					switch skillOutcome {
					case skill.OutcomeInstalled:
						fmt.Fprintf(stdout, "Installed memoria skill at %s.\n", skillPath)
					case skill.OutcomeUpdated:
						fmt.Fprintf(stdout, "Updated memoria skill (→ %s).\n", embVer)
					case skill.OutcomeSkipped:
						fmt.Fprintln(stdout, "Skill already up to date.")
					case skill.OutcomeOverwrittenUnparseable:
						fmt.Fprintln(stdout, "Replaced unparseable existing skill.")
					}
				}
			}

			// 6. Update state. (step numbering continues from 5 above)
			state, err := config.Read()
			if err != nil {
				// Non-fatal: start from a zero state.
				state = config.State{}
			}
			now := time.Now().UTC()
			if state.InstalledAt.IsZero() {
				state.InstalledAt = now
			}
			state.LastUpdated = now
			state.BoundTenant = whoami.TenantID
			state.BoundBrain = whoami.BrainID
			state.TokenSource = source.String()

			if err := config.Write(state); err != nil {
				// Warn but don't fail — the MCP entry and token are already written.
				fmt.Fprintf(stderr, "memoria: warning: failed to update state file: %v\n", err)
			}

			// 7. Done.
			fmt.Fprintln(stdout, "Done. Open a Claude Code session and try recall('hello world').")
			return nil
		},
	}

	cmd.Flags().StringVar(&apiURL, "api-url", "https://api.memoria.premex.se",
		"Memoria API base URL (override for testing or self-hosted instances)")

	return cmd
}
