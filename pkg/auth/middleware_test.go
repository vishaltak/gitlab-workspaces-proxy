package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"gitlab.com/remote-development/gitlab-workspaces-proxy/internal/logz"
	"gitlab.com/remote-development/gitlab-workspaces-proxy/pkg/gitlab"
	"gitlab.com/remote-development/gitlab-workspaces-proxy/pkg/upstream"
	"go.uber.org/zap/zaptest"
)

func TestGetHostnameFromState(t *testing.T) {
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
			expectedResult: "workspace1.example.com",
		},
	}

	for _, tr := range tt {
		t.Run(tr.description, func(t *testing.T) {
			result, err := getHostnameFromState(tr.state)
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
	err := fmt.Errorf("new error occurred")
	recorder.WriteHeader(http.StatusBadRequest)
	logger.Error("error processing request", logz.Error(err))
}

func TestRedirectToAuthUrl(t *testing.T) {
	config := &Config{
		Host:        "https://my.gitlab.com",
		ClientID:    "CLIENT_ID",
		RedirectURI: "https://workspaces.com/callback",
	}

	tests := []struct {
		description string
		requestURI  string
		expectedURL string
	}{
		{
			description: "With hostname only",
			requestURI:  "https://myworkspace.workspace.com",
			expectedURL: "https://my.gitlab.com/oauth/authorize?response_type=code&client_id=CLIENT_ID&redirect_uri=https://workspaces.com/callback&scope=openid profile api read_user&state=https%3A%2F%2Fmyworkspace.workspace.com",
		},
		{
			description: "With query string",
			requestURI:  "https://myworkspace.workspace.com?tkn=pass",
			expectedURL: "https://my.gitlab.com/oauth/authorize?response_type=code&client_id=CLIENT_ID&redirect_uri=https://workspaces.com/callback&scope=openid profile api read_user&state=https%3A%2F%2Fmyworkspace.workspace.com%3Ftkn%3Dpass",
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, tt.requestURI, nil)

			redirectToAuthURL(config, recorder, request)
			result := recorder.Result()

			require.Equal(t, http.StatusTemporaryRedirect, result.StatusCode)
			require.Equal(t, tt.expectedURL, result.Header["Location"][0])
			closeErr := result.Body.Close()
			if closeErr != nil {
				t.Error(closeErr)
			}
		})
	}
}

func TestIsRedirectUri(t *testing.T) {
	config := &Config{
		RedirectURI: "http://workspaces.com/callback",
		Protocol:    "http",
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
		upstreams          []upstream.HostMapping
		expectedStatusCode int
		host               string
	}{
		{
			description:        "When no cookie is present should redirect to auth url",
			request:            httptest.NewRequest(http.MethodGet, "http://workspace1.workspaces.com", nil),
			upstreams:          []upstream.HostMapping{{Hostname: "workspace1.workspaces.com", WorkspaceID: "1"}},
			expectedStatusCode: http.StatusTemporaryRedirect,
		},
		{
			description:        "When a valid cookie is present should return the result",
			request:            generateRequestWithCookie(t, generateToken(t, 10, "1"), "https://workspace1.workspaces.com"),
			upstreams:          []upstream.HostMapping{{Hostname: "workspace1.workspaces.com", WorkspaceID: "1"}},
			expectedStatusCode: http.StatusOK,
		},
		{
			description:        "When redirect uri is called without code throws an error",
			request:            httptest.NewRequest(http.MethodGet, "https://workspaces.com/callback", nil),
			upstreams:          []upstream.HostMapping{},
			expectedStatusCode: http.StatusBadRequest,
		},
		{
			description:        "When redirect uri is called with code but without state throws an error",
			request:            httptest.NewRequest(http.MethodGet, "https://workspaces.com/callback?code=123", nil),
			upstreams:          []upstream.HostMapping{},
			expectedStatusCode: http.StatusBadRequest,
		},
		{
			description:        "When redirect uri is called with code but without state throws an error",
			request:            httptest.NewRequest(http.MethodGet, "https://workspaces.com/callback?code=123", nil),
			upstreams:          []upstream.HostMapping{},
			expectedStatusCode: http.StatusBadRequest,
		},
		{
			description:        "When redirect uri is called with code and state, redirects to state",
			request:            httptest.NewRequest(http.MethodGet, "https://workspaces.com/callback?code=123&state=https://workspace1.workspaces.com", nil),
			upstreams:          []upstream.HostMapping{{Hostname: "workspace1.workspaces.com"}},
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

	config := &Config{
		Host:         svr.URL,
		ClientID:     "CLIENT_ID",
		ClientSecret: "CLIENT_SECRET",
		RedirectURI:  "http://workspaces.com/callback",
		SigningKey:   "abc",
		Protocol:     "http",
	}

	for _, tr := range tt {
		t.Run(tr.description, func(t *testing.T) {
			tracker := upstream.NewTracker(logger)

			for _, us := range tr.upstreams {
				tracker.Add(us)
			}
			recorder := httptest.NewRecorder()

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte("Hello World"))
			})

			middleware := NewMiddleware(logger, config, tracker, gitlab.MockAPIFactory)(handler)
			middleware.ServeHTTP(recorder, tr.request)

			result := recorder.Result()
			require.Equal(t, tr.expectedStatusCode, result.StatusCode)
			closeErr := result.Body.Close()
			if closeErr != nil {
				t.Error(closeErr)
			}
		})
	}
}
