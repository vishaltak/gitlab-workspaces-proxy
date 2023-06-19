package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/require"
)

const signingKey = "abc"

func TestValidateJwt(t *testing.T) {
	tt := []struct {
		description string
		token       string
		expected    bool
	}{
		{
			description: "When token is invalid should return false",
			token:       "123",
			expected:    false,
		},
		{
			description: "When a valid token is passed returns true",
			token:       generateToken(t, 1, "1"),
			expected:    true,
		},
		{
			description: "If a token is expired returns false",
			token:       generateToken(t, -1, "1"),
			expected:    false,
		},
		{
			description: "If a token is for a different workspace",
			token:       generateToken(t, -1, "2"),
			expected:    false,
		},
		{
			description: "If a token was generated using an unsupported method",
			token:       generateUnsupportedJWT(t),
			expected:    false,
		},
	}

	for _, tr := range tt {
		t.Run(tr.description, func(t *testing.T) {
			result := validateJWT(signingKey, tr.token, "1")
			require.Equal(t, tr.expected, result)
		})
	}
}

func generateToken(t *testing.T, expires int, workspaceID string) string {
	t.Helper()
	tkn, err := generateJWT(signingKey, workspaceID, expires)
	require.NoError(t, err)

	return tkn
}

func generateUnsupportedJWT(t *testing.T) string {
	t.Helper()

	testValidClaim := &Claims{
		WorkspaceID: "1",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1000 * time.Second)),
		},
	}

	// generate a token with unsupported HS384 signing method
	token := jwt.NewWithClaims(jwt.SigningMethodHS384, testValidClaim)
	tokenString, err := token.SignedString([]byte(signingKey))
	require.NoError(t, err)

	return tokenString
}
