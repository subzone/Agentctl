package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/spf13/cobra"

	"github.com/subzone/Agentctl/internal/controlplane"
	"github.com/subzone/Agentctl/internal/entitlement"
)

func newControlPlaneCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "controlplane",
		Short:  "Run the AgentCTL control plane (sandbox)",
		Hidden: true,
	}
	cmd.AddCommand(newControlPlaneServeCmd())
	return cmd
}

func newControlPlaneServeCmd() *cobra.Command {
	var addr string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the local control plane API on :8090",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if addr != "" {
				_ = os.Setenv("AGENTCTL_CP_ADDR", addr)
			}
			cfg, err := controlplane.LoadConfig()
			if err != nil {
				return err
			}
			srv, err := controlplane.NewServer(cfg)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Control plane listening on %s\n", cfg.Addr)
			fmt.Fprintf(out, "OpenAPI: controlplane/openapi.yaml\n")
			fmt.Fprintf(out, "Try: AGENTCTL_CONTROL_PLANE_URL=%s m license activate FS-PRO-SANDBOX-2026\n",
				entitlement.LocalControlPlaneURL())
			log.Printf("sandbox licenses: FS-PRO-SANDBOX-2026, FS-TEAM-SANDBOX-2026")
			return http.ListenAndServe(cfg.Addr, srv.Handler()) // #nosec G114 -- sandbox dev server only
		},
	}
	cmd.Flags().StringVar(&addr, "addr", ":8090", "listen address")
	return cmd
}
