package cmd

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"runtime"
	"time"

	"github.com/duggal1/Sapphire-cli/internal/version"
	"github.com/spf13/cobra"
)

// start records the process start time for uptime calculation.
var start = time.Now()

var pingCmd = &cobra.Command{
	Use:    "ping",
	Short:  "Return a JSON health ping",
	Long:   "Return a small JSON health check with agent identity, status, uptime, host info, and version details.",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		hostname, err := os.Hostname()
		if err != nil {
			hostname = "unknown"
		}

		uptimeMs := time.Since(start).Milliseconds()

		// Try to fetch an IP for the current machine for display purposes.
		ipv4 := "127.0.0.1"
		if addrs, err := net.InterfaceAddrs(); err == nil && len(addrs) > 0 {
			for _, addr := range addrs {
				if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
					ipv4 = ipnet.IP.String()
					break
				}
			}
		}

		out := map[string]interface{}{
			"agent_id":    "sapphire-cli",
			"status":      "ok",
			"uptime_ms":   uptimeMs,
			"hostname":    hostname,
			"ip_address":  ipv4,
			"go_version":  runtime.Version(),
			"app_version": version.Display(),
		}

		bts, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal ping response: %w", err)
		}

		fmt.Println(string(bts))
		return nil
	},
}
