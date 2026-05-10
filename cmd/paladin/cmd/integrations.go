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

var integrationsCmd = &cobra.Command{
	Use:   "integrations",
	Short: "Manage PaladinAI integrations (MCP server marketplace)",
}

var integrationsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available integrations",
	RunE: func(cmd *cobra.Command, args []string) error {
		tenant, err := requireTenant(cmd)
		if err != nil {
			return err
		}
		u, err := url.Parse(apiURL(cmd))
		if err != nil {
			return fmt.Errorf("invalid api-url: %w", err)
		}
		u.Path = "/api/v1/integrations"

		body, err := client.Get(cmd.Context(), u.String(), client.Options{TenantID: tenant, Token: optToken(cmd)})
		if err != nil {
			return err
		}

		outputFmt, _ := cmd.Flags().GetString("output")
		if outputFmt == "json" {
			fmt.Fprintln(os.Stdout, string(body))
			return nil
		}
		return printIntegrationsTable(body)
	},
}

var integrationsStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show enabled integrations and their health",
	RunE: func(cmd *cobra.Command, args []string) error {
		tenant, err := requireTenant(cmd)
		if err != nil {
			return err
		}
		u, err := url.Parse(apiURL(cmd))
		if err != nil {
			return fmt.Errorf("invalid api-url: %w", err)
		}
		u.Path = "/api/v1/integrations/status"

		body, err := client.Get(cmd.Context(), u.String(), client.Options{TenantID: tenant, Token: optToken(cmd)})
		if err != nil {
			return err
		}

		outputFmt, _ := cmd.Flags().GetString("output")
		if outputFmt == "json" {
			fmt.Fprintln(os.Stdout, string(body))
			return nil
		}
		return printIntegrationsTable(body)
	},
}

var integrationsEnableCmd = &cobra.Command{
	Use:   "enable <name>",
	Short: "Enable an integration",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		tenant, err := requireTenant(cmd)
		if err != nil {
			return err
		}

		configFile, _ := cmd.Flags().GetString("config")
		var configData []byte
		if configFile != "" {
			var err error
			configData, err = os.ReadFile(configFile)
			if err != nil {
				return fmt.Errorf("read config file: %w", err)
			}
		}

		payload := map[string]any{"name": args[0]}
		if configData != nil {
			var cfg any
			if err := json.Unmarshal(configData, &cfg); err != nil {
				return fmt.Errorf("parse config JSON: %w", err)
			}
			payload["config"] = cfg
		}
		data, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal payload: %w", err)
		}

		u, err := url.Parse(apiURL(cmd))
		if err != nil {
			return fmt.Errorf("invalid api-url: %w", err)
		}
		u.Path = fmt.Sprintf("/api/v1/integrations/%s/enable", url.PathEscape(args[0]))

		body, status, err := client.DoJSON(cmd.Context(), http.MethodPost, u.String(),
			client.Options{TenantID: tenant, Token: optToken(cmd)}, bytes.NewReader(data))
		if err != nil {
			return err
		}
		if status < 200 || status >= 300 {
			return fmt.Errorf("API error %d: %s", status, string(body))
		}
		fmt.Printf("Integration %q enabled.\n", args[0])
		return nil
	},
}

var integrationsDisableCmd = &cobra.Command{
	Use:   "disable <name>",
	Short: "Disable an integration",
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
		u.Path = fmt.Sprintf("/api/v1/integrations/%s/disable", url.PathEscape(args[0]))

		body, status, err := client.DoJSON(cmd.Context(), http.MethodPost, u.String(),
			client.Options{TenantID: tenant, Token: optToken(cmd)}, nil)
		if err != nil {
			return err
		}
		if status < 200 || status >= 300 {
			return fmt.Errorf("API error %d: %s", status, string(body))
		}
		fmt.Printf("Integration %q disabled.\n", args[0])
		return nil
	},
}

var integrationsShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show details of a specific integration",
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
		u.Path = fmt.Sprintf("/api/v1/integrations/%s", url.PathEscape(args[0]))

		body, err := client.Get(cmd.Context(), u.String(), client.Options{TenantID: tenant, Token: optToken(cmd)})
		if err != nil {
			return err
		}

		outputFmt, _ := cmd.Flags().GetString("output")
		if outputFmt == "json" {
			fmt.Fprintln(os.Stdout, string(body))
			return nil
		}

		var result map[string]any
		if err := json.Unmarshal(body, &result); err != nil {
			fmt.Fprintln(os.Stdout, string(body))
			return nil
		}
		printIntegrationDetail(result)
		return nil
	},
}

func printIntegrationsTable(body []byte) error {
	var response struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		fmt.Fprintln(os.Stdout, string(body))
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tCATEGORY\tSTATUS\tVERSION\tHEALTHY")
	for _, i := range response.Data {
		name := strField(i, "name")
		cat := strField(i, "category")
		status := strField(i, "status")
		version := strField(i, "version")
		healthy := fmt.Sprintf("%v", i["healthy"])
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", name, cat, status, version, healthy)
	}
	return w.Flush()
}

func printIntegrationDetail(i map[string]any) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fields := []struct{ k, label string }{
		{"name", "Name"},
		{"category", "Category"},
		{"status", "Status"},
		{"version", "Version"},
		{"healthy", "Healthy"},
		{"docs_url", "Docs"},
	}
	for _, f := range fields {
		v, ok := i[f.k]
		if !ok || v == nil {
			v = "-"
		}
		fmt.Fprintf(w, "%s:\t%v\n", f.label, v)
	}

	// Print tools list if present
	if tools, ok := i["tools"].([]any); ok && len(tools) > 0 {
		names := make([]string, 0, len(tools))
		for _, t := range tools {
			if name, ok := t.(string); ok {
				names = append(names, name)
			}
		}
		fmt.Fprintf(w, "Tools:\t%s\n", strings.Join(names, ", "))
	}
	w.Flush()
}

func init() {
	integrationsEnableCmd.Flags().String("config", "", "Path to JSON config file for the integration")

	integrationsCmd.AddCommand(
		integrationsListCmd,
		integrationsStatusCmd,
		integrationsEnableCmd,
		integrationsDisableCmd,
		integrationsShowCmd,
	)
	rootCmd.AddCommand(integrationsCmd)
}
