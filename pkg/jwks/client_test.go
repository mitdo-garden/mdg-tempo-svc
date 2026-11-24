package jwks

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestCacheAndRefreshOnUnknownKid(t *testing.T) {
	keyA, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	keyB, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	var hits atomic.Int32
	var serveB atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Cache-Control", "public, max-age=60")
		w.Header().Set("Content-Type", "application/json")
		keys := []map[string]string{jwkMap("kid-a", &keyA.PublicKey)}
		if serveB.Load() {
			keys = append(keys, jwkMap("kid-b", &keyB.PublicKey))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": keys})
	}))
	defer srv.Close()

	cl, err := NewClient(srv.URL, WithMinTTL(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	pubA, err := cl.Key(ctx, "kid-a")
	if err != nil {
		t.Fatal(err)
	}
	if pubA.X.Cmp(keyA.X) != 0 || pubA.Y.Cmp(keyA.Y) != 0 {
		t.Fatal("wrong key a")
	}
	if hits.Load() != 1 {
		t.Fatalf("hits=%d want 1", hits.Load())
	}

	// Cache hit — no extra fetch.
	if _, err := cl.Key(ctx, "kid-a"); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 1 {
		t.Fatalf("hits=%d want still 1", hits.Load())
	}

	// Unknown kid while cache fresh → force refresh, then succeed.
	serveB.Store(true)
	pubB, err := cl.Key(ctx, "kid-b")
	if err != nil {
		t.Fatal(err)
	}
	if pubB.X.Cmp(keyB.X) != 0 || pubB.Y.Cmp(keyB.Y) != 0 {
		t.Fatal("wrong key b")
	}
	if hits.Load() != 2 {
		t.Fatalf("hits=%d want 2 after unknown kid", hits.Load())
	}

	// Still-unknown after refresh → error.
	if _, err := cl.Key(ctx, "kid-missing"); err == nil {
		t.Fatal("expected unknown kid")
	}
	if hits.Load() != 3 {
		t.Fatalf("hits=%d want 3", hits.Load())
	}
}

func TestVerifierParsesES256(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=60")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]string{jwkMap("2026-10", &key.PublicKey)},
		})
	}))
	defer srv.Close()

	cl, err := NewClient(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	const uid = "550e8400-e29b-41d4-a716-446655440000"
	tok := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"iss": "mdg-auth-svc",
		"sub": uid,
		"uid": uid,
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	tok.Header["kid"] = "2026-10"
	raw, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}

	v := NewVerifier(cl, "mdg-auth-svc")
	claims := jwt.MapClaims{}
	if err := v.Parse(context.Background(), raw, claims); err != nil {
		t.Fatal(err)
	}
	if claims["sub"] != uid {
		t.Fatalf("claims=%v", claims)
	}
}

func TestCacheTTLFromHeader(t *testing.T) {
	if got := cacheTTL("public, max-age=30", time.Minute); got != time.Minute {
		t.Fatalf("floor: got %v", got)
	}
	if got := cacheTTL("public, max-age=120", time.Minute); got != 120*time.Second {
		t.Fatalf("header: got %v", got)
	}
}

func jwkMap(kid string, pub *ecdsa.PublicKey) map[string]string {
	return map[string]string{
		"kty": "EC",
		"use": "sig",
		"alg": "ES256",
		"kid": kid,
		"crv": "P-256",
		"x":   base64.RawURLEncoding.EncodeToString(padCoord(pub.X, 32)),
		"y":   base64.RawURLEncoding.EncodeToString(padCoord(pub.Y, 32)),
	}
}

func padCoord(v *big.Int, size int) []byte {
	b := v.Bytes()
	if len(b) >= size {
		return b
	}
	out := make([]byte, size)
	copy(out[size-len(b):], b)
	return out
}
