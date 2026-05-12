package auth

import (
	"crypto/subtle"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// SigningKey is one entry in the rotating key set. The newest key signs new
// tokens; all keys verify (until they age out and are removed from config).
type SigningKey struct {
	ID     string // kid header value; short, opaque
	Secret []byte // HMAC secret, ≥ 32 bytes recommended
}

// Issuer issues access tokens. Construct via NewIssuer.
type Issuer struct {
	keys      []SigningKey // index 0 is the active signer
	issuer    string
	audience  string
	ttl       time.Duration
}

// NewIssuer builds an Issuer. The first key in `keys` becomes the active
// signing key; remaining keys remain valid for verification only.
//
// Returns an error if any secret is shorter than 32 bytes (HS256 advisory)
// or if `keys` is empty.
func NewIssuer(keys []SigningKey, issuer, audience string, ttl time.Duration) (*Issuer, error) {
	if len(keys) == 0 {
		return nil, fmt.Errorf("auth: no signing keys configured")
	}
	if ttl <= 0 || ttl > time.Hour {
		return nil, fmt.Errorf("auth: access token ttl must be (0, 1h]; got %s", ttl)
	}
	for _, k := range keys {
		if len(k.Secret) < 32 {
			return nil, fmt.Errorf("auth: signing key %q is shorter than 32 bytes", k.ID)
		}
	}
	return &Issuer{keys: keys, issuer: issuer, audience: audience, ttl: ttl}, nil
}

// AccessClaims is the JWT body for access tokens. Field tags follow
// RFC 7519. Custom claims live under the Portal namespace.
type AccessClaims struct {
	jwt.RegisteredClaims
	TokenVersion int      `json:"tv"`
	Roles        []string `json:"roles,omitempty"`
	Email        string   `json:"email,omitempty"`
	Name         string   `json:"name,omitempty"`
}

// IssueInput is the data caller provides to mint a new access token.
type IssueInput struct {
	UserID       uuid.UUID
	Email        string
	DisplayName  string
	Roles        []string
	TokenVersion int
}

// Issue mints a signed access token for the given user.
func (i *Issuer) Issue(in IssueInput) (string, error) {
	if in.UserID == uuid.Nil {
		return "", fmt.Errorf("auth: user id required")
	}
	now := time.Now().UTC()
	jti, err := uuid.NewRandom()
	if err != nil {
		return "", fmt.Errorf("auth: jti gen: %w", err)
	}
	claims := AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    i.issuer,
			Subject:   in.UserID.String(),
			Audience:  jwt.ClaimStrings{i.audience},
			ExpiresAt: jwt.NewNumericDate(now.Add(i.ttl)),
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        jti.String(),
		},
		TokenVersion: in.TokenVersion,
		Roles:        in.Roles,
		Email:        in.Email,
		Name:         in.DisplayName,
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tok.Header["kid"] = i.keys[0].ID
	signed, err := tok.SignedString(i.keys[0].Secret)
	if err != nil {
		return "", fmt.Errorf("auth: sign: %w", err)
	}
	return signed, nil
}

// Verifier validates incoming tokens. It accepts any key in the rotation set,
// but only HS256.
type Verifier struct {
	keys     map[string]SigningKey
	issuer   string
	audience string
}

// NewVerifier builds a Verifier from a key set. Verification is
// constant-time against the registered keys; the kid header selects which.
func NewVerifier(keys []SigningKey, issuer, audience string) (*Verifier, error) {
	if len(keys) == 0 {
		return nil, fmt.Errorf("auth: no verification keys configured")
	}
	m := make(map[string]SigningKey, len(keys))
	for _, k := range keys {
		m[k.ID] = k
	}
	return &Verifier{keys: m, issuer: issuer, audience: audience}, nil
}

// Verify parses + validates a bearer token. It does NOT consult the database;
// callers must additionally check token_version and disabled_at against the
// user record before granting access.
func (v *Verifier) Verify(raw string) (*AccessClaims, error) {
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithIssuer(v.issuer),
		jwt.WithAudience(v.audience),
		jwt.WithExpirationRequired(),
	)
	claims := &AccessClaims{}
	tok, err := parser.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != "HS256" {
			return nil, ErrUnsupportedAlg
		}
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, ErrUnknownKey
		}
		k, ok := v.keys[kid]
		if !ok {
			return nil, ErrUnknownKey
		}
		// constant-time access — protects against side-channel kid probing
		_ = subtle.ConstantTimeCompare([]byte(kid), []byte(k.ID))
		return k.Secret, nil
	})
	if err != nil {
		switch {
		case isJWTErr(err, jwt.ErrTokenExpired):
			return nil, ErrTokenExpired
		case isJWTErr(err, jwt.ErrSignatureInvalid),
			isJWTErr(err, jwt.ErrTokenMalformed),
			isJWTErr(err, jwt.ErrTokenSignatureInvalid),
			isJWTErr(err, jwt.ErrTokenUnverifiable):
			return nil, ErrTokenInvalid
		}
		return nil, ErrTokenInvalid
	}
	if !tok.Valid {
		return nil, ErrTokenInvalid
	}
	return claims, nil
}

func isJWTErr(err, target error) bool {
	for e := err; e != nil; {
		if e == target {
			return true
		}
		u, ok := e.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		e = u.Unwrap()
	}
	return false
}
