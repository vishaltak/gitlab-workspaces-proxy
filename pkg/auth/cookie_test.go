package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckIfValidCookieExists(t *testing.T) {

	config := &AuthConfig{
		SigningKey: "abc",
	}

	tt := []struct {
		description string
		request     *http.Request
		expected    bool
	}{
		{
			description: "When no cookie exists returns false",
			request:     &http.Request{},
			expected:    false,
		},
		{
			description: "When a cookie exists but is invalid returns false",
			request:     generateRequestWithCookie("xyz"),
			expected:    false,
		},
		{
			description: "When a valid token exists returns true",
			request:     generateRequestWithCookie(generateToken(t, 1)),
			expected:    true,
		},
		{
			description: "When the token is expired returns false",
			request:     generateRequestWithCookie(generateToken(t, -1)),
			expected:    false,
		},
	}

	for _, tr := range tt {
		t.Run(tr.description, func(t *testing.T) {
			result := checkIfValidCookieExists(tr.request, config)
			require.Equal(t, tr.expected, result)
		})
	}

}

func generateRequestWithCookie(token string) *http.Request {
	recorder := httptest.NewRecorder()
	setCookie(recorder, token, "example.com", 1)
	request := &http.Request{Header: http.Header{"Cookie": recorder.HeaderMap["Set-Cookie"]}}
	return request
}