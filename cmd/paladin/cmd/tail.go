package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
)

const (
	wsPongWait    = 60 * time.Second
	wsPingPeriod  = (wsPongWait * 9) / 10
	wsDialTimeout = 10 * time.Second
	wsCloseGrace  = 2 * time.Second
	wsMaxBackoff  = 30 * time.Second
)

// AlertEvent is the JSON payload pushed by the server over the WebSocket.
type AlertEvent struct {
	Fingerprint   string    `json:"fingerprint"`
	Severity      string    `json:"severity"`
	Status        string    `json:"status"`
	Service       string    `json:"service"`
	Title         string    `json:"title"`
	CorrelationID string    `json:"correlation_id"`
	StartsAt      time.Time `json:"starts_at"`
}

var tailCmd = &cobra.Command{
	Use:   "tail",
	Short: "Stream live alert events from the server",
	Long: `paladin tail opens a WebSocket connection and streams alert events as they arrive.

Examples:
  paladin tail
  paladin tail --severity p1
  paladin tail --service payments-api
  paladin tail --since 30m

Press Ctrl+C to disconnect.`,
	RunE: runTail,
}

func runTail(cmd *cobra.Command, _ []string) error {
	tenant, err := requireTenant(cmd)
	if err != nil {
		return err
	}

	wsURL, err := alertWSURL(apiURL(cmd), cmd)
	if err != nil {
		return err
	}

	// Token precedence: --token flag > PALADIN_TOKEN env > keychain.
	token := optToken(cmd)
	if token == "" {
		token = os.Getenv("PALADIN_TOKEN")
	}
	if token == "" {
		if cfg, _ := loadConfig(); cfg != nil {
			stored, err := loadStoredToken(cfg, cfg.AuthEndpoint)
			if err != nil {
				return err
			}
			token = stored
		}
	}

	header := tailAuthHeaders(tenant, token)

	ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TIME\tSEVERITY\tSERVICE\tSTATUS\tALERT")
	w.Flush()

	fmt.Fprintf(os.Stderr, "Connecting to %s (tenant: %s) — Ctrl+C to quit\n", wsURL, tenant)

	return connectAndStream(ctx, wsURL, header, w)
}

func tailAuthHeaders(tenant, token string) http.Header {
	header := http.Header{}
	if token != "" {
		header.Set("Authorization", "Bearer "+token)
		return header
	}
	header.Set("X-Tenant-ID", tenant)
	return header
}

// connectAndStream connects to the WebSocket URL and writes arriving events to w.
// It reconnects with exponential backoff on unexpected disconnects.
func connectAndStream(ctx context.Context, wsURL string, header http.Header, w *tabwriter.Writer) error {
	backoff := time.Second
	dialer := websocket.Dialer{HandshakeTimeout: wsDialTimeout}

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		conn, _, err := dialer.DialContext(ctx, wsURL, header)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			fmt.Fprintf(os.Stderr, "connect error: %v — retrying in %s\n", err, backoff)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(backoff):
			}
			backoff = min(backoff*2, wsMaxBackoff)
			continue
		}

		backoff = time.Second // reset on successful connect
		fmt.Fprintln(os.Stderr, "connected")

		reconnect := streamMessages(ctx, conn, w)
		conn.Close()

		if !reconnect || ctx.Err() != nil {
			return nil
		}
		fmt.Fprintf(os.Stderr, "disconnected — retrying in %s\n", backoff)
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, wsMaxBackoff)
	}
}

// streamMessages reads events from conn until ctx is cancelled or the connection
// closes unexpectedly. Returns true if a reconnect should be attempted.
func streamMessages(ctx context.Context, conn *websocket.Conn, w *tabwriter.Writer) bool {
	// Extend read deadline on each pong to detect half-open connections.
	conn.SetReadDeadline(time.Now().Add(wsPongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(wsPongWait))
	})

	// Ping ticker + graceful-close goroutine.
	ticker := time.NewTicker(wsPingPeriod)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ctx.Done():
				// Signal the peer we are closing; set a short read deadline so
				// ReadMessage unblocks quickly rather than waiting for peer ACK.
				_ = conn.WriteControl(
					websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
					time.Now().Add(wsCloseGrace),
				)
				conn.SetReadDeadline(time.Now().Add(wsCloseGrace))
				return
			case <-ticker.C:
				if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(wsCloseGrace)); err != nil {
					return
				}
			}
		}
	}()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			select {
			case <-ctx.Done():
				return false // intentional close
			default:
				return true // unexpected — reconnect
			}
		}
		var ev AlertEvent
		if err := json.Unmarshal(msg, &ev); err != nil {
			continue // skip malformed frames
		}
		printEvent(w, ev)
	}
}

// printEvent writes a single alert event row to w and flushes.
func printEvent(w *tabwriter.Writer, ev AlertEvent) {
	ts := ev.StartsAt.Local().Format("15:04:05")
	if ev.StartsAt.IsZero() {
		ts = time.Now().Local().Format("15:04:05")
	}
	sev := strings.ToUpper(ev.Severity)
	svc := ev.Service
	if len(svc) > 24 {
		svc = svc[:24] + "..."
	}
	title := ev.Title
	if len(title) > 36 {
		title = title[:36] + "..."
	}
	fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", ts, sev, svc, ev.Status, title)
	w.Flush()
}

// alertWSURL converts the HTTP api base URL into a WebSocket alerts URL and
// appends filter query parameters from flags.
func alertWSURL(apiBase string, cmd *cobra.Command) (string, error) {
	u, err := url.Parse(apiBase)
	if err != nil {
		return "", fmt.Errorf("invalid api-url: %w", err)
	}
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	case "http":
		u.Scheme = "ws"
	default:
		return "", fmt.Errorf("unsupported scheme %q", u.Scheme)
	}
	u.Path = "/v2/ws/alerts"

	q := u.Query()
	if sev, _ := cmd.Flags().GetString("severity"); sev != "" {
		q.Set("severity", sev)
	}
	if svc, _ := cmd.Flags().GetString("service"); svc != "" {
		q.Set("service", svc)
	}
	if since, _ := cmd.Flags().GetString("since"); since != "" {
		q.Set("since", since)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func init() {
	tailCmd.Flags().String("severity", "", "Filter by severity (e.g. p1,p2)")
	tailCmd.Flags().String("service", "", "Filter by service name")
	tailCmd.Flags().String("since", "", "Replay alerts from duration ago (e.g. 30m, 1h)")

	rootCmd.AddCommand(tailCmd)
}
