package controlplane

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/subzone/Agentctl/internal/entitlement"

	_ "modernc.org/sqlite"
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

// Store persists license and webhook-idempotency state to a local SQLite
// file. A restart or redeploy must not lose customer license records, which
// an in-memory store would.
type Store struct {
	db *sql.DB
}

// NewStore opens (creating if needed) the SQLite-backed license store at
// path. When seedSandbox is true, two fixed sandbox licenses are inserted so
// `m controlplane serve` works out of the box for local testing — this must
// be false in production.
func NewStore(path string, seedSandbox bool) (*Store, error) {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create db dir: %w", err)
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // single-writer SQLite file
	if _, err := db.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;`); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS licenses (
			license_key  TEXT PRIMARY KEY,
			plan         TEXT NOT NULL,
			subject      TEXT NOT NULL,
			entitlements TEXT NOT NULL,
			revoked      INTEGER NOT NULL DEFAULT 0,
			version      INTEGER NOT NULL DEFAULT 1,
			updated_at   TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS webhook_events (
			event_id    TEXT PRIMARY KEY,
			received_at TEXT NOT NULL
		);
	`); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate schema: %w", err)
	}
	s := &Store{db: db}
	if seedSandbox {
		if err := s.seedSandboxLicenses(); err != nil {
			db.Close()
			return nil, err
		}
	}
	return s, nil
}

// Close releases the underlying database handle.
func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) seedSandboxLicenses() error {
	seed := []LicenseRecord{
		{LicenseKey: "FS-PRO-SANDBOX-2026", Plan: entitlement.PlanPro, Subject: "sandbox_pro", Entitlements: []string{"pro"}, Version: 1},
		{LicenseKey: "FS-TEAM-SANDBOX-2026", Plan: entitlement.PlanTeam, Subject: "sandbox_team", Entitlements: []string{"pro", "team"}, Version: 1},
	}
	for _, rec := range seed {
		if _, ok, err := s.getLicenseRaw(rec.LicenseKey); err != nil {
			return err
		} else if ok {
			continue // don't clobber a record already touched by a real webhook
		}
		if err := s.insertLicense(rec); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) GetLicense(key string) (*LicenseRecord, bool) {
	rec, ok, err := s.getLicenseRaw(key)
	if err != nil || !ok || rec.Revoked {
		return nil, false
	}
	return rec, true
}

func (s *Store) getLicenseRaw(key string) (*LicenseRecord, bool, error) {
	row := s.db.QueryRow(`SELECT license_key, plan, subject, entitlements, revoked, version FROM licenses WHERE license_key = ?`, key)
	var rec LicenseRecord
	var entsJSON string
	var revoked int
	if err := row.Scan(&rec.LicenseKey, &rec.Plan, &rec.Subject, &entsJSON, &revoked, &rec.Version); err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}
	rec.Revoked = revoked != 0
	_ = json.Unmarshal([]byte(entsJSON), &rec.Entitlements)
	return &rec, true, nil
}

func (s *Store) insertLicense(rec LicenseRecord) error {
	entsJSON, _ := json.Marshal(rec.Entitlements)
	_, err := s.db.Exec(`
		INSERT INTO licenses (license_key, plan, subject, entitlements, revoked, version, updated_at)
		VALUES (?, ?, ?, ?, 0, ?, ?)`,
		rec.LicenseKey, string(rec.Plan), rec.Subject, string(entsJSON), rec.Version, time.Now().UTC().Format(time.RFC3339))
	return err
}

// UpsertLicense inserts or updates a license record (idempotent per webhook
// delivery), bumping the version on update so cached client entitlements
// know to refresh.
func (s *Store) UpsertLicense(rec LicenseRecord) error {
	existing, ok, err := s.getLicenseRaw(rec.LicenseKey)
	if err != nil {
		return err
	}
	if ok {
		rec.Version = existing.Version + 1
	} else if rec.Version == 0 {
		rec.Version = 1
	}
	entsJSON, _ := json.Marshal(rec.Entitlements)
	_, err = s.db.Exec(`
		INSERT INTO licenses (license_key, plan, subject, entitlements, revoked, version, updated_at)
		VALUES (?, ?, ?, ?, 0, ?, ?)
		ON CONFLICT(license_key) DO UPDATE SET
			plan = excluded.plan,
			subject = excluded.subject,
			entitlements = excluded.entitlements,
			revoked = 0,
			version = excluded.version,
			updated_at = excluded.updated_at`,
		rec.LicenseKey, string(rec.Plan), rec.Subject, string(entsJSON), rec.Version, time.Now().UTC().Format(time.RFC3339))
	return err
}

// RevokeLicense marks a license revoked without deleting its history.
func (s *Store) RevokeLicense(key string) error {
	_, err := s.db.Exec(`UPDATE licenses SET revoked = 1, version = version + 1, updated_at = ? WHERE license_key = ?`,
		time.Now().UTC().Format(time.RFC3339), key)
	return err
}

// MarkWebhookSeen records a webhook event id and reports whether this is the
// first time it's been seen (false means it's a duplicate delivery — skip
// re-applying it).
func (s *Store) MarkWebhookSeen(id string) (bool, error) {
	_, err := s.db.Exec(`INSERT INTO webhook_events (event_id, received_at) VALUES (?, ?)`,
		id, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		if isUniqueConstraintErr(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func isUniqueConstraintErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "constraint failed")
}
