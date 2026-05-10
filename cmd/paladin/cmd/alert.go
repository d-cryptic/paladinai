package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"text/tabwriter"
	"time"

	"github.com/paladinai/paladinai/cmd/paladin/client"
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
		u, err := url.Parse(apiURL)
		if err != nil {
			return fmt.Errorf("invalid api-url: %w", err)
		}
		u.Path = "/api/v1/alerts"
		q := u.Query()
		q.Set("status", "firing")
		u.RawQuery = q.Encode()

		body, err := client.Get(cmd.Context(), u.String(), client.Options{
			TenantID: tenant,
			Token:    optToken(cmd),
		})
		if err != nil {
			return err
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
