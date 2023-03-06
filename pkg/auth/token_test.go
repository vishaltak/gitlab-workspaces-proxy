package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetToken(t *testing.T) {
	tt := []struct {
		description         string
		code                string
		expectedAccessToken string
		expectError         bool
	}{
		{
			description:         "Returns token when valid response is sent back",
			code:                "123",
			expectedAccessToken: "abc",
			expectError:         false,
		},
		{
			description:         "Returns error when server sends an error",
			code:                "INVALID",
			expectedAccessToken: "",
			expectError:         true,
		},
	}

	ctx := context.Background()

	for _, tr := range tt {
		t.Run(tr.description, func(t *testing.T) {
			svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

				code := r.FormValue("code")
				if code == "INVALID" {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}

				result := token{
					AccessToken: "abc",
				}

				data, err := json.Marshal(result)
				require.Nil(t, err)

				w.Write(data)
			}))

			config := &AuthConfig{
				Host: svr.URL,
			}

			result, err := getToken(ctx, config, tr.code)
			if tr.expectError {
				require.NotNil(t, err)
				return
			}

			require.Nil(t, err)
			require.Equal(t, tr.expectedAccessToken, result.AccessToken)
		})
	}

}