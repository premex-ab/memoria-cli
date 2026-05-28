package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/premex-ab/memoria-cli/internal/config"
	"github.com/premex-ab/memoria-cli/internal/version"
	"github.com/spf13/cobra"
)

// executablePath is a variable so tests can override it.
var executablePath = os.Executable

// maxBinarySize is the maximum number of bytes we will read from a binary
// download response. The real memoria binary is well under 20 MB; 100 MB is a
// generous ceiling that still protects against a runaway or malicious server
// filling disk / exhausting memory.
const maxBinarySize = 100 << 20 // 100 MB

// defaultReleasesURL is the default GitHub releases latest endpoint.
const defaultReleasesURL = "https://api.github.com/repos/premex-ab/memoria-cli/releases/latest"

// releaseAsset is one entry in the assets array of a GitHub release.
type releaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// releaseInfo is the parsed body from the GitHub releases/latest endpoint.
type releaseInfo struct {
	TagName string         `json:"tag_name"`
	Assets  []releaseAsset `json:"assets"`
}

func newUpdateCmd() *cobra.Command {
	var checkOnly bool
	var sourceURL string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Self-update the memoria CLI to the latest release",
		Long: `memoria update fetches the latest GitHub release, downloads the binary
for the current OS and architecture, and atomically replaces the running binary.

Use --check-only to print availability without downloading anything.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			stdout := cmd.OutOrStdout()
			stderr := cmd.ErrOrStderr()

			if sourceURL == "" {
				sourceURL = defaultReleasesURL
			}

			// Build an HTTP client with a 60-second timeout for downloads.
			httpClient := &http.Client{Timeout: 60 * time.Second}

			// Fetch the latest release manifest.
			info, err := fetchRelease(httpClient, sourceURL)
			if err != nil {
				fmt.Fprintf(stderr, "memoria: fetch release: %v\n", err)
				return err
			}

			// Strip the "cli/" prefix from the tag to get a clean semver.
			latestTag := info.TagName
			latestVer := strings.TrimPrefix(latestTag, "cli/")

			// Compare against the running version.
			cmp := version.CompareSemver(version.Version, latestVer)

			// Always bump LastChecked, even if no update is available.
			bumpLastChecked()

			if cmp >= 0 {
				// Current version is same or newer — no update needed.
				fmt.Fprintf(stdout, "memoria %s is up to date.\n", version.Version)
				return nil
			}

			// An update is available.
			if checkOnly {
				fmt.Fprintf(stdout,
					"Update available: %s → %s. Run `memoria update` to install.\n",
					version.Version, latestVer)
				return nil
			}

			// Find the matching asset for this OS/arch.
			assetName := fmt.Sprintf("memoria-%s-%s", runtime.GOOS, runtime.GOARCH)
			var downloadURL string
			for _, a := range info.Assets {
				if a.Name == assetName {
					downloadURL = a.BrowserDownloadURL
					break
				}
			}
			if downloadURL == "" {
				err := fmt.Errorf("no asset found for %s/%s (looking for %q)",
					runtime.GOOS, runtime.GOARCH, assetName)
				fmt.Fprintf(stderr, "memoria: %v\n", err)
				return err
			}

			fmt.Fprintf(stdout, "Downloading memoria %s for %s/%s...\n",
				latestVer, runtime.GOOS, runtime.GOARCH)

			// Resolve the real binary path (follow symlinks).
			exePath, err := executablePath()
			if err != nil {
				fmt.Fprintf(stderr, "memoria: resolve executable path: %v\n", err)
				return err
			}
			exePath, err = filepath.EvalSymlinks(exePath)
			if err != nil {
				fmt.Fprintf(stderr, "memoria: eval symlinks: %v\n", err)
				return err
			}

			// Download to a temp file in the same directory so rename is atomic.
			exeDir := filepath.Dir(exePath)
			if err := downloadAndReplace(httpClient, downloadURL, exeDir, exePath); err != nil {
				fmt.Fprintf(stderr, "memoria: %v\n", err)
				return err
			}

			fmt.Fprintf(stdout, "Installed memoria %s.\n", latestVer)
			return nil
		},
	}

	cmd.Flags().BoolVar(&checkOnly, "check-only",
		false, "Check for updates without downloading")
	cmd.Flags().StringVar(&sourceURL, "source",
		"", "Override the releases JSON endpoint (for testing)")

	return cmd
}

// fetchRelease GETs the releases JSON endpoint and parses the response.
func fetchRelease(client *http.Client, url string) (*releaseInfo, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "memoria-cli/"+version.Version)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var info releaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("decode release JSON: %w", err)
	}
	return &info, nil
}

// downloadAndReplace downloads the asset at url into a temp file in dir, then
// atomically renames it to destPath. Sets 0755 permissions on the temp file.
// On any error, the temp file is removed before returning.
func downloadAndReplace(client *http.Client, url, dir, destPath string) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build download request: %w", err)
	}
	req.Header.Set("User-Agent", "memoria-cli/"+version.Version)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("download HTTP %d: %s", resp.StatusCode, string(body))
	}

	tmp, err := os.CreateTemp(dir, "memoria.*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	committed := false
	defer func() {
		if !committed {
			os.Remove(tmpName)
		}
	}()

	written, err := io.Copy(tmp, io.LimitReader(resp.Body, maxBinarySize))
	if err != nil {
		tmp.Close()
		return fmt.Errorf("write download: %w", err)
	}
	// If we hit the ceiling exactly, the server may have been serving more.
	// Treat this as an error rather than silently accepting a truncated binary.
	if written == maxBinarySize && resp.ContentLength > maxBinarySize {
		tmp.Close()
		return fmt.Errorf("download exceeded %d-byte ceiling (Content-Length: %d)", maxBinarySize, resp.ContentLength)
	}
	if err := tmp.Chmod(0o755); err != nil {
		tmp.Close()
		return fmt.Errorf("chmod: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmpName, destPath); err != nil {
		return fmt.Errorf("rename to %s: %w", destPath, err)
	}
	committed = true
	return nil
}

// bumpLastChecked updates LastChecked in the state file. Non-fatal: ignores errors.
func bumpLastChecked() {
	state, err := config.Read()
	if err != nil {
		// Don't overwrite a state file we couldn't read — that risks blowing
		// away BoundTenant/BoundBrain/TokenSource.
		return
	}
	state.LastChecked = time.Now().UTC()
	_ = config.Write(state) //nolint:errcheck — best-effort
}
