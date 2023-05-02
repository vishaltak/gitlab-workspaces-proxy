package auth

import (
	"fmt"
	"log"
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
	Protocol     string `yaml:"protocol"`
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
				return
			}

			protocol := "https"
			if config.Protocol != "" {
				protocol = config.Protocol
			}
			url := fmt.Sprintf("%s://%s%s%s%s", protocol, r.Host, r.URL.Port(), r.URL.Path, r.URL.RawQuery)
			log.Debug("finding upstream with url", zap.String("url", url))
			workspace, err := getWorkspaceFromURL(url, upstreams)
			if err != nil {
				errorResponse(log, err, w)
				return
			}

			// Check if cookie is already present for workspace ID
			if !checkIfValidCookieExists(r, config, workspace.WorkspaceID) {
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

		workspace, err := getWorkspaceFromURL(state, upstreams)
		if err != nil {
			errorResponse(log, err, w)
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
	log.Printf("getHostnameFromState state=%s", stateURL)

	u, err := url.Parse(stateURL)
	if err != nil {
		return "", err
	}
	log.Printf("getHostnameFromState u.Hostname()=%s", u.Hostname())

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

	protocol := "https"
	if config.Protocol != "" {
		protocol = config.Protocol
	}

	state := url.QueryEscape(fmt.Sprintf("%s://%s%s%s%s", protocol, r.Host, port, r.URL.Path, query))
	authURL := fmt.Sprintf("%s/oauth/authorize?response_type=code&client_id=%s&redirect_uri=%s&scope=openid profile api read_user&state=%s", config.Host, config.ClientID, config.RedirectURI, state)
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

func isRedirectURI(config *Config, r *http.Request) bool {
	protocol := "https"
	if config.Protocol != "" {
		protocol = config.Protocol
	}

	uri := fmt.Sprintf("%s://%s%s", protocol, r.Host, r.URL.Path)
	return uri == config.RedirectURI
}

func getWorkspaceFromURL(url string, upstreams *upstream.Tracker) (*upstream.HostMapping, error) {
	hostname, err := getHostnameFromState(url)
	if err != nil {
		return nil, fmt.Errorf("could not parse workspace from host %s", err)
	}

	upstream, err := upstreams.Get(hostname)
	if err != nil {
		return nil, fmt.Errorf("could not find upstream workspace %s", err)
	}

	return upstream, nil
}
