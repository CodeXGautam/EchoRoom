package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

// supabaseClaims is the subset of Supabase access-token claims we use.
// See https://supabase.com/docs/guides/auth/jwt-fields
type supabaseClaims struct {
	jwt.RegisteredClaims
	Role        string `json:"role"`
	IsAnonymous bool   `json:"is_anonymous"`
}

// SupabaseVerifier checks Supabase access tokens.
//
// Projects using JWT signing keys (asymmetric, ES256/RS256) are verified
// against the public JWKS endpoint, which keyfunc caches and refreshes in the
// background. Projects still on the legacy shared secret (HS256) can pass it
// as legacySecret.
type SupabaseVerifier struct {
	parser       *jwt.Parser
	jwks         keyfunc.Keyfunc
	legacySecret []byte
}

// NewSupabaseVerifier starts the JWKS refresh goroutine; it stops when ctx ends.
func NewSupabaseVerifier(ctx context.Context, supabaseURL, legacySecret string) (*SupabaseVerifier, error) {
	if supabaseURL == "" {
		return nil, errors.New("supabase url is required")
	}
	jwksURL := supabaseURL + "/auth/v1/.well-known/jwks.json"
	jwks, err := keyfunc.NewDefaultCtx(ctx, []string{jwksURL})
	if err != nil {
		return nil, fmt.Errorf("load supabase jwks: %w", err)
	}

	algs := []string{"ES256", "RS256"}
	if legacySecret != "" {
		algs = append(algs, "HS256")
	}

	return &SupabaseVerifier{
		parser: jwt.NewParser(
			jwt.WithValidMethods(algs),
			jwt.WithAudience("authenticated"),
			jwt.WithIssuer(supabaseURL+"/auth/v1"),
			jwt.WithExpirationRequired(),
		),
		jwks:         jwks,
		legacySecret: []byte(legacySecret),
	}, nil
}

func (v *SupabaseVerifier) Verify(_ context.Context, token string) (Identity, error) {
	var claims supabaseClaims
	_, err := v.parser.ParseWithClaims(token, &claims, v.keyFor)
	if err != nil {
		return Identity{}, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	if claims.Subject == "" || claims.Role != "authenticated" {
		return Identity{}, fmt.Errorf("%w: missing subject or wrong role", ErrInvalidToken)
	}
	return Identity{UserID: claims.Subject, IsAnonymous: claims.IsAnonymous}, nil
}

// keyFor picks the verification key based on the token's algorithm.
// WithValidMethods already rejected anything outside the allowed list, so an
// attacker cannot downgrade an asymmetric token to HS256 unless a legacy
// secret was configured.
func (v *SupabaseVerifier) keyFor(t *jwt.Token) (any, error) {
	if t.Method.Alg() == "HS256" {
		if len(v.legacySecret) == 0 {
			return nil, errors.New("hs256 not enabled")
		}
		return v.legacySecret, nil
	}
	return v.jwks.Keyfunc(t)
}
