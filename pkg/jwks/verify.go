package jwks

import (
	"context"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

// Verifier verifies mdg-auth-svc ES256 JWTs using cached JWKS public keys.
type Verifier struct {
	client *Client
	issuer string
}

// NewVerifier returns a JWT verifier backed by a JWKS Client.
// issuer should match jwt.issuer from mdg-auth-svc (e.g. "mdg-auth-svc").
func NewVerifier(client *Client, issuer string) *Verifier {
	return &Verifier{client: client, issuer: issuer}
}

// Parse verifies the token signature and standard claims, returning parsed claims.
// On unknown kid it refreshes JWKS once (via Client.Key) then retries.
func (v *Verifier) Parse(ctx context.Context, raw string, claims jwt.Claims) error {
	if claims == nil {
		claims = jwt.MapClaims{}
	}

	opts := []jwt.ParserOption{
		jwt.WithValidMethods([]string{jwt.SigningMethodES256.Alg()}),
	}
	if v.issuer != "" {
		opts = append(opts, jwt.WithIssuer(v.issuer))
	}

	_, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodES256 {
			return nil, fmt.Errorf("%w: unexpected alg %v", ErrUnsupported, t.Header["alg"])
		}
		kid, _ := t.Header["kid"].(string)
		return v.client.Key(ctx, kid)
	}, opts...)
	return err
}
