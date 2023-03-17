package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestGetWorkspaceName(t *testing.T) {
	tt := []struct {
		description    string
		state          string
		expectedError  bool
		expectedResult string
	}{
		{
			description:   "When state is invalid throws an error",
			state:         "://123",
			expectedError: true,
		},
		{
			description:    "When a valid host name exists returns first part",
			state:          "http://workspace1.example.com",
			expectedResult: "workspace1",
		},
	}

	for _, tr := range tt {
		t.Run(tr.description, func(t *testing.T) {
			result, err := getWorkspaceName(tr.state)
			if tr.expectedError {
				require.NotNil(t, err)
				return
			}

			require.Nil(t, err)
			require.Equal(t, tr.expectedResult, result)
		})
	}
}

func TestErrorResponse(t *testing.T) {
	recorder := httptest.NewRecorder()
	logger := zaptest.NewLogger(t)
	err := fmt.Errorf("New error occurred")
	errorResponse(logger, err, recorder)
}

func TestRedirectToAuthUrl(t *testing.T) {
	config := &AuthConfig{
		Host:        "http://my.gitlab.com",
		ClientID:    "CLIENT_ID",
		RedirectURI: "http://workspaces.com/callback",
	}

	tests := []struct {
		description string
		requestURI  string
		expectedURL string
	}{
		{
			description: "With hostname only",
			requestURI:  "http://myworkspace.workspace.com",
			expectedURL: "http://my.gitlab.com/oauth/authorize?response_type=code&client_id=CLIENT_ID&redirect_uri=http://workspaces.com/callback&scope=openid profile&state=http%3A%2F%2Fmyworkspace.workspace.com",
		},
		{
			description: "With query string",
			requestURI:  "http://myworkspace.workspace.com?tkn=pass",
			expectedURL: "http://my.gitlab.com/oauth/authorize?response_type=code&client_id=CLIENT_ID&redirect_uri=http://workspaces.com/callback&scope=openid profile&state=http%3A%2F%2Fmyworkspace.workspace.com%3Ftkn%3Dpass",
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, tt.requestURI, nil)

			redirectToAuthURL(config, recorder, request)
			result := recorder.Result()
			defer result.Body.Close()

			require.Equal(t, http.StatusTemporaryRedirect, result.StatusCode)

			require.Equal(t, tt.expectedURL, result.Header["Location"][0])
		})
	}
}

func TestIsRedirectUri(t *testing.T) {
	config := &AuthConfig{
		RedirectURI: "http://workspaces.com/callback",
	}

	tt := []struct {
		description    string
		request        *http.Request
		expectedResult bool
	}{
		{
			description:    "When the redirect uri does not match the current uri returns false",
			request:        httptest.NewRequest(http.MethodGet, "http://workspaces.com", nil),
			expectedResult: false,
		},
		{
			description:    "When the redirect uri does match the current uri returns true",
			request:        httptest.NewRequest(http.MethodGet, "http://workspaces.com/callback", nil),
			expectedResult: true,
		},
	}

	for _, tr := range tt {
		t.Run(tr.description, func(t *testing.T) {
			result := isRedirectURI(config, tr.request)
			require.Equal(t, tr.expectedResult, result)
		})
	}
}

func TestMiddleware(t *testing.T) {
	logger := zaptest.NewLogger(t)

	tt := []struct {
		description        string
		request            *http.Request
		expectedStatusCode int
	}{
		{
			description:        "When no cookie is present, should redirect to auth url",
			request:            httptest.NewRequest(http.MethodGet, "http://workspace1.workspaces.com", nil),
			expectedStatusCode: http.StatusTemporaryRedirect,
		},
		{
			description:        "When a valid cookie is present, should return the result",
			request:            generateRequestWithCookie(generateToken(t, 10), "http://workspace1.workspaces.com"),
			expectedStatusCode: http.StatusOK,
		},
		{
			description:        "When redirect uri is called without code throws an error",
			request:            httptest.NewRequest(http.MethodGet, "http://workspaces.com/callback", nil),
			expectedStatusCode: http.StatusBadRequest,
		},
		{
			description:        "When redirect uri is called with code but without state throws an error",
			request:            httptest.NewRequest(http.MethodGet, "http://workspaces.com/callback?code=123", nil),
			expectedStatusCode: http.StatusBadRequest,
		},
		{
			description:        "When redirect uri is called with code but without state throws an error",
			request:            httptest.NewRequest(http.MethodGet, "http://workspaces.com/callback?code=123", nil),
			expectedStatusCode: http.StatusBadRequest,
		},
		{
			description:        "When redirect uri is called with code and state, redirects to state",
			request:            httptest.NewRequest(http.MethodGet, "http://workspaces.com/callback?code=123&state=http://workspace1.workspaces.com", nil),
			expectedStatusCode: http.StatusTemporaryRedirect,
		},
	}

	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := token{
			AccessToken: "abc",
		}

		data, err := json.Marshal(result)
		require.Nil(t, err)

		_, _ = w.Write(data)
	}))

	config := &AuthConfig{
		Host:         svr.URL,
		ClientID:     "CLIENT_ID",
		ClientSecret: "CLIENT_SECRET",
		RedirectURI:  "http://workspaces.com/callback",
		SigningKey:   "abc",
	}

	for _, tr := range tt {
		t.Run(tr.description, func(t *testing.T) {
			recorder := httptest.NewRecorder()

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte("Hello World"))
			})

			middleware := NewMiddleware(logger, config)(handler)
			middleware.ServeHTTP(recorder, tr.request)

			result := recorder.Result()
			defer result.Body.Close()
			require.Equal(t, tr.expectedStatusCode, result.StatusCode)
		})
	}
}
