package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"text/tabwriter"

	"github.com/paladinai/paladinai/cmd/paladin/client"
	"github.com/spf13/cobra"
)

var slugRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}[a-z0-9]$`)

var tenantCmd = &cobra.Command{
	Use:   "tenant",
	Short: "Manage tenants (admin only)",
}

var tenantListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all tenants",
	RunE: func(cmd *cobra.Command, args []string) error {
		u, err := url.Parse(authURL(cmd))
		if err != nil {
			return fmt.Errorf("invalid auth-url: %w", err)
		}
		u.Path = "/api/v1/tenants"

		body, err := client.Get(cmd.Context(), u.String(), client.Options{AdminSecret: adminSecret(cmd)})
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

		if !slugRE.MatchString(slug) {
			return fmt.Errorf("invalid slug %q: must be 2-64 chars, lowercase alphanumeric and hyphens, not starting or ending with a hyphen", slug)
		}

		payload := map[string]any{"slug": slug, "name": name}
		data, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal payload: %w", err)
		}

		u, err := url.Parse(authURL(cmd))
		if err != nil {
			return fmt.Errorf("invalid auth-url: %w", err)
		}
		u.Path = "/api/v1/tenants"

		body, status, err := client.DoJSON(cmd.Context(), http.MethodPost, u.String(),
			client.Options{AdminSecret: adminSecret(cmd)}, bytes.NewReader(data))
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
	u, err := url.Parse(authURL(cmd))
	if err != nil {
		return fmt.Errorf("invalid auth-url: %w", err)
	}
	u.Path = fmt.Sprintf("/api/v1/tenants/%s/%s", url.PathEscape(id), action)

	body, status, err := client.DoJSON(cmd.Context(), http.MethodPost, u.String(),
		client.Options{AdminSecret: adminSecret(cmd)}, nil)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("API error %d: %s", status, string(body))
	}
	past := map[string]string{"suspend": "suspended", "resume": "resumed"}
	fmt.Printf("Tenant %q %s.\n", id, past[action])
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
		id, _ := t["id"].(string)
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

var tenantMigrateCmd = &cobra.Command{
	Use:   "migrate <slug>",
	Short: "Migrate tenant to a different deployment tier",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		slug := args[0]
		if !slugRE.MatchString(slug) {
			return fmt.Errorf("invalid slug %q: must be 2-64 chars, lowercase alphanumeric and hyphens", slug)
		}

		tier, _ := cmd.Flags().GetString("tier")
		if tier == "" {
			return fmt.Errorf("--tier is required (pool, bridge, silo)")
		}
		validTiers := map[string]bool{"pool": true, "bridge": true, "silo": true}
		if !validTiers[tier] {
			return fmt.Errorf("invalid tier %q: must be pool, bridge, or silo", tier)
		}

		u, err := url.Parse(authURL(cmd))
		if err != nil {
			return fmt.Errorf("invalid auth-url: %w", err)
		}
		u.Path = fmt.Sprintf("/api/v1/tenants/%s/migrate", url.PathEscape(slug))

		payload := map[string]string{"tier": tier}
		data, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal payload: %w", err)
		}

		body, status, err := client.DoJSON(cmd.Context(), http.MethodPost, u.String(),
			client.Options{AdminSecret: adminSecret(cmd)}, bytes.NewReader(data))
		if err != nil {
			return err
		}
		if status < 200 || status >= 300 {
			return fmt.Errorf("API error %d: %s", status, string(body))
		}
		fmt.Fprintf(os.Stdout, "Tenant %q migration to %q tier initiated.\n", slug, tier)
		return nil
	},
}

var tenantDeleteCmd = &cobra.Command{
	Use:   "delete <slug>",
	Short: "Permanently delete a tenant (irreversible)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		slug := args[0]
		if !slugRE.MatchString(slug) {
			return fmt.Errorf("invalid slug %q: must be 2-64 chars, lowercase alphanumeric and hyphens", slug)
		}

		yes, _ := cmd.Flags().GetBool("yes")
		if !yes {
			fmt.Fprintf(os.Stderr, "WARNING: This will permanently delete tenant %q and all its data. Use --yes to skip in scripts.\n", slug)
			fmt.Fprint(os.Stderr, "Type the tenant slug to confirm: ")
			r := bufio.NewReader(os.Stdin)
			confirm, err := readLine(r)
			if err != nil {
				return fmt.Errorf("read confirmation: %w", err)
			}
			if confirm != slug {
				return fmt.Errorf("confirmation mismatch — deletion cancelled")
			}
		}

		u, err := url.Parse(authURL(cmd))
		if err != nil {
			return fmt.Errorf("invalid auth-url: %w", err)
		}
		u.Path = fmt.Sprintf("/api/v1/tenants/%s", url.PathEscape(slug))

		body, status, err := client.DoJSON(cmd.Context(), http.MethodDelete, u.String(),
			client.Options{AdminSecret: adminSecret(cmd)}, nil)
		if err != nil {
			return err
		}
		if status < 200 || status >= 300 {
			return fmt.Errorf("API error %d: %s", status, string(body))
		}
		fmt.Fprintf(os.Stdout, "Tenant %q deleted.\n", slug)
		return nil
	},
}

func init() {
	tenantCmd.PersistentFlags().String("auth-url", envStr("PALADIN_AUTH_URL", "http://localhost:9003"), "PaladinAI Auth service URL")
	tenantCmd.PersistentFlags().String("admin-secret", os.Getenv("PALADIN_ADMIN_SECRET"), "Admin secret for tenant management")

	tenantCreateCmd.Flags().String("slug", "", "Tenant slug (required, 2-64 chars, lowercase alphanumeric/hyphens)")
	tenantCreateCmd.Flags().String("name", "", "Tenant display name (required)")
	_ = tenantCreateCmd.MarkFlagRequired("slug")
	_ = tenantCreateCmd.MarkFlagRequired("name")

	tenantMigrateCmd.Flags().String("tier", "", "Target tier: pool, bridge, silo")
	_ = tenantMigrateCmd.MarkFlagRequired("tier")

	tenantDeleteCmd.Flags().Bool("yes", false, "Skip confirmation prompt (use with caution)")

	tenantCmd.AddCommand(tenantListCmd, tenantCreateCmd, tenantSuspendCmd, tenantResumeCmd,
		tenantMigrateCmd, tenantDeleteCmd)
}
