package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/subzone/Agentctl/internal/controlplane"
)

func main() {
	cfg, err := controlplane.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}
	srv, err := controlplane.NewServer(cfg)
	if err != nil {
		log.Fatal(err)
	}
	addr := cfg.Addr
	log.Printf("AgentCTL control plane listening on %s", addr)
	log.Printf("Webhook secret: set AGENTCTL_FREEMIUS_WEBHOOK_SECRET (default: dev-webhook-secret)")
	log.Printf("Sandbox licenses: FS-PRO-SANDBOX-2026, FS-TEAM-SANDBOX-2026")
	if err := http.ListenAndServe(addr, srv.Handler()); err != nil { //nolint:gosec // local dev server
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
