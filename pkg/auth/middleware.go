package auth

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	"gitlab.com/remote-development/gitlab-workspaces-proxy/internal/logz"
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
	logger *zap.Logger,
	config *Config,
	upstreams *upstream.Tracker,
	apiFactory gitlab.APIFactory,
) HTTPMiddleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// TODO: refactor this block - https://gitlab.com/gitlab-org/gitlab/-/issues/408340
			// Check path if callback then get token and set cookie
			if isRedirectURI(config, r) {
				handleRedirect(logger, r, w, config, upstreams, apiFactory)
				return
			}

			protocol := "https"
			if config.Protocol != "" {
				protocol = config.Protocol
			}
			workspaceURL := fmt.Sprintf("%s://%s%s%s%s", protocol, r.Host, r.URL.Port(), r.URL.Path, r.URL.RawQuery)
			logger.Debug("attempting to find workspace upstream from url", logz.WorkspaceURL(workspaceURL))
			workspace, err := getWorkspaceFromURL(workspaceURL, upstreams)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				logger.Error("failed to find workspace upstream from url",
					logz.Error(err),
					logz.WorkspaceURL(workspaceURL),
				)
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
	logger *zap.Logger,
	r *http.Request,
	w http.ResponseWriter,
	config *Config,
	upstreams *upstream.Tracker,
	apiFactory gitlab.APIFactory,
) {
	if authCode, ok := r.URL.Query()["code"]; ok {
		token, err := getToken(r.Context(), config, authCode[0])
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			logger.Error("failed to find token in the request", logz.Error(err))
			return
		}

		state := r.URL.Query().Get("state")
		if state == "" {
			w.WriteHeader(http.StatusBadRequest)
			logger.Error("failed to find state in the request", logz.Error(err))
			return
		}

		workspace, err := getWorkspaceFromURL(state, upstreams)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			logger.Error("failed to find workspace upstream from state",
				logz.Error(err),
				logz.WorkspaceURL(state),
			)
		}

		logger.Debug("attempting to authorize workspace access request", logz.WorkspaceName(workspace.WorkspaceName))
		err = checkAuthorization(r.Context(), token.AccessToken, workspace.WorkspaceID, apiFactory)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			logger.Error("failed to authorize workspace access request",
				logz.Error(err),
				logz.WorkspaceName(workspace.WorkspaceName),
			)
			return
		}
		logger.Debug("workspace access authorization successful", logz.WorkspaceName(workspace.WorkspaceName))

		// Create JWT for cookie
		signedJwt, err := generateJWT(config.SigningKey, workspace.WorkspaceID, token.ExpiresIn)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			logger.Error("failed to generate jwt",
				logz.Error(err),
				logz.WorkspaceName(workspace.WorkspaceName),
			)
			return
		}

		// Redirect to url from state
		stateURI, _ := url.QueryUnescape(state)

		// Write Cookie
		setCookie(w, signedJwt, r.Host, token.ExpiresIn)

		http.Redirect(w, r, stateURI, http.StatusTemporaryRedirect)
		return
	} else {
		w.WriteHeader(http.StatusBadRequest)
		logger.Error("failed to find auth code in the request")
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

	upstreamHostMapping, err := upstreams.GetByHostname(hostname)
	if err != nil {
		return nil, fmt.Errorf("could not find upstream workspace %s", err)
	}

	return upstreamHostMapping, nil
}
