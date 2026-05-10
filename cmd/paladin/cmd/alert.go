package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

var alertCmd = &cobra.Command{
	Use:   "alert",
	Short: "Manage and query alerts",
}

var alertListCmd = &cobra.Command{
	Use:   "list",
	Short: "List active alerts for the tenant",
	RunE: func(cmd *cobra.Command, args []string) error {
		tenant, err := requireTenant(cmd)
		if err != nil {
			return err
		}

		apiURL, _ := cmd.Flags().GetString("api-url")
		url := fmt.Sprintf("%s/api/v1/alerts?status=firing", apiURL)

		req, err := http.NewRequestWithContext(cmd.Context(), http.MethodGet, url, nil)
		if err != nil {
			return fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("X-Tenant-ID", tenant)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("request failed: %w", err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
		}

		outputFmt, _ := cmd.Flags().GetString("output")
		if outputFmt == "json" {
			fmt.Fprintln(os.Stdout, string(body))
			return nil
		}

		return printAlertTable(body)
	},
}

func printAlertTable(body []byte) error {
	var response struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		fmt.Fprintln(os.Stdout, string(body))
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "FINGERPRINT\tSEVERITY\tSTATUS\tCORRELATION\tAGE")
	for _, a := range response.Data {
		fp := strField(a, "fingerprint")
		sev := strField(a, "severity")
		status := strField(a, "status")
		corrID := strField(a, "correlation_id")
		age := ""
		if sa, ok := a["starts_at"].(string); ok {
			if t, err := time.Parse(time.RFC3339, sa); err == nil {
				age = time.Since(t).Round(time.Second).String()
			}
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", fp, sev, status, corrID, age)
	}
	return w.Flush()
}

func strField(m map[string]any, key string) string {
	v, _ := m[key].(string)
	if len(v) > 20 {
		return v[:20]
	}
	return v
}

func init() {
	alertCmd.AddCommand(alertListCmd)
}
