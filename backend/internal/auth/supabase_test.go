package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// fakeSupabase serves a JWKS for one ES256 key, like a real project does.
func fakeSupabase(t *testing.T) (*httptest.Server, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pad := func(b []byte) []byte { // P-256 coordinates are 32 bytes
		out := make([]byte, 32)
		copy(out[32-len(b):], b)
		return out
	}
	jwks := map[string]any{"keys": []map[string]string{{
		"kty": "EC", "crv": "P-256", "alg": "ES256", "use": "sig", "kid": "test-key",
		"x": base64.RawURLEncoding.EncodeToString(pad(key.X.Bytes())),
		"y": base64.RawURLEncoding.EncodeToString(pad(key.Y.Bytes())),
	}}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/v1/.well-known/jwks.json" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(jwks)
	}))
	t.Cleanup(srv.Close)
	return srv, key
}

func sign(t *testing.T, key *ecdsa.PrivateKey, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	tok.Header["kid"] = "test-key"
	s, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSupabaseVerifier(t *testing.T) {
	srv, key := fakeSupabase(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	v, err := NewSupabaseVerifier(ctx, srv.URL, "")
	if err != nil {
		t.Fatal(err)
	}

	good := jwt.MapClaims{
		"sub": "user-123", "aud": "authenticated", "role": "authenticated",
		"iss": srv.URL + "/auth/v1", "exp": time.Now().Add(time.Hour).Unix(),
		"is_anonymous": true,
	}
	with := func(k string, val any) jwt.MapClaims {
		c := jwt.MapClaims{}
		for kk, vv := range good {
			c[kk] = vv
		}
		if val == nil {
			delete(c, k)
		} else {
			c[k] = val
		}
		return c
	}

	id, err := v.Verify(ctx, sign(t, key, good))
	if err != nil || id.UserID != "user-123" || !id.IsAnonymous {
		t.Fatalf("valid token: id=%+v err=%v", id, err)
	}

	otherKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	bad := map[string]string{
		"expired":      sign(t, key, with("exp", time.Now().Add(-time.Minute).Unix())),
		"no exp":       sign(t, key, with("exp", nil)),
		"wrong aud":    sign(t, key, with("aud", "someone-else")),
		"wrong iss":    sign(t, key, with("iss", "https://evil.example/auth/v1")),
		"anon role":    sign(t, key, with("role", "anon")),
		"wrong key":    sign(t, otherKey, good),
		"garbage":      "not.a.jwt",
		"hs256 forged": hs256(t, good),
	}
	for name, tok := range bad {
		if _, err := v.Verify(ctx, tok); err == nil {
			t.Errorf("%s: expected rejection", name)
		}
	}
}

func hs256(t *testing.T, c jwt.MapClaims) string {
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString([]byte("guess"))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestDevVerifier(t *testing.T) {
	if _, err := (DevVerifier{}).Verify(context.Background(), "dev:abcd1234"); err != nil {
		t.Fatal(err)
	}
	for _, tok := range []string{"abcd1234", "dev:", "dev:short", "dev:has spaces!!"} {
		if _, err := (DevVerifier{}).Verify(context.Background(), tok); err == nil {
			t.Errorf("%q: expected rejection", tok)
		}
	}
}
