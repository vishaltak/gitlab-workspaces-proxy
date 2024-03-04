package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckIfValidCookieExists(t *testing.T) {
	config := &Config{
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
			request:     generateRequestWithCookie(t, "xyz", "https://my.workspace.com"),
			expected:    false,
		},
		{
			description: "When a valid token exists returns true",
			request:     generateRequestWithCookie(t, generateToken(t, 1, "1"), "https://my.workspace.com"),
			expected:    true,
		},
		{
			description: "When the token is expired returns false",
			request:     generateRequestWithCookie(t, generateToken(t, -1, "1"), "https://my.workspace.com"),
			expected:    false,
		},
	}

	for _, tr := range tt {
		t.Run(tr.description, func(t *testing.T) {
			result := checkIfValidCookieExists(tr.request, config, "1")
			require.Equal(t, tr.expected, result)
		})
	}
}

func generateRequestWithCookie(t *testing.T, token string, url string) *http.Request {
	t.Helper()
	recorder := httptest.NewRecorder()
	setCookie(recorder, token, "example.com", 1)

	request := httptest.NewRequest(http.MethodGet, url, nil)
	result := recorder.Result()

	request.Header = http.Header{"Cookie": result.Header["Set-Cookie"]}
	closeErr := result.Body.Close()
	if closeErr != nil {
		t.Error(closeErr)
	}
	return request
}
