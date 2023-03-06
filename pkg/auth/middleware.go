package auth

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
)

type AuthConfig struct {
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
	RedirectUri  string `yaml:"redirect_uri"`
	Host         string `yaml:"host"`
	SigningKey   string `yaml:"signing_key"`
}

type HttpMiddleware func(http.Handler) http.Handler

func NewMiddleware(config *AuthConfig) HttpMiddleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			// Check path if callback then get token and set cookie
			if isRedirectUri(config, r) {
				if authCode, ok := r.URL.Query()["code"]; ok {
					token, err := getToken(r.Context(), config, authCode[0])
					if err != nil {
						errorResponse(fmt.Errorf("error getting token %s", err), w)
						return
					}

					state := r.URL.Query().Get("state")
					if state == "" {
						errorResponse(fmt.Errorf("state not present in request"), w)
						return
					}

					workspaceName, err := getWorkspaceName(state)
					if err != nil {
						errorResponse(fmt.Errorf("could not parse workspace from host %s", err), w)
						return
					}

					username, err := checkAuthorization(config, token.AccessToken, workspaceName)
					if err != nil {
						errorResponse(fmt.Errorf("could not authorize request %s", err), w)
						return
					}

					// Create JWT for cookie
					signedJwt, err := generateJwt(config.SigningKey, username, token.ExpiresIn)
					if err != nil {
						errorResponse(fmt.Errorf("could not generate jwt %s", err), w)
						return
					}

					// Redirect to url from state
					stateUri, _ := url.QueryUnescape(state)

					// Write Cookie
					setCookie(w, signedJwt, r.Host, token.ExpiresIn)

					http.Redirect(w, r, stateUri, http.StatusTemporaryRedirect)
					return
				}
			}

			// Check if cookie is already present
			if !checkIfValidCookieExists(r, config) {
				redirectToAuthUrl(config, w, r)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func getWorkspaceName(state string) (string, error) {
	stateUrl, err := url.QueryUnescape(state)
	if err != nil {
		return "", err
	}

	u, err := url.Parse(stateUrl)
	if err != nil {
		return "", err
	}

	// Get first part of hostname
	hostElements := strings.Split(u.Hostname(), ".")

	return hostElements[0], nil
}

func errorResponse(err error, w http.ResponseWriter) {
	w.WriteHeader(http.StatusBadRequest)
	log.Println(err)
}

func redirectToAuthUrl(config *AuthConfig, w http.ResponseWriter, r *http.Request) {
	// Calculate state based on current host
	state := url.QueryEscape(fmt.Sprintf("http://%s%s", r.Host, r.URL.Path))
	authUrl := fmt.Sprintf("%s/oauth/authorize?response_type=code&client_id=%s&redirect_uri=%s&scope=openid profile&state=%s", config.Host, config.ClientID, config.RedirectUri, state)
	http.Redirect(w, r, authUrl, http.StatusTemporaryRedirect)
}

func isRedirectUri(config *AuthConfig, r *http.Request) bool {
	uri := fmt.Sprintf("http://%s%s%s", r.URL.Scheme, r.Host, r.URL.Path)
	return uri == config.RedirectUri
}

func checkAuthorization(config *AuthConfig, accessToken string, workspace string) (string, error) {
	return "patnaikshekhar", nil
}
