// Package jwks is the consumer-side JWKS client for Mitdo services that
// verify mdg-auth-svc ES256 session JWTs without ever seeing private keys.
//
// Cache policy:
//   - Respect Cache-Control max-age from the JWKS response (default 60s).
//   - On an unknown kid, force one refresh then retry (key rotation).
package jwks

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const defaultMaxAge = 60 * time.Second

var (
	ErrEmptyURL    = errors.New("jwks: empty URL")
	ErrUnknownKid  = errors.New("jwks: unknown kid")
	ErrNoKeys      = errors.New("jwks: empty key set")
	ErrBadResponse = errors.New("jwks: bad response")
	ErrUnsupported = errors.New("jwks: unsupported key")
)

// Client fetches and caches a JWKS document.
type Client struct {
	url    string
	http   *http.Client
	minTTL time.Duration // floor for cache TTL (also used when header missing)

	mu      sync.RWMutex
	keys    map[string]*ecdsa.PublicKey
	expires time.Time

	// Single in-flight refresh so concurrent unknown-kid lookups share one fetch.
	refreshMu sync.Mutex
}

// Option configures a Client.
type Option func(*Client)

// WithHTTPClient sets the HTTP client used to fetch JWKS.
func WithHTTPClient(c *http.Client) Option {
	return func(cl *Client) {
		if c != nil {
			cl.http = c
		}
	}
}

// WithMinTTL sets the minimum cache duration (also the default when the
// JWKS response has no usable Cache-Control). Default is 60s.
func WithMinTTL(d time.Duration) Option {
	return func(cl *Client) {
		if d > 0 {
			cl.minTTL = d
		}
	}
}

// NewClient builds a JWKS consumer for the given URL
// (e.g. https://api.mitdo.com/.well-known/jwks.json).
func NewClient(jwksURL string, opts ...Option) (*Client, error) {
	if strings.TrimSpace(jwksURL) == "" {
		return nil, ErrEmptyURL
	}
	cl := &Client{
		url:    strings.TrimSpace(jwksURL),
		http:   &http.Client{Timeout: 10 * time.Second},
		minTTL: defaultMaxAge,
		keys:   make(map[string]*ecdsa.PublicKey),
	}
	for _, opt := range opts {
		opt(cl)
	}
	return cl, nil
}

// Key returns the ECDSA P-256 public key for kid.
// Uses the cache while fresh; if kid is missing (or cache expired), refreshes
// JWKS once then looks up again — so rotation has no downtime for consumers.
func (c *Client) Key(ctx context.Context, kid string) (*ecdsa.PublicKey, error) {
	if kid == "" {
		return nil, fmt.Errorf("%w: empty", ErrUnknownKid)
	}

	if pub, ok := c.lookupFresh(kid); ok {
		return pub, nil
	}

	// Cache miss or stale: refresh. Force when the cache is still "fresh"
	// but kid is absent (new key after rotation within max-age window).
	force := c.cacheFresh()
	if err := c.refresh(ctx, force); err != nil {
		return nil, err
	}

	if pub, ok := c.lookupAny(kid); ok {
		return pub, nil
	}
	return nil, fmt.Errorf("%w: %q", ErrUnknownKid, kid)
}

// Refresh forces a JWKS fetch regardless of cache TTL.
func (c *Client) Refresh(ctx context.Context) error {
	return c.refresh(ctx, true)
}

func (c *Client) cacheFresh() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return time.Now().Before(c.expires) && len(c.keys) > 0
}

func (c *Client) lookupFresh(kid string) (*ecdsa.PublicKey, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !time.Now().Before(c.expires) {
		return nil, false
	}
	pub, ok := c.keys[kid]
	return pub, ok
}

func (c *Client) lookupAny(kid string) (*ecdsa.PublicKey, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	pub, ok := c.keys[kid]
	return pub, ok
}

func (c *Client) refresh(ctx context.Context, force bool) error {
	c.refreshMu.Lock()
	defer c.refreshMu.Unlock()

	// Another waiter may have refreshed while we blocked.
	if !force && c.cacheFresh() {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")

	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: status %d", ErrBadResponse, res.StatusCode)
	}

	var doc struct {
		Keys []jwk `json:"keys"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return fmt.Errorf("%w: %v", ErrBadResponse, err)
	}
	if len(doc.Keys) == 0 {
		return ErrNoKeys
	}

	next := make(map[string]*ecdsa.PublicKey, len(doc.Keys))
	for _, k := range doc.Keys {
		pub, err := k.ecdsaPublic()
		if err != nil {
			return err
		}
		if k.Kid == "" {
			return fmt.Errorf("%w: key missing kid", ErrBadResponse)
		}
		next[k.Kid] = pub
	}

	ttl := cacheTTL(res.Header.Get("Cache-Control"), c.minTTL)
	c.mu.Lock()
	c.keys = next
	c.expires = time.Now().Add(ttl)
	c.mu.Unlock()
	return nil
}

func cacheTTL(cacheControl string, min time.Duration) time.Duration {
	ttl := min
	for _, part := range strings.Split(cacheControl, ",") {
		part = strings.TrimSpace(strings.ToLower(part))
		if strings.HasPrefix(part, "max-age=") {
			n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(part, "max-age=")))
			if err == nil && n >= 0 {
				ttl = time.Duration(n) * time.Second
			}
		}
	}
	if ttl < min {
		ttl = min
	}
	return ttl
}

type jwk struct {
	Kty string `json:"kty"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	Kid string `json:"kid"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

func (k jwk) ecdsaPublic() (*ecdsa.PublicKey, error) {
	if k.Kty != "" && k.Kty != "EC" {
		return nil, fmt.Errorf("%w: kty %q", ErrUnsupported, k.Kty)
	}
	if k.Alg != "" && k.Alg != "ES256" {
		return nil, fmt.Errorf("%w: alg %q", ErrUnsupported, k.Alg)
	}
	if k.Crv != "" && k.Crv != "P-256" {
		return nil, fmt.Errorf("%w: crv %q", ErrUnsupported, k.Crv)
	}
	xb, err := base64.RawURLEncoding.DecodeString(k.X)
	if err != nil {
		return nil, fmt.Errorf("%w: x: %v", ErrBadResponse, err)
	}
	yb, err := base64.RawURLEncoding.DecodeString(k.Y)
	if err != nil {
		return nil, fmt.Errorf("%w: y: %v", ErrBadResponse, err)
	}
	if len(xb) == 0 || len(yb) == 0 {
		return nil, fmt.Errorf("%w: empty x/y", ErrBadResponse)
	}
	pub := &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     new(big.Int).SetBytes(xb),
		Y:     new(big.Int).SetBytes(yb),
	}
	if !pub.Curve.IsOnCurve(pub.X, pub.Y) {
		return nil, fmt.Errorf("%w: point not on P-256", ErrBadResponse)
	}
	return pub, nil
}
