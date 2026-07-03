package controlplane

import (
	"sync"

	"github.com/subzone/Agentctl/internal/entitlement"
)

// LicenseRecord is the server-side view of a Freemius license.
type LicenseRecord struct {
	LicenseKey   string
	Plan         entitlement.Plan
	Subject      string
	Entitlements []string
	Revoked      bool
	Version      int
}

// Store holds license records in memory (Phase 1; Postgres in production).
type Store struct {
	mu       sync.RWMutex
	licenses map[string]*LicenseRecord
	seen     map[string]bool // webhook idempotency keys
}

func NewStore() *Store {
	s := &Store{
		licenses: make(map[string]*LicenseRecord),
		seen:     make(map[string]bool),
	}
	s.seedSandboxLicenses()
	return s
}

func (s *Store) seedSandboxLicenses() {
	s.licenses["FS-PRO-SANDBOX-2026"] = &LicenseRecord{
		LicenseKey:   "FS-PRO-SANDBOX-2026",
		Plan:         entitlement.PlanPro,
		Subject:      "sandbox_pro",
		Entitlements: []string{"pro"},
		Version:      1,
	}
	s.licenses["FS-TEAM-SANDBOX-2026"] = &LicenseRecord{
		LicenseKey:   "FS-TEAM-SANDBOX-2026",
		Plan:         entitlement.PlanTeam,
		Subject:      "sandbox_team",
		Entitlements: []string{"pro", "team"},
		Version:      1,
	}
}

func (s *Store) GetLicense(key string) (*LicenseRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.licenses[key]
	if !ok || rec.Revoked {
		return nil, false
	}
	return rec, ok
}

func (s *Store) UpsertLicense(rec LicenseRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing := s.licenses[rec.LicenseKey]
	if existing != nil {
		rec.Version = existing.Version + 1
	}
	s.licenses[rec.LicenseKey] = &rec
}

func (s *Store) RevokeLicense(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if rec, ok := s.licenses[key]; ok {
		rec.Revoked = true
		rec.Version++
	}
}

func (s *Store) MarkWebhookSeen(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.seen[id] {
		return false
	}
	s.seen[id] = true
	return true
}
