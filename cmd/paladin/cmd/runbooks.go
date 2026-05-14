package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/paladinai/paladinai/cmd/paladin/client"
	"github.com/spf13/cobra"
)

var runbooksCmd = &cobra.Command{
	Use:   "runbooks",
	Short: "Manage and search runbooks",
}

// paladin runbooks import --source <github|confluence|notion|file> [flags]
var runbooksImportCmd = &cobra.Command{
	Use:   "import",
	Short: "Import runbooks from an external source",
	Long: `Import runbooks from GitHub, Confluence, Notion, or a local file.

Examples:
  paladin runbooks import --source github --repo owner/repo --path docs/runbooks
  paladin runbooks import --source file --path ./runbook.md
  paladin runbooks import --source confluence --space OPS`,
	RunE: func(cmd *cobra.Command, args []string) error {
		tenant, err := requireTenant(cmd)
		if err != nil {
			return err
		}

		source, _ := cmd.Flags().GetString("source")
		if source == "" {
			return fmt.Errorf("--source is required (github, confluence, notion, file)")
		}
		validSources := map[string]bool{"github": true, "confluence": true, "notion": true, "file": true}
		if !validSources[source] {
			return fmt.Errorf("invalid --source %q: must be one of github, confluence, notion, file", source)
		}

		payload := runbookImportPayload(cmd, source)

		data, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal payload: %w", err)
		}

		u, err := url.Parse(apiURL(cmd))
		if err != nil {
			return fmt.Errorf("invalid api-url: %w", err)
		}
		u.Path = "/api/v1/runbooks/import"

		opts, err := commandOptions(cmd, tenant)
		if err != nil {
			return err
		}
		body, status, err := client.DoJSON(cmd.Context(), http.MethodPost, u.String(),
			opts, bytes.NewReader(data))
		if err != nil {
			return err
		}
		if status < 200 || status >= 300 {
			return fmt.Errorf("API error %d: %s", status, string(body))
		}

		var result runbooksImportResult
		result.Source = source
		if err := json.Unmarshal(body, &result); err != nil {
			if outputFormat(cmd) == "json" {
				return json.NewEncoder(os.Stdout).Encode(runbooksImportResult{
					Source:  source,
					Message: string(body),
				})
			}
			fmt.Fprintln(os.Stdout, string(body))
			return nil
		}
		return writeRunbooksImportResult(cmd, result)
	},
}

type runbooksImportResult struct {
	Source   string `json:"source"`
	Imported int    `json:"imported"`
	JobID    string `json:"job_id,omitempty"`
	Message  string `json:"message,omitempty"`
}

func runbookImportPayload(cmd *cobra.Command, source string) map[string]any {
	payload := map[string]any{"source": source}
	for _, key := range []string{"repo", "path", "space", "branch", "url", "database-id", "token"} {
		value, _ := cmd.Flags().GetString(key)
		if strings.TrimSpace(value) != "" {
			payload[key] = value
		}
	}
	return payload
}

func writeRunbooksImportResult(cmd *cobra.Command, result runbooksImportResult) error {
	if outputFormat(cmd) == "json" {
		if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
			return fmt.Errorf("write import result: %w", err)
		}
		return nil
	}
	if result.JobID != "" {
		fmt.Printf("Import started (job: %s). Runbooks will be embedded in the background.\n", result.JobID)
	} else {
		fmt.Printf("Imported %d runbook(s) from %s.\n", result.Imported, result.Source)
	}
	return nil
}

// paladin runbooks list
var runbooksListCmd = &cobra.Command{
	Use:   "list",
	Short: "List imported runbooks and their embedding status",
	RunE: func(cmd *cobra.Command, args []string) error {
		tenant, err := requireTenant(cmd)
		if err != nil {
			return err
		}

		u, err := url.Parse(apiURL(cmd))
		if err != nil {
			return fmt.Errorf("invalid api-url: %w", err)
		}
		u.Path = "/api/v1/runbooks"

		q := u.Query()
		if limit, _ := cmd.Flags().GetInt("limit"); limit > 0 {
			q.Set("limit", fmt.Sprintf("%d", limit))
		}
		if src, _ := cmd.Flags().GetString("source"); src != "" {
			q.Set("source", src)
		}
		u.RawQuery = q.Encode()

		opts, err := commandOptions(cmd, tenant)
		if err != nil {
			return err
		}
		body, err := client.Get(cmd.Context(), u.String(), opts)
		if err != nil {
			return err
		}

		if outputFormat(cmd) == "json" {
			fmt.Fprintln(os.Stdout, string(body))
			return nil
		}
		return printRunbooksTable(body)
	},
}

// paladin runbooks search <query>
var runbooksSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Semantic search across imported runbooks",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		tenant, err := requireTenant(cmd)
		if err != nil {
			return err
		}

		query := strings.Join(args, " ")
		topK, _ := cmd.Flags().GetInt("top")

		payload := map[string]any{
			"query": query,
			"top_k": topK,
		}
		data, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal payload: %w", err)
		}

		u, err := url.Parse(apiURL(cmd))
		if err != nil {
			return fmt.Errorf("invalid api-url: %w", err)
		}
		u.Path = "/api/v1/runbooks/search"

		opts, err := commandOptions(cmd, tenant)
		if err != nil {
			return err
		}
		body, status, err := client.DoJSON(cmd.Context(), http.MethodPost, u.String(),
			opts, bytes.NewReader(data))
		if err != nil {
			return err
		}
		if status < 200 || status >= 300 {
			return fmt.Errorf("API error %d: %s", status, string(body))
		}

		if outputFormat(cmd) == "json" {
			fmt.Fprintln(os.Stdout, string(body))
			return nil
		}
		return printRunbookSearchResults(body)
	},
}

func printRunbooksTable(body []byte) error {
	var response struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		fmt.Fprintln(os.Stdout, string(body))
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tTITLE\tSOURCE\tEMBEDDED\tUPDATED")
	for _, rb := range response.Data {
		id := strField(rb, "id")
		if len(id) > 12 {
			id = id[:12]
		}
		title := strField(rb, "title")
		if len(title) > 40 {
			title = title[:40] + "..."
		}
		source := strField(rb, "source")
		embedded := fmt.Sprintf("%v", rb["embedded"])
		updated := strField(rb, "updated_at")
		if len(updated) > 19 {
			updated = updated[:19]
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", id, title, source, embedded, updated)
	}
	return w.Flush()
}

func printRunbookSearchResults(body []byte) error {
	var response struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		fmt.Fprintln(os.Stdout, string(body))
		return nil
	}
	if len(response.Data) == 0 {
		fmt.Fprintln(os.Stdout, "No matching runbooks found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "SCORE\tTITLE\tSOURCE\tID")
	for _, rb := range response.Data {
		score := fmt.Sprintf("%.3f", floatField(rb, "score"))
		title := strField(rb, "title")
		if len(title) > 48 {
			title = title[:48] + "..."
		}
		source := strField(rb, "source")
		id := strField(rb, "id")
		if len(id) > 12 {
			id = id[:12]
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", score, title, source, id)
	}
	return w.Flush()
}

func floatField(m map[string]any, key string) float64 {
	if v, ok := m[key]; ok {
		switch f := v.(type) {
		case float64:
			return f
		case float32:
			return float64(f)
		}
	}
	return 0
}

func init() {
	runbooksImportCmd.Flags().String("source", "", "Source type: github, confluence, notion, file")
	runbooksImportCmd.Flags().String("repo", "", "GitHub repository (owner/repo) — for --source github")
	runbooksImportCmd.Flags().String("path", "", "Path within repo or local file path")
	runbooksImportCmd.Flags().String("space", "", "Confluence/Notion space key — for --source confluence/notion")
	runbooksImportCmd.Flags().String("branch", "", "Git branch to import from — for --source github")
	runbooksImportCmd.Flags().String("url", "", "Source base URL — for Confluence or other hosted sources")
	runbooksImportCmd.Flags().String("database-id", "", "Notion database ID — for --source notion")
	runbooksImportCmd.Flags().String("token", "", "Source access token; prefer env vars in shells")

	runbooksListCmd.Flags().Int("limit", 50, "Maximum number of runbooks to return")
	runbooksListCmd.Flags().String("source", "", "Filter by source type")

	runbooksSearchCmd.Flags().Int("top", 5, "Number of top results to return")

	runbooksCmd.AddCommand(runbooksImportCmd, runbooksListCmd, runbooksSearchCmd)
	rootCmd.AddCommand(runbooksCmd)
}
