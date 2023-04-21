package auth

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"gitlab.com/remote-development/gitlab-workspaces-proxy/pkg/gitlab"
	"gitlab.com/remote-development/gitlab-workspaces-proxy/pkg/upstream"
	"go.uber.org/zap"
)

type Config struct {
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
	RedirectURI  string `yaml:"redirect_uri"`
	Host         string `yaml:"host"`
	SigningKey   string `yaml:"signing_key"`
}

type HTTPMiddleware func(http.Handler) http.Handler

func NewMiddleware(
	log *zap.Logger,
	config *Config,
	upstreams *upstream.Tracker,
	apiFactory gitlab.APIFactory,
) HTTPMiddleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// TODO: refactor this block - https://gitlab.com/gitlab-org/gitlab/-/issues/408340
			// Check path if callback then get token and set cookie
			if isRedirectURI(config, r) {
				handleRedirect(log, r, w, config, upstreams, apiFactory)
			}

			// Check if cookie is already present
			if !checkIfValidCookieExists(r, config) {
				redirectToAuthURL(config, w, r)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func handleRedirect(
	log *zap.Logger,
	r *http.Request,
	w http.ResponseWriter,
	config *Config,
	upstreams *upstream.Tracker,
	apiFactory gitlab.APIFactory,
) {
	if authCode, ok := r.URL.Query()["code"]; ok {
		token, err := getToken(r.Context(), config, authCode[0])
		if err != nil {
			errorResponse(log, fmt.Errorf("error getting token %s", err), w)
			return
		}

		state := r.URL.Query().Get("state")
		if state == "" {
			errorResponse(log, fmt.Errorf("state not present in request"), w)
			return
		}

		hostname, err := getHostnameFromState(state)
		if err != nil {
			errorResponse(log, fmt.Errorf("could not parse workspace from host %s", err), w)
			return
		}

		log.Debug("Searching for upstream", zap.String("hostname", hostname))
		workspace, err := upstreams.Get(hostname)
		if err != nil {
			errorResponse(log, fmt.Errorf("could not find upstream workspace %s", err), w)
			return
		}

		log.Debug("Checking authorization", zap.String("workspace", workspace.WorkspaceID))
		err = checkAuthorization(r.Context(), token.AccessToken, workspace.WorkspaceID, apiFactory)
		if err != nil {
			errorResponse(log, fmt.Errorf("could not authorize request %s", err), w)
			return
		}
		log.Debug("Authorization verified", zap.String("workspace", workspace.Host))

		// Create JWT for cookie
		signedJwt, err := generateJWT(config.SigningKey, workspace.WorkspaceID, token.ExpiresIn)
		if err != nil {
			errorResponse(log, fmt.Errorf("could not generate jwt %s", err), w)
			return
		}

		// Redirect to url from state
		stateURI, _ := url.QueryUnescape(state)

		// Write Cookie
		setCookie(w, signedJwt, r.Host, token.ExpiresIn)

		http.Redirect(w, r, stateURI, http.StatusTemporaryRedirect)
		return
	} else {
		errorResponse(log, fmt.Errorf("could not find auth code"), w)
		return
	}
}

func getHostnameFromState(state string) (string, error) {
	stateURL, err := url.QueryUnescape(state)
	if err != nil {
		return "", err
	}

	u, err := url.Parse(stateURL)
	if err != nil {
		return "", err
	}

	// Get first part of hostname (without port)
	hostElements := strings.Split(u.Hostname(), ":")

	return hostElements[0], nil
}

func errorResponse(log *zap.Logger, err error, w http.ResponseWriter) {
	w.WriteHeader(http.StatusBadRequest)
	log.Error("error processing request", zap.Error(err))
}

func redirectToAuthURL(config *Config, w http.ResponseWriter, r *http.Request) {
	// Calculate state based on current host
	query := ""
	port := ""
	if r.URL.RawQuery != "" {
		query = fmt.Sprintf("?%s", r.URL.RawQuery)
	}

	if r.URL.Port() != "" {
		port = fmt.Sprintf(":%s", r.URL.Port())
	}

	state := url.QueryEscape(fmt.Sprintf("http://%s%s%s%s", r.Host, port, r.URL.Path, query))
	authURL := fmt.Sprintf("%s/oauth/authorize?response_type=code&client_id=%s&redirect_uri=%s&scope=openid profile api read_user&state=%s", config.Host, config.ClientID, config.RedirectURI, state)
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

func isRedirectURI(config *Config, r *http.Request) bool {
	uri := fmt.Sprintf("http://%s%s", r.Host, r.URL.Path)
	return uri == config.RedirectURI
}
