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
			token:       generateToken(t, 1),
			expected:    true,
		},
		{
			description: "If a token is expired returns false",
			token:       generateToken(t, -1),
			expected:    false,
		},
	}

	for _, tr := range tt {
		t.Run(tr.description, func(t *testing.T) {
			result := validateJwt(signingKey, tr.token)
			require.Equal(t, tr.expected, result)
		})
	}
}

func generateToken(t *testing.T, expires int) string {
	tkn, err := generateJwt(signingKey, "test", expires)
	require.Nil(t, err)

	return tkn
}
