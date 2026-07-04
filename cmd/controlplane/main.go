package main

import (
	"log"

	"github.com/subzone/Agentctl/internal/controlplane"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := controlplane.LoadConfig()
	if err != nil {
		return err
	}
	srv, err := controlplane.NewServer(cfg)
	if err != nil {
		return err
	}
	defer srv.Close()

	addr := cfg.Addr
	log.Printf("AgentCTL control plane listening on %s (env=%s, db=%s)", addr, cfg.Env, cfg.DBPath)
	if !cfg.IsProduction() {
		log.Printf("Webhook secret: set AGENTCTL_FREEMIUS_WEBHOOK_SECRET (default: dev-webhook-secret)")
		log.Printf("Sandbox licenses: FS-PRO-SANDBOX-2026, FS-TEAM-SANDBOX-2026")
	}
	return controlplane.Listen(addr, srv.Handler())
}
