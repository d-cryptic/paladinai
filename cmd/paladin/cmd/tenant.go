package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"text/tabwriter"

	"github.com/paladinai/paladinai/cmd/paladin/client"
	"github.com/spf13/cobra"
)

var tenantCmd = &cobra.Command{
	Use:   "tenant",
	Short: "Manage tenants (admin only)",
}

var tenantListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all tenants",
	RunE: func(cmd *cobra.Command, args []string) error {
		u, _ := url.Parse(authURL(cmd))
		u.Path = "/api/v1/tenants"

		body, err := client.Get(cmd.Context(), u.String(), client.Options{Token: adminSecret(cmd)})
		if err != nil {
			return err
		}

		outputFmt, _ := cmd.Flags().GetString("output")
		if outputFmt == "json" {
			fmt.Fprintln(os.Stdout, string(body))
			return nil
		}
		return printTenantTable(body)
	},
}

var tenantCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new tenant",
	RunE: func(cmd *cobra.Command, args []string) error {
		slug, _ := cmd.Flags().GetString("slug")
		name, _ := cmd.Flags().GetString("name")

		payload := map[string]any{"slug": slug, "name": name}
		data, _ := json.Marshal(payload)

		u, _ := url.Parse(authURL(cmd))
		u.Path = "/api/v1/tenants"

		body, status, err := client.DoJSON(cmd.Context(), http.MethodPost, u.String(),
			client.Options{Token: adminSecret(cmd)}, bytes.NewReader(data))
		if err != nil {
			return err
		}
		if status != http.StatusCreated {
			return fmt.Errorf("API error %d: %s", status, string(body))
		}

		var t map[string]any
		if err := json.Unmarshal(body, &t); err == nil {
			fmt.Printf("Tenant created: %s (slug: %s)\n", t["id"], t["slug"])
		} else {
			fmt.Println("Tenant created successfully.")
		}
		return nil
	},
}

var tenantSuspendCmd = &cobra.Command{
	Use:   "suspend <tenant-id>",
	Short: "Suspend a tenant",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return tenantStateAction(cmd, args[0], "suspend")
	},
}

var tenantResumeCmd = &cobra.Command{
	Use:   "resume <tenant-id>",
	Short: "Resume a suspended tenant",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return tenantStateAction(cmd, args[0], "resume")
	},
}

func tenantStateAction(cmd *cobra.Command, id, action string) error {
	u, _ := url.Parse(authURL(cmd))
	u.Path = fmt.Sprintf("/api/v1/tenants/%s/%s", url.PathEscape(id), action)

	body, status, err := client.DoJSON(cmd.Context(), http.MethodPost, u.String(),
		client.Options{Token: adminSecret(cmd)}, nil)
	if err != nil {
		return err
	}
	if status != http.StatusOK && status != http.StatusNoContent {
		return fmt.Errorf("API error %d: %s", status, string(body))
	}
	fmt.Printf("Tenant %q %sd.\n", id, action)
	return nil
}

func printTenantTable(body []byte) error {
	var response struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		fmt.Fprintln(os.Stdout, string(body))
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tSLUG\tNAME\tSTATE\tCREATED")
	for _, t := range response.Data {
		id := strField(t, "id")
		slug := strField(t, "slug")
		name := strField(t, "name")
		state := strField(t, "state")
		created := strField(t, "created_at")
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", id, slug, name, state, created)
	}
	return w.Flush()
}

// authURL returns the paladin-auth service URL from the --auth-url flag.
func authURL(cmd *cobra.Command) string {
	f := cmd.Flag("auth-url")
	if f == nil {
		return "http://localhost:9003"
	}
	return f.Value.String()
}

// adminSecret returns the X-Admin-Secret value from --admin-secret flag or env.
func adminSecret(cmd *cobra.Command) string {
	f := cmd.Flag("admin-secret")
	if f == nil {
		return ""
	}
	return f.Value.String()
}

func init() {
	tenantCmd.PersistentFlags().String("auth-url", envStr("PALADIN_AUTH_URL", "http://localhost:9003"), "PaladinAI Auth service URL")
	tenantCmd.PersistentFlags().String("admin-secret", os.Getenv("PALADIN_ADMIN_SECRET"), "Admin secret for tenant management")

	tenantCreateCmd.Flags().String("slug", "", "Tenant slug (required, 2-64 chars, lowercase alphanumeric/hyphens)")
	tenantCreateCmd.Flags().String("name", "", "Tenant display name (required)")
	_ = tenantCreateCmd.MarkFlagRequired("slug")
	_ = tenantCreateCmd.MarkFlagRequired("name")

	tenantCmd.AddCommand(tenantListCmd, tenantCreateCmd, tenantSuspendCmd, tenantResumeCmd)
}
