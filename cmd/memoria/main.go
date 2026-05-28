package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/premex-ab/memoria-cli/internal/auth"
	"github.com/premex-ab/memoria-cli/internal/version"
	"github.com/spf13/cobra"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "memoria",
		Short: "Memoria CLI — manage your AI memory from the command line",
		Long: `memoria is the command-line interface for Memoria (https://memoria.premex.se).

Use 'memoria init <token>' to connect a brain to your Claude Code sessions,
and 'memoria headers' to resolve the active API token as an HTTP header.`,
		// Don't print usage on error — errors are usually auth failures, not syntax errors.
		SilenceUsage: true,
		// Don't let cobra print the error again — each subcommand's RunE prints its own
		// styled message to stderr; we just need cobra to drive the non-zero exit.
		SilenceErrors: true,
	}

	root.Version = version.Version

	root.AddCommand(newHeadersCmd())

	return root
}

func newHeadersCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "headers",
		Short: "Print the Authorization header for the active API token (used by Claude Code's headersHelper)",
		Long: `memoria headers resolves the active Memoria API token and prints
a JSON object suitable for use as MCP connection headers.

Resolution order:
  1. $MEMORIA_API_KEY environment variable
  2. macOS keychain (darwin only)
  3. ~/.config/memoria/credentials file fallback

This command is invoked automatically by Claude Code at MCP connection time
via the headersHelper mechanism. It must complete in under 10 seconds.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			token, _, err := auth.Resolve()
			if err != nil {
				fmt.Fprintln(os.Stderr, "memoria: no API token found.", err.Error())
				fmt.Fprintln(os.Stderr, "Run 'memoria init <token>' to configure, or set $MEMORIA_API_KEY.")
				return err
			}

			headers := map[string]string{
				"Authorization": "Bearer " + token,
			}
			out, err := json.Marshal(headers)
			if err != nil {
				return fmt.Errorf("failed to encode headers: %w", err)
			}
			fmt.Println(string(out))
			return nil
		},
	}
}
