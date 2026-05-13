package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/paladinai/paladinai/cmd/paladin/client"
	"github.com/paladinai/paladinai/internal/integrationpkg"
	"github.com/spf13/cobra"
)

var integrationsCmd = &cobra.Command{
	Use:   "integrations",
	Short: "Manage PaladinAI integrations (MCP server marketplace)",
}

// integrationsDir resolves the directory holding integration.yaml definitions.
// Order: --integrations-dir flag, ./integrations, then <binary>/../integrations.
func integrationsDir(cmd *cobra.Command) string {
	if dir, _ := cmd.Flags().GetString("integrations-dir"); dir != "" {
		return dir
	}
	if _, err := os.Stat("integrations"); err == nil {
		return "integrations"
	}
	if exe, err := exec.LookPath(os.Args[0]); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "..", "integrations")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return "integrations"
}

var integrationsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available integrations",
	RunE: func(cmd *cobra.Command, args []string) error {
		all, err := integrationpkg.LoadAll(integrationsDir(cmd))
		if err != nil {
			return fmt.Errorf("load integrations: %w", err)
		}

		outputFmt, _ := cmd.Flags().GetString("output")
		if outputFmt == "json" {
			data, err := json.MarshalIndent(all, "", "  ")
			if err != nil {
				return fmt.Errorf("marshal json: %w", err)
			}
			fmt.Fprintln(os.Stdout, string(data))
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tVERSION\tDESCRIPTION\tTOOLS")
		for _, i := range all {
			toolsStr := strings.Join(i.Tools, ",")
			if toolsStr == "" {
				toolsStr = "-"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", i.Name, i.Version, i.Description, toolsStr)
		}
		return w.Flush()
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

		opts, err := commandOptions(cmd, tenant)
		if err != nil {
			return err
		}
		body, err := client.Get(cmd.Context(), u.String(), opts)
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

		opts, err := commandOptions(cmd, tenant)
		if err != nil {
			return err
		}
		body, status, err := client.DoJSON(cmd.Context(), http.MethodPost, u.String(),
			opts, nil)
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
		integ, err := integrationpkg.LoadByName(integrationsDir(cmd), args[0])
		if err != nil {
			return fmt.Errorf("load integration: %w", err)
		}

		outputFmt, _ := cmd.Flags().GetString("output")
		if outputFmt == "json" {
			data, err := json.MarshalIndent(integ, "", "  ")
			if err != nil {
				return fmt.Errorf("marshal json: %w", err)
			}
			fmt.Fprintln(os.Stdout, string(data))
			return nil
		}

		printIntegrationDefinition(integ)
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

func printIntegrationDefinition(i *integrationpkg.Integration) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "Name:\t%s\n", i.Name)
	fmt.Fprintf(w, "Version:\t%s\n", i.Version)
	fmt.Fprintf(w, "Description:\t%s\n", i.Description)
	fmt.Fprintf(w, "Docs:\t%s\n", i.DocsURL)
	fmt.Fprintf(w, "Receiver:\t%s\n", i.Receiver.Type)
	if i.Receiver.Path != "" {
		fmt.Fprintf(w, "  Path:\t%s\n", i.Receiver.Path)
	}
	if i.Receiver.HMACHeader != "" {
		fmt.Fprintf(w, "  HMAC Header:\t%s\n", i.Receiver.HMACHeader)
	}
	fmt.Fprintf(w, "Auth:\t%s\n", i.Auth.Type)
	for _, f := range i.Auth.Fields {
		secret := ""
		if f.Secret {
			secret = " (secret)"
		}
		fmt.Fprintf(w, "  - %s%s\t%s\n", f.Name, secret, f.Description)
	}
	if len(i.Tools) > 0 {
		fmt.Fprintf(w, "Tools:\t%s\n", strings.Join(i.Tools, ", "))
	}
	if len(i.ConfigSchema) > 0 {
		fmt.Fprintln(w, "Config schema:")
		for key, field := range i.ConfigSchema {
			req := ""
			if field.Required {
				req = " (required)"
			}
			fmt.Fprintf(w, "  - %s [%s]%s\t%s\n", key, field.Type, req, field.Description)
		}
	}
	w.Flush()
}

func init() {
	integrationsCmd.PersistentFlags().String("integrations-dir", "", "Path to integrations/ directory with integration.yaml files")
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
