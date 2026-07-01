package desktop

import "github.com/subzone/Agentctl/internal/entitlement"

// EntitlementInfo is the user's plan state for the desktop UI.
type EntitlementInfo struct {
	Plan         string   `json:"plan"`
	Entitlements []string `json:"entitlements"`
	Packages     []string `json:"packages"`
	LicenseHint  string   `json:"licenseHint"`
	IsPro        bool     `json:"isPro"`
}

// PackageOffer describes a sellable bundle and lock state.
type PackageOffer struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Entitlements []string `json:"entitlements"`
	Locked       bool     `json:"locked"`
	Installed    bool     `json:"installed"`
}

// GetEntitlement returns the current license/plan state.
func (a *App) GetEntitlement() EntitlementInfo {
	s, err := entitlement.Load()
	if err != nil {
		s = entitlement.Default()
	}
	return EntitlementInfo{
		Plan:         string(s.Plan),
		Entitlements: s.EffectiveEntitlements(),
		Packages:     s.Packages,
		LicenseHint:  s.LicenseHint,
		IsPro:        s.HasEntitlement("pro"),
	}
}

// ActivateLicense activates a license key (dev keys until control plane ships).
func (a *App) ActivateLicense(key string) (EntitlementInfo, error) {
	s, err := entitlement.ActivateDevKey(key)
	if err != nil {
		return EntitlementInfo{}, err
	}
	if err := entitlement.Save(s); err != nil {
		return EntitlementInfo{}, err
	}
	return a.GetEntitlement(), nil
}

// InstallPackage installs a curated package if entitled.
func (a *App) InstallPackage(name string) error {
	return installPackageByName(name)
}
