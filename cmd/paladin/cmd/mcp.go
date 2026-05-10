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

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Manage MCP server registrations",
}

var mcpListCmd = &cobra.Command{
	Use:   "list",
	Short: "List registered MCP servers for the tenant",
	RunE: func(cmd *cobra.Command, args []string) error {
		tenant, err := requireTenant(cmd)
		if err != nil {
			return err
		}
		u, err := url.Parse(apiURL(cmd))
		if err != nil {
			return fmt.Errorf("invalid api-url: %w", err)
		}
		u.Path = "/api/v1/mcp/servers"

		body, err := client.Get(cmd.Context(), u.String(), client.Options{TenantID: tenant, Token: optToken(cmd)})
		if err != nil {
			return err
		}

		outputFmt, _ := cmd.Flags().GetString("output")
		if outputFmt == "json" {
			fmt.Fprintln(os.Stdout, string(body))
			return nil
		}
		return printMCPTable(body)
	},
}

var mcpRegisterCmd = &cobra.Command{
	Use:   "register",
	Short: "Register an MCP server",
	RunE: func(cmd *cobra.Command, args []string) error {
		tenant, err := requireTenant(cmd)
		if err != nil {
			return err
		}

		id, _ := cmd.Flags().GetString("id")
		name, _ := cmd.Flags().GetString("name")
		endpoint, _ := cmd.Flags().GetString("endpoint")
		capsStr, _ := cmd.Flags().GetString("capabilities")

		caps := []string{}
		for _, c := range strings.Split(capsStr, ",") {
			if t := strings.TrimSpace(c); t != "" {
				caps = append(caps, t)
			}
		}

		payload := map[string]any{
			"id":           id,
			"name":         name,
			"endpoint":     endpoint,
			"capabilities": caps,
		}
		data, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal payload: %w", err)
		}

		u, err := url.Parse(apiURL(cmd))
		if err != nil {
			return fmt.Errorf("invalid api-url: %w", err)
		}
		u.Path = "/api/v1/mcp/servers"

		body, status, err := client.DoJSON(cmd.Context(), http.MethodPost, u.String(), client.Options{TenantID: tenant, Token: optToken(cmd)}, bytes.NewReader(data))
		if err != nil {
			return err
		}
		if status != http.StatusCreated {
			return fmt.Errorf("API error %d: %s", status, string(body))
		}
		fmt.Println("MCP server registered successfully.")
		return nil
	},
}

var mcpGetCmd = &cobra.Command{
	Use:   "get <server-id>",
	Short: "Get details of a registered MCP server",
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
		u.Path = fmt.Sprintf("/api/v1/mcp/servers/%s", url.PathEscape(args[0]))

		body, err := client.Get(cmd.Context(), u.String(), client.Options{TenantID: tenant, Token: optToken(cmd)})
		if err != nil {
			return err
		}

		outputFmt, _ := cmd.Flags().GetString("output")
		if outputFmt == "json" {
			fmt.Fprintln(os.Stdout, string(body))
			return nil
		}
		return printMCPSingle(body)
	},
}

var mcpHeartbeatCmd = &cobra.Command{
	Use:   "heartbeat <server-id>",
	Short: "Send a heartbeat for a registered MCP server",
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
		u.Path = fmt.Sprintf("/api/v1/mcp/servers/%s/heartbeat", url.PathEscape(args[0]))

		body, status, err := client.DoJSON(cmd.Context(), http.MethodPost, u.String(), client.Options{TenantID: tenant, Token: optToken(cmd)}, nil)
		if err != nil {
			return err
		}
		if status != http.StatusOK {
			return fmt.Errorf("API error %d: %s", status, string(body))
		}
		fmt.Printf("Heartbeat sent for server %q.\n", args[0])
		return nil
	},
}

var mcpDeregisterCmd = &cobra.Command{
	Use:   "deregister <server-id>",
	Short: "Deregister an MCP server",
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
		u.Path = fmt.Sprintf("/api/v1/mcp/servers/%s", url.PathEscape(args[0]))

		body, status, err := client.DoJSON(cmd.Context(), http.MethodDelete, u.String(), client.Options{TenantID: tenant, Token: optToken(cmd)}, nil)
		if err != nil {
			return err
		}
		if status == http.StatusNoContent {
			fmt.Printf("MCP server %q deregistered.\n", args[0])
			return nil
		}
		return fmt.Errorf("API error %d: %s", status, string(body))
	},
}

func printMCPSingle(body []byte) error {
	var response struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		fmt.Fprintln(os.Stdout, string(body))
		return nil
	}
	s := response.Data
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "ID:\t%s\n", strField(s, "id"))
	fmt.Fprintf(w, "Name:\t%s\n", strField(s, "name"))
	fmt.Fprintf(w, "Endpoint:\t%s\n", strField(s, "endpoint"))
	fmt.Fprintf(w, "Healthy:\t%v\n", s["healthy"])
	if c, ok := s["capabilities"].([]any); ok {
		parts := make([]string, 0, len(c))
		for _, v := range c {
			if str, ok := v.(string); ok {
				parts = append(parts, str)
			}
		}
		fmt.Fprintf(w, "Capabilities:\t%s\n", strings.Join(parts, ", "))
	}
	return w.Flush()
}

func printMCPTable(body []byte) error {
	var response struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		fmt.Fprintln(os.Stdout, string(body))
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tENDPOINT\tHEALTHY\tCAPABILITIES")
	for _, s := range response.Data {
		id := strField(s, "id")
		name := strField(s, "name")
		ep := strField(s, "endpoint")
		healthy := fmt.Sprintf("%v", s["healthy"])
		caps := ""
		if c, ok := s["capabilities"].([]any); ok {
			parts := make([]string, 0, len(c))
			for _, v := range c {
				if str, ok := v.(string); ok {
					parts = append(parts, str)
				}
			}
			caps = strings.Join(parts, ", ")
			if len(caps) > 40 {
				caps = caps[:40] + "…"
			}
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", id, name, ep, healthy, caps)
	}
	return w.Flush()
}

func init() {
	mcpRegisterCmd.Flags().String("id", "", "Server ID (required)")
	mcpRegisterCmd.Flags().String("name", "", "Server name (required)")
	mcpRegisterCmd.Flags().String("endpoint", "", "Server endpoint URL (required)")
	mcpRegisterCmd.Flags().String("capabilities", "", "Comma-separated list of tool names")
	_ = mcpRegisterCmd.MarkFlagRequired("id")
	_ = mcpRegisterCmd.MarkFlagRequired("name")
	_ = mcpRegisterCmd.MarkFlagRequired("endpoint")
	_ = mcpRegisterCmd.MarkFlagRequired("capabilities")

	mcpCmd.AddCommand(mcpListCmd, mcpGetCmd, mcpRegisterCmd, mcpDeregisterCmd, mcpHeartbeatCmd)
}
