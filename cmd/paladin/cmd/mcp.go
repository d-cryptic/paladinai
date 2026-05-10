package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"

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
		apiURL, _ := cmd.Flags().GetString("api-url")
		body, err := apiGet(cmd, apiURL+"/api/v1/mcp/servers", tenant)
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
		data, _ := json.Marshal(payload)

		apiURL, _ := cmd.Flags().GetString("api-url")
		req, _ := http.NewRequestWithContext(cmd.Context(), http.MethodPost,
			apiURL+"/api/v1/mcp/servers", bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", tenant)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("request failed: %w", err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode != http.StatusCreated {
			return fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
		}
		fmt.Println("MCP server registered successfully.")
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
		apiURL, _ := cmd.Flags().GetString("api-url")
		url := fmt.Sprintf("%s/api/v1/mcp/servers/%s", apiURL, args[0])

		req, _ := http.NewRequestWithContext(cmd.Context(), http.MethodDelete, url, nil)
		req.Header.Set("X-Tenant-ID", tenant)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("request failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNoContent {
			fmt.Printf("MCP server %q deregistered.\n", args[0])
			return nil
		}
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	},
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

func apiGet(cmd *cobra.Command, url, tenant string) ([]byte, error) {
	req, err := http.NewRequestWithContext(cmd.Context(), http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("X-Tenant-ID", tenant)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
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

	mcpCmd.AddCommand(mcpListCmd, mcpRegisterCmd, mcpDeregisterCmd)
}
