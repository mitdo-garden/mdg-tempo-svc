package middleware

import (
	"context"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"

	"github.com/mitdo-garden/mdg-tempo-svc/internal/auth"
	"github.com/mitdo-garden/mdg-tempo-svc/internal/httperr"
)

const UserClaimsKey = "userClaims"

// TokenVerifier verifies JWTs using JWKS public keys (ES256 + kid).
type TokenVerifier interface {
	Parse(ctx context.Context, raw string, claims jwt.Claims) error
}

// Authenticate requires a valid access JWT via Bearer or session cookie.
func Authenticate(verifier TokenVerifier, cookieName string) fiber.Handler {
	return func(c fiber.Ctx) error {
		raw, source := extractToken(c, cookieName)
		if raw == "" {
			return httperr.Write(c, fiber.StatusUnauthorized, "unauthenticated", "Sign in required.")
		}

		claims := &auth.Claims{}
		if err := verifier.Parse(c.Context(), raw, claims); err != nil {
			return httperr.Write(c, fiber.StatusUnauthorized, "unauthenticated", "Sign in required.")
		}
		if claims.UserID == "" {
			return httperr.Write(c, fiber.StatusUnauthorized, "unauthenticated", "Sign in required.")
		}

		c.Locals(UserClaimsKey, claims)
		c.Locals("authSource", source)
		return c.Next()
	}
}

// ClaimsFromLocals returns the verified session claims, if present.
func ClaimsFromLocals(c fiber.Ctx) (*auth.Claims, bool) {
	claims, ok := c.Locals(UserClaimsKey).(*auth.Claims)
	return claims, ok
}

// UserID returns the authenticated garden owner id (UUID).
func UserID(c fiber.Ctx) (string, bool) {
	claims, ok := ClaimsFromLocals(c)
	if !ok || claims.UserID == "" {
		return "", false
	}
	return claims.UserID, true
}

func extractToken(c fiber.Ctx, cookieName string) (token string, source string) {
	if authz := strings.TrimSpace(c.Get("Authorization")); authz != "" {
		const prefix = "Bearer "
		if strings.HasPrefix(authz, prefix) {
			tok := strings.TrimSpace(authz[len(prefix):])
			if tok != "" {
				return tok, "bearer"
			}
		}
		return "", "bearer"
	}
	if cookie := c.Cookies(cookieName); cookie != "" {
		return cookie, "cookie"
	}
	return "", ""
}
