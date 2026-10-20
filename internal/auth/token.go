package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// AccessTokenTTL is the access token lifetime (SPEC-01 NFR-02).
const AccessTokenTTL = 15 * time.Minute

// Issuer is the iss claim of every token this service signs.
const Issuer = "pharmachat-api"

// MinSigningKeyLen is the shortest JWT_SIGNING_KEY accepted (SPEC-01 §5).
const MinSigningKeyLen = 32

// Claims is the access token payload.
type Claims struct {
	UserID   uuid.UUID `json:"user_id"`
	TenantID uuid.UUID `json:"tenant_id"`
	Roles    []string  `json:"roles"`
	jwt.RegisteredClaims
}

// TokenIssuer signs and verifies HS256 access tokens.
type TokenIssuer struct {
	key    []byte
	now    func() time.Time
	parser *jwt.Parser
}

// NewTokenIssuer returns an issuer for key. now is the clock, time.Now outside tests.
func NewTokenIssuer(key []byte, now func() time.Time) (*TokenIssuer, error) {
	if len(key) < MinSigningKeyLen {
		return nil, fmt.Errorf("JWT_SIGNING_KEY must be at least %d bytes, got %d", MinSigningKeyLen, len(key))
	}
	return &TokenIssuer{
		key: key,
		now: now,
		parser: jwt.NewParser(
			jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
			jwt.WithIssuer(Issuer),
			jwt.WithExpirationRequired(),
			jwt.WithTimeFunc(now),
		),
	}, nil
}

// Issue signs an access token for the user.
func (ti *TokenIssuer) Issue(userID, tenantID uuid.UUID, roles []string) (string, error) {
	now := ti.now()
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		UserID:   userID,
		TenantID: tenantID,
		Roles:    roles,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    Issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(AccessTokenTTL)),
		},
	}).SignedString(ti.key)
	if err != nil {
		return "", fmt.Errorf("sign access token: %w", err)
	}
	return token, nil
}

// Parse verifies the signature, algorithm, issuer and expiry, and returns the claims.
func (ti *TokenIssuer) Parse(token string) (*Claims, error) {
	var c Claims
	if _, err := ti.parser.ParseWithClaims(token, &c, func(*jwt.Token) (any, error) { return ti.key, nil }); err != nil {
		return nil, fmt.Errorf("parse access token: %w", err)
	}
	if c.UserID == uuid.Nil || c.TenantID == uuid.Nil {
		return nil, errors.New("parse access token: user_id or tenant_id is missing")
	}
	return &c, nil
}
