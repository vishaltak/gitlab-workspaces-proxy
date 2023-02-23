package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

func getToken(ctx context.Context, config *AuthConfig, code string) (*token, error) {
	u := fmt.Sprintf("%s/oauth/token", config.Host)
	form := url.Values{
		"redirect_uri":  []string{config.RedirectUri},
		"grant_type":    []string{"authorization_code"},
		"code":          []string{code},
		"client_id":     []string{config.ClientID},
		"client_secret": []string{config.ClientSecret},
	}

	req, err := http.NewRequest(http.MethodPost, u, bytes.NewBufferString(form.Encode()))
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	client := http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	var tkn token
	err = json.NewDecoder(res.Body).Decode(&tkn)
	if err != nil {
		return nil, err
	}

	return &tkn, nil
}

type token struct {
	AccessToken  string `json:"access_token"`
	IdToken      string `json:"id_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}
