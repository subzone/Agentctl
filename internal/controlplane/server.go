package controlplane

import (
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/subzone/Agentctl/internal/entitlement"
)

// Server is the control plane HTTP API.
type Server struct {
	cfg   Config
	store *Store
	pub   ed25519.PublicKey
}

// NewServer constructs a Server with an in-memory license store.
func NewServer(cfg Config) (*Server, error) {
	pub, err := entitlement.DevPublicKey()
	if err != nil {
		return nil, err
	}
	return &Server{cfg: cfg, store: NewStore(), pub: pub}, nil
}

// Handler returns the root http.Handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("POST /v1/auth/freemius/activate", s.handleActivate)
	mux.HandleFunc("POST /v1/auth/refresh", s.handleRefresh)
	mux.HandleFunc("GET /v1/entitlements/me", s.handleEntitlementsMe)
	mux.HandleFunc("GET /v1/packages", s.handlePackages)
	mux.HandleFunc("POST /v1/webhooks/freemius", s.handleFreemiusWebhook)
	mux.HandleFunc("POST /v1/broker/llm", s.handleBrokerNotImplemented)
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"version": s.cfg.Version,
	})
}

type activateBody struct {
	LicenseKey string `json:"license_key"`
	MachineID  string `json:"machine_id"`
}

func (s *Server) handleActivate(w http.ResponseWriter, r *http.Request) {
	var body activateBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	key := strings.TrimSpace(body.LicenseKey)
	rec, ok := s.store.GetLicense(key)
	if !ok {
		writeError(w, http.StatusNotFound, "license not found or revoked")
		return
	}
	token, state, err := IssueToken(rec, s.cfg.SigningKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, entitlementResponseFrom(state, token))
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	claims, err := s.parseBearer(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	subject := claims.Subject
	if subject == "" {
		subject = claims.RegisteredClaims.Subject
	}
	rec := &LicenseRecord{
		Plan:         claims.Plan,
		Subject:      subject,
		Entitlements: claims.Entitlements,
		Version:      claims.Ver + 1,
	}
	token, state, err := IssueToken(rec, s.cfg.SigningKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, entitlementResponseFrom(state, token))
}

func (s *Server) handleEntitlementsMe(w http.ResponseWriter, r *http.Request) {
	claims, err := s.parseBearer(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"sub":          claims.RegisteredClaims.Subject,
		"plan":         claims.Plan,
		"entitlements": claims.Entitlements,
		"packages":     claims.Packages,
		"ver":          claims.Ver,
	})
}

func (s *Server) handlePackages(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"packages": PackageCatalog()})
}

func (s *Server) handleFreemiusWebhook(w http.ResponseWriter, r *http.Request) {
	secret := strings.TrimSpace(r.Header.Get("X-Agentctl-Webhook-Secret"))
	if secret == "" || secret != s.cfg.WebhookSecret {
		writeError(w, http.StatusUnauthorized, "invalid webhook secret")
		return
	}
	var ev FreemiusWebhookEvent
	if err := json.NewDecoder(r.Body).Decode(&ev); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if err := ApplyFreemiusEvent(s.store, ev); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleBrokerNotImplemented(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusNotImplemented, "broker not available in Phase 1")
}

func (s *Server) parseBearer(r *http.Request) (*entitlement.Claims, error) {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(auth, "Bearer ") {
		return nil, fmt.Errorf("missing bearer token")
	}
	token := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	return entitlement.ParseToken(token, s.pub)
}

type apiEntitlementResponse struct {
	Token        string   `json:"token"`
	Plan         string   `json:"plan"`
	Entitlements []string `json:"entitlements"`
	Packages     []string `json:"packages"`
	Subject      string   `json:"subject"`
	ExpiresAt    string   `json:"expires_at"`
}

func entitlementResponseFrom(state entitlement.State, token string) apiEntitlementResponse {
	exp := ""
	if state.ExpiresAt > 0 {
		exp = time.Unix(state.ExpiresAt, 0).UTC().Format(time.RFC3339)
	}
	return apiEntitlementResponse{
		Token:        token,
		Plan:         string(state.Plan),
		Entitlements: state.EffectiveEntitlements(),
		Packages:     state.Packages,
		Subject:      state.Subject,
		ExpiresAt:    exp,
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
