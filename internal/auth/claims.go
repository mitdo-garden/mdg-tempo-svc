package auth

import (
	"github.com/golang-jwt/jwt/v5"
)

// Claims mirrors mdg-auth-svc access JWT claims (uid + profile fields).
type Claims struct {
	UserID   string `json:"uid"`
	Provider string `json:"provider"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	jwt.RegisteredClaims
}
