package auth

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	SessionCookieName = "gitlab-workspace-session"
)

func checkIfValidCookieExists(r *http.Request, config *Config, workspaceID string) bool {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return false
	}

	if cookie.Value == "" {
		return false
	}

	return validateJWT(config.SigningKey, cookie.Value, workspaceID)
}

func setCookie(w http.ResponseWriter, value string, domain string, expires int) {
	// Remove port
	domainElements := strings.Split(domain, ":")
	cookie := &http.Cookie{
		Path:    "/",
		Domain:  fmt.Sprintf(".%s", domainElements[0]),
		Name:    SessionCookieName,
		Value:   value,
		Expires: time.Now().Add(time.Duration(expires) * time.Second),
		Secure:  false,
	}
	http.SetCookie(w, cookie)
}
