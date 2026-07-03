package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/subzone/Agentctl/internal/entitlement"
)

func newLicenseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "license",
		Short: "View or activate your AgentCTL plan",
	}
	cmd.AddCommand(newLicenseStatusCmd())
	cmd.AddCommand(newLicenseActivateCmd())
	cmd.AddCommand(newLicenseRefreshCmd())
	cmd.AddCommand(newLicenseClearCmd())
	return cmd
}

func newLicenseStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current plan and entitlements",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := entitlement.Load()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Plan:         %s\n", s.Plan)
			if s.LicenseHint != "" {
				fmt.Fprintf(out, "License:      %s\n", s.LicenseHint)
			}
			if s.Source != "" {
				fmt.Fprintf(out, "Source:       %s\n", s.Source)
			}
			if cp := entitlement.ControlPlaneURL(); cp != "" {
				fmt.Fprintf(out, "Control plane: %s\n", cp)
			}
			ents := s.EffectiveEntitlements()
			fmt.Fprintf(out, "Entitlements: %v\n", ents)
			if len(s.Packages) > 0 {
				fmt.Fprintf(out, "Installed:    %v\n", s.Packages)
			}
			if s.Plan == entitlement.PlanFree {
				fmt.Fprintln(out)
				fmt.Fprintln(out, "Upgrade: activate a license with `m license activate <key>`")
				fmt.Fprintln(out, "Dev test key: AGENTCTL-PRO-DEV-2026")
				fmt.Fprintln(out, "Sandbox: AGENTCTL_CONTROL_PLANE_URL=http://localhost:8090 m controlplane serve")
			}
			return nil
		},
	}
}

func newLicenseActivateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "activate <license-key>",
		Short: "Activate a Pro, Team, or Enterprise license",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := entitlement.Activate(args[0])
			if err != nil {
				return err
			}
			if err := entitlement.Save(s); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "✓ Activated %s plan (entitlements: %v)\n", s.Plan, s.EffectiveEntitlements())
			if s.Source == "control-plane" {
				fmt.Fprintln(cmd.OutOrStdout(), "Entitlement signed by control plane (JWT cached locally).")
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Install Pro packages: m packages install pro-dev")
			return nil
		},
	}
}

func newLicenseRefreshCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "refresh",
		Short: "Refresh signed entitlement from the control plane",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := entitlement.Refresh()
			if err != nil {
				return err
			}
			if err := entitlement.Save(s); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "✓ Refreshed %s plan (entitlements: %v)\n", s.Plan, s.EffectiveEntitlements())
			return nil
		},
	}
}

func newLicenseClearCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "clear",
		Short:  "Revert to free plan (remove local license file)",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := entitlement.Clear(); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Reverted to free plan.")
			return nil
		},
	}
}
