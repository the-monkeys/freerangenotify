//go:build ignore

// gen_token.go generates a short-lived JWT for CI integration tests.
// It uses the JWT_SECRET env var (falls back to "ci-test-jwt-secret").
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func main() {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "ci-test-jwt-secret"
	}

	claims := &struct {
		UserID string `json:"user_id"`
		Email  string `json:"email"`
		jwt.RegisteredClaims
	}{
		UserID: "00000000-0000-0000-0000-000000000001",
		Email:  "ci-admin@test.local",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := t.SignedString([]byte(secret))
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to sign token: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(s)
}
