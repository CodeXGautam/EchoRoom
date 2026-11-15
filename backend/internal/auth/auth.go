// Package auth verifies access tokens and exposes the caller's identity.
//
// In production tokens are Supabase Auth JWTs. The Go service never sees
// passwords: the browser signs in with supabase-js (anonymously or with a
// real account) and sends the resulting access token as a Bearer token.
package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

// Identity is who the caller is, as proven by their token.
type Identity struct {
	UserID      string
	IsAnonymous bool
}

// Verifier turns a raw bearer token into an Identity.
type Verifier interface {
	Verify(ctx context.Context, token string) (Identity, error)
}

var (
	ErrMissingToken = errors.New("missing bearer token")
	ErrInvalidToken = errors.New("invalid token")
)

type ctxKey struct{}

// FromContext returns the identity stored by Middleware.
func FromContext(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(ctxKey{}).(Identity)
	return id, ok
}

// Middleware rejects requests without a valid "Authorization: Bearer <token>"
// header and stores the Identity in the request context.
func Middleware(v Verifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := bearerToken(r)
			if !ok {
				http.Error(w, ErrMissingToken.Error(), http.StatusUnauthorized)
				return
			}
			id, err := v.Verify(r.Context(), token)
			if err != nil {
				http.Error(w, ErrInvalidToken.Error(), http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, id)))
		})
	}
}

func bearerToken(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(h) <= len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return "", false
	}
	return strings.TrimSpace(h[len(prefix):]), true
}
