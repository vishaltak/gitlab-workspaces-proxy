package auth

import (
	"testing"

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
	require.Nil(t, err)

	return tkn
}
