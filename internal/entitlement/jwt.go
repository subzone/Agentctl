package entitlement

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// DevPrivateKeyHex is the Ed25519 seed for sandbox JWT signing (control plane dev only).
const DevPrivateKeyHex = "d8d14ea39788842c5151355fc582afe30ff498e4afbead6fecaeb8e443ce13da"

const jwtIssuer = "agentctl-control-plane"
const defaultJWTTTL = 30 * 24 * time.Hour

// Claims is the signed entitlement payload exchanged with the control plane.
type Claims struct {
	Plan         Plan     `json:"plan"`
	Entitlements []string `json:"entitlements,omitempty"`
	Packages     []string `json:"packages,omitempty"`
	Ver          int      `json:"ver,omitempty"`
	Subject      string   `json:"-"`
	jwt.RegisteredClaims
}

// SignClaims issues a JWT for the control plane (server-side).
func SignClaims(claims Claims, privateKey ed25519.PrivateKey, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		ttl = defaultJWTTTL
	}
	now := time.Now()
	subject := claims.Subject
	if subject == "" {
		subject = "user"
	}
	claims.RegisteredClaims = jwt.RegisteredClaims{
		Issuer:    jwtIssuer,
		Subject:   subject,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	return token.SignedString(privateKey)
}

// ParseToken verifies a JWT and returns claims using the given public key.
func ParseToken(tokenString string, publicKey ed25519.PublicKey) (*Claims, error) {
	tokenString = strings.TrimSpace(tokenString)
	if tokenString == "" {
		return nil, fmt.Errorf("empty token")
	}
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodEdDSA {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return publicKey, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	if claims.Issuer != "" && claims.Issuer != jwtIssuer {
		return nil, fmt.Errorf("invalid issuer")
	}
	return claims, nil
}

// DevPublicKey returns the sandbox verification key derived from the dev seed.
func DevPublicKey() (ed25519.PublicKey, error) {
	priv, err := DevPrivateKey()
	if err != nil {
		return nil, err
	}
	return priv.Public().(ed25519.PublicKey), nil
}

// DevPrivateKey returns the sandbox signing key (control plane dev only).
func DevPrivateKey() (ed25519.PrivateKey, error) {
	b, err := hex.DecodeString(DevPrivateKeyHex)
	if err != nil {
		return nil, err
	}
	switch len(b) {
	case ed25519.SeedSize:
		return ed25519.NewKeyFromSeed(b), nil
	case ed25519.PrivateKeySize:
		return ed25519.PrivateKey(b), nil
	default:
		return nil, fmt.Errorf("invalid dev private key length")
	}
}

// StateFromClaims maps verified JWT claims to persisted local state.
func StateFromClaims(c *Claims, token string) State {
	if c == nil {
		return Default()
	}
	subject := c.Subject
	if subject == "" {
		subject = c.RegisteredClaims.Subject
	}
	plan := c.Plan
	if plan == "" {
		plan = PlanFree
	}
	var expires int64
	if c.ExpiresAt != nil {
		expires = c.ExpiresAt.Unix()
	}
	return State{
		Plan:         plan,
		Entitlements: append([]string(nil), c.Entitlements...),
		Packages:     append([]string(nil), c.Packages...),
		Subject:      subject,
		LicenseHint:  "jwt",
		ActivatedAt:  time.Now().Unix(),
		ExpiresAt:    expires,
		Source:       "control-plane",
		Token:        token,
	}
}

// ApplyToken verifies a JWT with the dev public key and returns state.
func ApplyToken(token string) (State, error) {
	pub, err := DevPublicKey()
	if err != nil {
		return State{}, err
	}
	claims, err := ParseToken(token, pub)
	if err != nil {
		return State{}, fmt.Errorf("verify entitlement: %w", err)
	}
	return StateFromClaims(claims, token), nil
}
