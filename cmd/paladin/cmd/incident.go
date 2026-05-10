package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"text/tabwriter"
	"time"

	"github.com/paladinai/paladinai/cmd/paladin/client"
	"github.com/spf13/cobra"
)

var incidentCmd = &cobra.Command{
	Use:   "incident",
	Short: "Manage incidents",
}

var incidentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List incidents for the tenant",
	RunE: func(cmd *cobra.Command, args []string) error {
		tenant, err := requireTenant(cmd)
		if err != nil {
			return err
		}

		u, err := url.Parse(apiURL(cmd))
		if err != nil {
			return fmt.Errorf("invalid api-url: %w", err)
		}
		u.Path = "/api/v1/incidents"
		q := u.Query()
		if status, _ := cmd.Flags().GetString("status"); status != "" {
			q.Set("status", status)
		}
		u.RawQuery = q.Encode()

		body, err := client.Get(cmd.Context(), u.String(), client.Options{TenantID: tenant, Token: optToken(cmd)})
		if err != nil {
			return err
		}

		outputFmt, _ := cmd.Flags().GetString("output")
		if outputFmt == "json" {
			fmt.Fprintln(os.Stdout, string(body))
			return nil
		}
		return printIncidentTable(body)
	},
}

var incidentShowCmd = &cobra.Command{
	Use:   "show <incident-id>",
	Short: "Show details of a specific incident",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		tenant, err := requireTenant(cmd)
		if err != nil {
			return err
		}

		u, err := url.Parse(apiURL(cmd))
		if err != nil {
			return fmt.Errorf("invalid api-url: %w", err)
		}
		u.Path = fmt.Sprintf("/api/v1/incidents/%s", url.PathEscape(args[0]))

		body, err := client.Get(cmd.Context(), u.String(), client.Options{TenantID: tenant, Token: optToken(cmd)})
		if err != nil {
			return err
		}

		outputFmt, _ := cmd.Flags().GetString("output")
		if outputFmt == "json" {
			fmt.Fprintln(os.Stdout, string(body))
			return nil
		}

		var inc map[string]any
		if err := json.Unmarshal(body, &inc); err != nil {
			fmt.Fprintln(os.Stdout, string(body))
			return nil
		}
		printIncidentDetail(inc)
		return nil
	},
}

var incidentResolveCmd = &cobra.Command{
	Use:   "resolve <incident-id>",
	Short: "Resolve an incident",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		tenant, err := requireTenant(cmd)
		if err != nil {
			return err
		}

		note, _ := cmd.Flags().GetString("note")
		payload := map[string]any{"resolution_note": note}
		data, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal payload: %w", err)
		}

		u, err := url.Parse(apiURL(cmd))
		if err != nil {
			return fmt.Errorf("invalid api-url: %w", err)
		}
		u.Path = fmt.Sprintf("/api/v1/incidents/%s/resolve", url.PathEscape(args[0]))

		body, status, err := client.DoJSON(cmd.Context(), http.MethodPost, u.String(),
			client.Options{TenantID: tenant, Token: optToken(cmd)}, bytes.NewReader(data))
		if err != nil {
			return err
		}
		if status < 200 || status >= 300 {
			return fmt.Errorf("API error %d: %s", status, string(body))
		}
		fmt.Printf("Incident %q resolved.\n", args[0])
		return nil
	},
}

func printIncidentTable(body []byte) error {
	var response struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		fmt.Fprintln(os.Stdout, string(body))
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tTITLE\tSEVERITY\tSTATUS\tAGE")
	for _, inc := range response.Data {
		// IDs must not be truncated — use raw type assertion
		id, _ := inc["id"].(string)
		title := strField(inc, "title")
		sev := strField(inc, "severity")
		status := strField(inc, "status")
		age := ""
		if ca, ok := inc["created_at"].(string); ok {
			if t, err := time.Parse(time.RFC3339, ca); err == nil {
				age = time.Since(t).Round(time.Second).String()
			}
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", id, title, sev, status, age)
	}
	return w.Flush()
}

func printIncidentDetail(inc map[string]any) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fields := []struct{ k, label string }{
		{"id", "ID"},
		{"title", "Title"},
		{"severity", "Severity"},
		{"status", "Status"},
		{"correlation_id", "Correlation ID"},
		{"created_at", "Created"},
		{"resolved_at", "Resolved"},
	}
	for _, f := range fields {
		v, ok := inc[f.k]
		if !ok || v == nil {
			v = "-"
		}
		fmt.Fprintf(w, "%s:\t%v\n", f.label, v)
	}
	w.Flush()
}

// paladin incidents replay <id>
var incidentReplayCmd = &cobra.Command{
	Use:   "replay <id>",
	Short: "Replay an incident through the current agent build for regression testing",
	Long: `Sends the incident's original alert data through the agent pipeline again.
Useful for testing agent improvements without real infrastructure.

The replay runs asynchronously; use --wait to poll for completion.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		tenant, err := requireTenant(cmd)
		if err != nil {
			return err
		}

		u, err := url.Parse(apiURL(cmd))
		if err != nil {
			return fmt.Errorf("invalid api-url: %w", err)
		}
		u.Path = fmt.Sprintf("/api/v1/incidents/%s/replay", url.PathEscape(args[0]))

		body, status, err := client.DoJSON(cmd.Context(), http.MethodPost, u.String(),
			client.Options{TenantID: tenant, Token: optToken(cmd)}, nil)
		if err != nil {
			return err
		}
		if status < 200 || status >= 300 {
			return fmt.Errorf("API error %d: %s", status, string(body))
		}

		outputFmt, _ := cmd.Flags().GetString("output")
		if outputFmt == "json" {
			fmt.Fprintln(os.Stdout, string(body))
			return nil
		}

		var result struct {
			ReplayID string `json:"replay_id"`
			Status   string `json:"status"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			fmt.Fprintln(os.Stdout, string(body))
			return nil
		}
		fmt.Printf("Replay started (id: %s, status: %s).\n", result.ReplayID, result.Status)
		fmt.Printf("Use `paladin incidents show %s` to check the replayed incident.\n", result.ReplayID)
		return nil
	},
}

func init() {
	incidentListCmd.Flags().String("status", "", "Filter by status (open, resolved)")
	incidentResolveCmd.Flags().String("note", "", "Resolution note")

	incidentCmd.AddCommand(incidentListCmd, incidentShowCmd, incidentResolveCmd, incidentReplayCmd)
}
